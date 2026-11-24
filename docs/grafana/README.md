# GoAway Grafana Dashboard

Official Prometheus + Grafana dashboard for GoAway (`goaway-dashboard.json`).

## Included Panels

- **KPIs**: DNS QPS, Block Rate %, Cache Hit Ratio %, P95 Resolution Latency, Component Health (`goaway_service_health`).
- **Traffic & Filtering**: Query breakdown (Total / Cached / Forwarded / Blocked / Throttled), queries by DNS record type, Top 10 blocked domains, blocked queries by category.
- **Caching & Security**: Cached vs Serve-Stale vs Prefetch hits, DNSSEC validation breakdown (`secure`, `insecure`, `bogus`).
- **Upstreams & Latency**: P50 / P95 / P99 resolution latency, upstream wins by server/protocol/strategy (`strict`, `parallel_best`, `random`), upstream P95 latency by server/protocol.

## Import into Grafana

1. Open Grafana → **Dashboards → New → Import**.
2. Upload `docs/grafana/goaway-dashboard.json`.
3. Select your Prometheus datasource.

## Prometheus Scrape Configuration

GoAway exposes Prometheus metrics on `/metrics` (HTTP API port, default `8080`):

```yaml
scrape_configs:
  - job_name: goaway
    scrape_interval: 15s
    static_configs:
      - targets: ["goaway.local:8080"]
```

## Kubernetes (Prometheus Operator / kube-prometheus-stack)

Enable the built-in `ServiceMonitor` in `charts/goaway/values.yaml`:

```yaml
serviceMonitor:
  enabled: true
  interval: 15s
  additionalLabels:
    release: kube-prometheus-stack
```
