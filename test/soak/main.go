// Command soak drives a real goaway binary at a fixed QPS over every DNS
// transport and checks that memory and goroutines stay flat.
//
//	go build -o tmp/goaway . && go run ./test/soak -bin tmp/goaway
//
// It starts a stub upstream, writes a config (TLS, DoT/DoH/DoQ/DoH3, no rate
// limit, upstream = stub) into a temp dir, launches goaway there, then for each
// protocol runs -duration of load, sampling RSS (ps) and go_goroutines
// (/metrics). Pass: RSS at the end grew < -max-growth over the value after
// -warmup, and goroutines did too (plus a small absolute slack). Results are
// printed as a Markdown table; exit status 1 if any protocol fails.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"goaway/backend/settings"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/rdata"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

const (
	portDNS, portDoT, portDoH, portDoQ, portDoH3, portAPI = 15353, 15853, 15443, 15854, 15444, 18080
	names                                                 = 5000 // distinct qnames; upstream TTL 30s keeps some cache misses
	workers                                               = 64
)

type exchangeFunc func(ctx context.Context, m *dns.Msg) (*dns.Msg, error)

func main() {
	bin := flag.String("bin", "tmp/goaway", "goaway binary")
	protos := flag.String("proto", "udp,tcp,dot,doh,doq,doh3", "comma-separated protocols")
	qps := flag.Int("qps", 1000, "queries per second")
	duration := flag.Duration("duration", 10*time.Minute, "load duration per protocol")
	warmup := flag.Duration("warmup", 2*time.Minute, "warm-up excluded from the growth check")
	maxGrowth := flag.Float64("max-growth", 0.10, "max RSS / goroutine growth after warm-up")
	flag.Parse()

	binPath, err := filepath.Abs(*bin)
	must(err)
	dir, err := os.MkdirTemp("", "goaway-soak-")
	must(err)
	defer func() { _ = os.RemoveAll(dir) }()

	upstream := startStubUpstream()
	pool := writeConfig(dir, upstream)
	pid, stop := startGoaway(binPath, dir)
	defer stop()
	cleanup = stop

	clientTLS := &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
	readyClient, closeReady := newClient("udp", clientTLS)
	waitReady(readyClient)
	closeReady()

	fmt.Printf("\n| Protocol | QPS (achieved) | Errors | p50 | p99 | RSS after warm-up | RSS end | RSS growth | Goroutines after warm-up | Goroutines end | Result |\n")
	fmt.Printf("| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n")
	failed := false
	for _, p := range strings.Split(*protos, ",") {
		r := run(p, clientTLS, pid, *qps, *duration, *warmup)
		rssGrowth := float64(r.rssEnd-r.rssWarm) / float64(r.rssWarm)
		ok := rssGrowth < *maxGrowth &&
			float64(r.gorEnd) <= float64(r.gorWarm)*(1+*maxGrowth)+20 &&
			r.errors*1000 < r.total // < 0.1% errors
		failed = failed || !ok
		fmt.Printf("| %s | %.0f | %d / %d | %s | %s | %.1f MiB | %.1f MiB | %+.1f%% | %d | %d | %s |\n",
			p, r.achievedQPS, r.errors, r.total, r.p50, r.p99,
			mib(r.rssWarm), mib(r.rssEnd), rssGrowth*100, r.gorWarm, r.gorEnd, map[bool]string{true: "PASS", false: "FAIL"}[ok])
	}
	if failed {
		die("at least one protocol failed")
	}
}

type result struct {
	total, errors   int64
	achievedQPS     float64
	p50, p99        time.Duration
	rssWarm, rssEnd int64 // bytes
	gorWarm, gorEnd int
}

func run(proto string, clientTLS *tls.Config, pid, qps int, duration, warmup time.Duration) result {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	jobs := make(chan struct{}, qps)
	var total, errs, seq atomic.Int64
	var mu sync.Mutex
	var lat []time.Duration
	start := time.Now()

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			exchange, closeClient := newClient(proto, clientTLS)
			defer closeClient()
			for range jobs {
				n := seq.Add(1)
				q := dns.NewMsg(fmt.Sprintf("s%d.soak.test.", n%names), dns.TypeA)
				qctx, qcancel := context.WithTimeout(context.Background(), 5*time.Second)
				t0 := time.Now()
				resp, err := exchange(qctx, q)
				d := time.Since(t0)
				qcancel()
				total.Add(1)
				if err != nil || resp.Rcode != dns.RcodeSuccess || len(resp.Answer) == 0 {
					errs.Add(1)
				}
				if time.Since(start) > warmup {
					mu.Lock()
					lat = append(lat, d)
					mu.Unlock()
				}
			}
		})
	}

	// Pacer: qps/100 tokens every 10ms. A full channel means workers can't keep up.
	go func() {
		defer close(jobs)
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				for range qps / 100 {
					select {
					case jobs <- struct{}{}:
					default:
					}
				}
			}
		}
	}()

	var r result
	warmTimer := time.After(warmup)
	sample := time.NewTicker(10 * time.Second)
	defer sample.Stop()
	for done := false; !done; {
		select {
		case <-warmTimer:
			r.rssWarm, r.gorWarm = rss(pid), goroutines()
		case <-sample.C:
			fmt.Fprintf(os.Stderr, "[%s %3.0fs] queries=%d errors=%d rss=%.1fMiB goroutines=%d\n",
				proto, time.Since(start).Seconds(), total.Load(), errs.Load(), mib(rss(pid)), goroutines())
		case <-ctx.Done():
			done = true
		}
	}
	wg.Wait()
	r.rssEnd, r.gorEnd = rss(pid), goroutines()
	r.total, r.errors = total.Load(), errs.Load()
	r.achievedQPS = float64(r.total) / time.Since(start).Seconds()
	slices.Sort(lat)
	if len(lat) > 0 {
		r.p50, r.p99 = lat[len(lat)/2].Round(time.Microsecond), lat[len(lat)*99/100].Round(time.Microsecond)
	}
	return r
}

// newClient returns an exchange function owned by one worker goroutine and a
// func that closes its connections, so each phase starts from a clean server.
func newClient(proto string, clientTLS *tls.Config) (exchangeFunc, func()) {
	addr := func(port int) string { return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) }
	switch proto {
	case "udp":
		c := dns.NewClient()
		return func(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
			in, _, err := c.Exchange(ctx, m, "udp", addr(portDNS))
			return in, err
		}, func() {}
	case "tcp", "dot":
		c := dns.NewClient()
		var conn net.Conn
		return func(ctx context.Context, m *dns.Msg) (in *dns.Msg, err error) {
				for range 2 { // the server closes connections after MaxTCPQueries; redial once
					if conn == nil {
						if proto == "dot" {
							conn, err = tls.Dial("tcp", addr(portDoT), clientTLS)
						} else {
							conn, err = net.Dial("tcp", addr(portDNS))
						}
						if err != nil {
							return nil, err
						}
					}
					if in, _, err = c.ExchangeWithConn(ctx, m.Copy(), conn); err == nil {
						return in, nil
					}
					_ = conn.Close()
					conn = nil
				}
				return nil, err
			}, func() {
				if conn != nil {
					_ = conn.Close()
				}
			}
	case "doh", "doh3":
		url := "https://" + addr(portDoH) + "/dns-query"
		var rt http.RoundTripper = &http.Transport{TLSClientConfig: clientTLS.Clone(), ForceAttemptHTTP2: true}
		if proto == "doh3" {
			url = "https://" + addr(portDoH3) + "/dns-query"
			rt = &http3.Transport{TLSClientConfig: clientTLS.Clone()}
		}
		hc := &http.Client{Transport: rt}
		return func(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
				m.ID = 0
				if err := m.Pack(); err != nil {
					return nil, err
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(m.Data))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", "application/dns-message")
				resp, err := hc.Do(req)
				if err != nil {
					return nil, err
				}
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
				}
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, err
				}
				out := &dns.Msg{Data: body}
				return out, out.Unpack()
			}, func() {
				if c, ok := rt.(io.Closer); ok { // http3.Transport
					_ = c.Close()
				}
				hc.CloseIdleConnections()
			}
	case "doq":
		tlsConf := clientTLS.Clone()
		tlsConf.NextProtos, tlsConf.MinVersion = []string{"doq"}, tls.VersionTLS13
		var conn *quic.Conn
		return func(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
				if conn == nil || conn.Context().Err() != nil {
					c, err := quic.DialAddr(ctx, addr(portDoQ), tlsConf, &quic.Config{KeepAlivePeriod: 15 * time.Second})
					if err != nil {
						return nil, err
					}
					conn = c
				}
				m.ID = 0 // RFC 9250
				if err := m.Pack(); err != nil {
					return nil, err
				}
				stream, err := conn.OpenStreamSync(ctx)
				if err != nil {
					return nil, err
				}
				defer stream.CancelRead(0)
				if d, ok := ctx.Deadline(); ok {
					_ = stream.SetDeadline(d)
				}
				if _, err := stream.Write(binary.BigEndian.AppendUint16(nil, uint16(len(m.Data)))); err != nil { //nolint:gosec // DNS message <= 64KiB
					return nil, err
				}
				if _, err := stream.Write(m.Data); err != nil {
					return nil, err
				}
				_ = stream.Close()
				var l [2]byte
				if _, err := io.ReadFull(stream, l[:]); err != nil {
					return nil, err
				}
				out := &dns.Msg{Data: make([]byte, binary.BigEndian.Uint16(l[:]))}
				if _, err := io.ReadFull(stream, out.Data); err != nil {
					return nil, err
				}
				return out, out.Unpack()
			}, func() {
				if conn != nil {
					_ = conn.CloseWithError(0, "")
				}
			}
	}
	die("unknown protocol %q", proto)
	return nil, nil
}

func startStubUpstream() string {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	must(err)
	srv := &dns.Server{Net: "udp", PacketConn: pc, Handler: dns.HandlerFunc(func(_ context.Context, w dns.ResponseWriter, r *dns.Msg) {
		resp := r.Copy()
		resp.Response, resp.RecursionAvailable = true, true
		resp.Answer = []dns.RR{&dns.A{
			Hdr: dns.Header{Name: r.Question[0].Header().Name, Class: dns.ClassINET, TTL: 30},
			A:   rdata.A{Addr: netip.MustParseAddr("192.0.2.1")},
		}}
		if err := resp.Pack(); err == nil {
			_, _ = io.Copy(w, resp)
		}
	})}
	go func() { _ = srv.ListenAndServe() }()
	return pc.LocalAddr().String()
}

// writeConfig writes a self-signed cert and goaway's config/settings.yaml into
// dir (goaway reads ./config/settings.yaml) and returns the CA pool for clients.
func writeConfig(dir, upstream string) *x509.CertPool {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	must(err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	must(err)
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	must(os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	must(os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))

	// Let goaway's settings package write its defaults, then override.
	wd, err := os.Getwd()
	must(err)
	must(os.Chdir(dir))
	cfg, err := settings.LoadSettings()
	must(err)
	cfg.DNS.Address = "127.0.0.1"
	cfg.DNS.Ports = settings.PortsConfig{TCPUDP: portDNS, DoT: portDoT, DoH: portDoH, DoQ: portDoQ, DoH3: portDoH3}
	cfg.DNS.TLS = settings.TLSConfig{Enabled: true, Cert: certPath, Key: keyPath}
	cfg.DNS.Upstream.Servers = []settings.UpstreamServer{{Name: "stub", Address: upstream, Protocol: "udp", Enabled: true}}
	cfg.DNS.RateLimit.Enabled = false // measuring resource stability, not the limiter
	cfg.API.Port = portAPI
	cfg.API.Authentication = false
	cfg.Misc.Dashboard = false
	cfg.Misc.ScheduledBlacklistUpdates = false
	cfg.Logging.Level = 2 // WARNING
	cfg.Save()
	must(os.Chdir(wd))

	pool := x509.NewCertPool()
	pool.AddCert(must2(x509.ParseCertificate(der)))
	return pool
}

func startGoaway(bin, dir string) (pid int, stop func()) {
	logFile, err := os.Create(filepath.Join(os.TempDir(), "goaway-soak.log"))
	must(err)
	cmd := exec.Command(bin, "--log-level=2", "--auth=false", "--dashboard=false") // #nosec G204 - operator-supplied binary
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, logFile, logFile
	must(cmd.Start())
	fmt.Fprintf(os.Stderr, "goaway pid %d, log %s\n", cmd.Process.Pid, logFile.Name())
	var once sync.Once
	return cmd.Process.Pid, func() {
		once.Do(func() {
			_ = cmd.Process.Signal(os.Interrupt)
			done := make(chan struct{})
			go func() { _ = cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
			}
			_ = logFile.Close()
		})
	}
}

func waitReady(exchange exchangeFunc) {
	deadline := time.Now().Add(90 * time.Second)
	last := "no attempt"
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		resp, err := exchange(ctx, dns.NewMsg("ready.soak.test.", dns.TypeA))
		cancel()
		switch g := goroutines(); {
		case err != nil:
			last = "dns: " + err.Error()
		case resp.Rcode != dns.RcodeSuccess:
			last = "dns rcode " + dns.RcodeToString[resp.Rcode]
		case g == 0:
			last = "no go_goroutines on /metrics"
		default:
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	die("goaway did not become ready (%s); see %s", last, filepath.Join(os.TempDir(), "goaway-soak.log"))
}

// rss returns the resident set size of pid in bytes (ps works on Linux and macOS).
func rss(pid int) int64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output() // #nosec G204 - fixed command
	if err != nil {
		return 0
	}
	kb, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return kb * 1024
}

// goroutines reads go_goroutines from goaway's Prometheus endpoint.
func goroutines() int {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", portAPI))
	if err != nil {
		return 0
	}
	defer func() { _ = resp.Body.Close() }()
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "go_goroutines "); ok {
			f, _ := strconv.ParseFloat(v, 64)
			return int(f)
		}
	}
	return 0
}

func mib(b int64) float64 { return float64(b) / (1 << 20) }

func must(err error) {
	if err != nil {
		die("%v", err)
	}
}

func must2[T any](v T, err error) T { must(err); return v }

// cleanup stops goaway; die runs it because os.Exit skips deferred calls.
var cleanup = func() {}

func die(format string, args ...any) {
	cleanup()
	fmt.Fprintf(os.Stderr, "soak: "+format+"\n", args...)
	os.Exit(1)
}
