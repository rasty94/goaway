package server

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func (s *DNSServer) exchangeWithProtocol(msg *dns.Msg, addr, proto string) (*dns.Msg, error) {
	switch strings.ToLower(proto) {
	case "udp":
		client := &dns.Client{Net: "udp", Timeout: 5 * time.Second}
		in, _, err := client.Exchange(msg, addr)
		return in, err
	case "tcp":
		client := &dns.Client{Net: "tcp", Timeout: 5 * time.Second}
		in, _, err := client.Exchange(msg, addr)
		return in, err
	case "dot":
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
			addr = net.JoinHostPort(addr, "853")
		} else if port == "53" {
			addr = net.JoinHostPort(host, "853")
		}
		client := &dns.Client{
			Net: "tcp-tls",
			TLSConfig: &tls.Config{
				InsecureSkipVerify: false,
				ServerName:         host,
			},
			Timeout: 5 * time.Second,
		}
		in, _, err := client.Exchange(msg, addr)
		return in, err
	case "doh":
		return s.exchangeDoH(msg, addr)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", proto)
	}
}

func (s *DNSServer) exchangeDoH(msg *dns.Msg, url string) (*dns.Msg, error) {
	pack, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(pack))
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
	if err := response.Unpack(body); err != nil {
		return nil, err
	}

	return response, nil
}
