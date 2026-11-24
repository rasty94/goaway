package server

import (
	"bufio"
	"fmt"
	"goaway/backend/settings"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/rdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startMockRESPServer(t *testing.T, expectedPass string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	var mu sync.Mutex
	store := make(map[string]string)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimSpace(line)
					if !strings.HasPrefix(line, "*") {
						return
					}
					argc, err := strconv.Atoi(line[1:])
					if err != nil {
						return
					}
					args := make([]string, 0, argc)
					for range argc {
						lenLine, err := r.ReadString('\n')
						if err != nil {
							return
						}
						n, err := strconv.Atoi(strings.TrimSpace(lenLine)[1:])
						if err != nil {
							return
						}
						buf := make([]byte, n+2)
						if _, err := io.ReadFull(r, buf); err != nil {
							return
						}
						args = append(args, string(buf[:n]))
					}

					cmd := strings.ToUpper(args[0])
					switch cmd {
					case "AUTH":
						if expectedPass == "" || (len(args) > 1 && args[1] == expectedPass) {
							_, _ = io.WriteString(c, "+OK\r\n")
						} else {
							_, _ = io.WriteString(c, "-ERR invalid password\r\n")
						}
					case "SELECT":
						_, _ = io.WriteString(c, "+OK\r\n")
					case "SET":
						mu.Lock()
						store[args[1]] = args[2]
						mu.Unlock()
						_, _ = io.WriteString(c, "+OK\r\n")
					case "GET":
						mu.Lock()
						val, ok := store[args[1]]
						mu.Unlock()
						if !ok {
							_, _ = io.WriteString(c, "$-1\r\n")
						} else {
							_, _ = fmt.Fprintf(c, "$%d\r\n%s\r\n", len(val), val)
						}
					default:
						_, _ = io.WriteString(c, "-ERR unknown command\r\n")
					}
				}
			}(conn)
		}
	}()

	return ln.Addr().String()
}

func TestRedisSharedCacheWriteThroughBetweenNodes(t *testing.T) {
	addr := startMockRESPServer(t, "secret123")

	cfg := &settings.Config{
		DNS: settings.DNSConfig{
			CacheEnabled: true,
			RedisCache: settings.RedisCacheConfig{
				Enabled:  true,
				Address:  addr,
				Password: "secret123",
				DB:       1,
				Prefix:   "test:dns:",
			},
		},
	}

	nodeA := &DNSServer{Config: cfg}
	nodeB := &DNSServer{Config: cfg}

	records := []dns.RR{
		&dns.A{
			Hdr: dns.Header{Name: "shared.example.com.", Class: dns.ClassINET, TTL: 120},
			A:   rdata.A{Addr: netip.MustParseAddr("5.6.7.8")},
		},
	}

	nodeA.redisCacheSet("shared.example.com.:A", "shared.example.com.", records, 120, DNSSECStatusSecure, "query")

	got, dnssecStatus, source, ok := nodeB.redisCacheGet("shared.example.com.:A")
	require.True(t, ok)
	assert.Equal(t, DNSSECStatusSecure, dnssecStatus)
	assert.Equal(t, "query", source)
	require.Len(t, got, 1)
	aRec, ok := got[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "5.6.7.8", aRec.Addr.String())

	// Node B should also have hydrated its local DomainCache
	_, cachedLocally := nodeB.DomainCache.Load("shared.example.com.:A")
	assert.True(t, cachedLocally)
}
