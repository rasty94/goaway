# Resolver benchmarks and soak baseline

How to measure goaway's DNS transports and the numbers to compare against.
Re-run both after changes to the resolver, the listeners or their dependencies
(`codeberg.org/miekg/dns`, `quic-go`), and update this page when the baseline
moves.

## Transport benchmarks

```sh
make bench-transport
```

`BenchmarkTransport` (one client, sequential) and `BenchmarkTransportParallel`
(one client per CPU) live in `backend/dns/server/transport_bench_test.go`. They
start each listener in-process on `127.0.0.1` with a stub handler that returns
one A record, so the numbers measure the transport (framing, TLS/QUIC, HTTP)
and goaway's listener and upstream-client code, not resolution, cache or
blocklists. TCP and DoT keep one connection per client, as stub resolvers do.

`TestTransportStartStopNoLeaks` in the same file runs in CI: it starts each
listener, serves queries, shuts it down and fails if any goroutine survives
(`goleak.VerifyNone`).

Baseline: Apple M3 Pro (11 cores, 18 GiB), macOS 27.2, Go 1.27.4, commit
`99a2464`, `-benchtime 2s`.

| Protocol | Sequential | Parallel (11) | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| UDP | 96.0 µs | 70.1 µs | 4,760 | 55 |
| TCP | 25.2 µs | 7.8 µs | 3,888 | 33 |
| DoT | 25.7 µs | 7.5 µs | 4,021 | 34 |
| DoH | 60.4 µs | 18.6 µs | 14,894 | 130 |
| DoQ | 60.3 µs | 12.3 µs | 7,752 | 131 |
| DoH3 | 82.5 µs | 20.6 µs | 27,710 | 293 |

UDP is the slowest here only because the client opens a new socket per query
(`dns.Client.Exchange`); it is not a server-side cost.

## Soak test

```sh
make soak                                              # defaults: 10 min per protocol, ~1 h
make soak SOAK_ARGS="-duration 3m -warmup 1m -proto doq"
```

`test/soak` builds nothing itself; `make soak` builds `tmp/goaway-soak` and runs
it. The tool:

1. starts a stub upstream on `127.0.0.1` (A record, TTL 30 s);
2. writes a self-signed certificate and `config/settings.yaml` into a temp dir:
   TLS on, UDP/TCP 15353, DoT 15853, DoH 15443, DoQ 15854, DoH3 15444, API
   18080, upstream = the stub, rate limiting and authentication off;
3. starts goaway there (log in `$TMPDIR/goaway-soak.log`) and, for each
   protocol in turn, sends `-qps` queries per second (default 1000) over 5,000
   distinct names for `-duration`, from 64 client goroutines;
4. samples RSS (`ps`) and `go_goroutines` (`/metrics`) every 10 s.

A protocol passes when, comparing the end of the run with the end of
`-warmup` (default 2 min), RSS grew less than `-max-growth` (default 10 %),
goroutines grew less than 10 % plus 20, and fewer than 0.1 % of queries failed.
It exits 1 if any protocol fails. Not part of CI.

Each phase reuses the same goaway process, so the full resolver path runs:
cache hits, cache misses to the upstream, query logging to SQLite and
statistics.

### Baseline

Full run with the defaults: 1000 QPS, 10 min per protocol, 2 min warm-up,
protocols run one after another against one goaway process. Same machine and
commit as above (`make soak`, about 1 h in total).

| Protocol | QPS | Errors | p50 | p99 | RSS after warm-up | RSS end | RSS growth | Goroutines after warm-up | Goroutines end | Result |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| UDP | 1000 | 0 / 599,970 | 1.99 ms | 5.58 ms | 113.6 MiB | 94.2 MiB | -17.1 % | 37 | 34 | PASS |
| TCP | 1000 | 0 / 599,970 | 0.90 ms | 5.98 ms | 95.3 MiB | 95.5 MiB | +0.1 % | 100 | 34 | PASS |
| DoT | 1000 | 0 / 599,980 | 0.84 ms | 5.92 ms | 87.3 MiB | 87.8 MiB | +0.5 % | 98 | 34 | PASS |
| DoH | 1000 | 0 / 599,960 | 1.02 ms | 6.17 ms | 90.8 MiB | 86.0 MiB | -5.3 % | 162 | 34 | PASS |
| DoQ | 1000 | 0 / 599,900 | 0.95 ms | 5.81 ms | 88.5 MiB | 88.9 MiB | +0.5 % | 226 | 34 | PASS |
| DoH3 | 1000 | 0 / 599,880 | 1.07 ms | 5.35 ms | 89.6 MiB | 89.8 MiB | +0.3 % | 357 | 34 | PASS |

How to read it:

- "Goroutines after warm-up" is taken under load and includes one or more
  goroutines per open client connection (64 clients). "Goroutines end" is taken
  after the clients have closed their connections. Every protocol returns to
  the same idle count (34), so no connection leaves goroutines behind.
- RSS settles in the first minutes; drops (UDP, DoH) are the Go runtime
  returning memory to the OS. No protocol grew by more than 0.5 %.
- Latency is end to end through goaway, including cache misses to the local
  stub upstream; it is not comparable with the in-process benchmarks.

## Bugs found while building this

These were fixed alongside the benchmarks; the tests named here fail without
the fix.

- **Every cache hit answered SERVFAIL** (with the cached records attached).
  Status strings came from `dnsutil.CodeToString`, which names EDNS0 option
  codes (`CODE0`), and the handler maps unknown names to SERVFAIL. Query logs
  also showed `CODE0`/`CODE3` instead of `NOERROR`/`NXDOMAIN`.
  `TestResolveCacheHitStatusIsNOERROR`.
- **DoQ responses were double length-prefixed**, so no DoQ client could parse
  them: `dns.Msg.WriteTo` already frames for stream writers and
  `doqResponseWriter.Write` framed again. `TestDoQServerAndUpstreamRoundTripAndSessionReuse`
  now writes the way production does.
- **DoQ connections wedged after about 100 queries under concurrency**: neither
  side consumed the stream FIN, so quic-go never retired streams or returned
  stream credit. `TestDoQServerReleasesStreamsWhenFINArrivesLate`.
- **Concurrent DoQ upstream dials closed each other's live connection.**
  Covered by `BenchmarkTransportParallel/doq`.
- **`/metrics` was gzipped twice** for clients sending
  `Accept-Encoding: gzip`, as Prometheus does. `TestMetricsNotDoubleGzipped`.
