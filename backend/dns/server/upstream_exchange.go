package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
)

func (s *DNSServer) exchangeWithProtocol(msg *dns.Msg, addr, proto string) (*dns.Msg, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch strings.ToLower(proto) {
	case "udp":
		client := dns.NewClient()
		in, _, err := client.Exchange(ctx, msg, "udp", addr)
		return in, err
	case "tcp":
		client := dns.NewClient()
		in, _, err := client.Exchange(ctx, msg, "tcp", addr)
		return in, err
	case "dot":
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
			addr = net.JoinHostPort(addr, "853")
		} else if port == "53" {
			addr = net.JoinHostPort(host, "853")
		}
		client := dns.NewClient()
		client.Transport.TLSConfig = &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         host,
		}
		in, _, err := client.Exchange(ctx, msg, "tcp", addr)
		return in, err
	case "doh":
		return s.exchangeDoH(msg, addr)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", proto)
	}
}

func (s *DNSServer) exchangeDoH(msg *dns.Msg, url string) (*dns.Msg, error) {
	if err := msg.Pack(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(msg.Data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	response := new(dns.Msg)
	response.Data = body
	if err := response.Unpack(); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *DNSServer) ProbeUpstream(server string) *UpstreamHealth {
	msg := dns.NewMsg(dnsutil.Fqdn("."), dns.TypeNS)
	msg.RecursionDesired = true

	parts := strings.Split(server, ":")
	proto := "udp"
	addr := server
	if len(parts) > 1 && (parts[0] == "https" || parts[0] == "http") {
		proto = "doh"
	} else if len(parts) > 1 {
		// handle proto:addr or addr:port
		// simpler: if it contains '/', it's DoH URL. if it contains tls://, it's DoT.
		if strings.Contains(server, "://") {
			p := strings.Split(server, "://")
			proto = p[0]
			addr = p[1]
		}
	}

	start := time.Now()
	_, err := s.exchangeWithProtocol(msg, addr, proto)
	latency := time.Since(start)

	status := "Healthy"
	if err != nil {
		status = "Unreachable"
		log.Debug("Upstream %s is Unreachable: %v", server, err)
	} else if latency > 500*time.Millisecond {
		status = "Slow"
	}

	health := &UpstreamHealth{
		Server:    server,
		Status:    status,
		Latency:   latency,
		LastCheck: time.Now(),
	}
	s.UpstreamHealth.Store(server, health)
	return health
}
