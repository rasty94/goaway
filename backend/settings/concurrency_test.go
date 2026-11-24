package settings

import (
	"sync"
	"testing"
)

func TestConfigConcurrentUpdateAndDNSSnapshot(t *testing.T) {
	cfg := &Config{
		mu: &sync.RWMutex{},
		DNS: DNSConfig{
			CacheEnabled: true,
			Upstream: UpstreamConfig{
				Strategy: "strict",
				Servers: []UpstreamServer{
					{Name: "cf", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true},
				},
			},
		},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := range 200 {
			cfg.LockConfig()
			cfg.DNS.CacheEnabled = i%2 == 0
			cfg.DNS.EDEEnabled = i%3 == 0
			cfg.DNS.Upstream.Servers = []UpstreamServer{
				{Name: "cf", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true},
				{Name: "gg", Address: "8.8.8.8:53", Protocol: "udp", Enabled: i%2 == 0},
			}
			cfg.UnlockConfig()
		}
	}()

	go func() {
		defer wg.Done()
		for range 200 {
			snap := cfg.DNSSnapshot()
			_ = snap.CacheEnabled
			_ = snap.EDEEnabled
			for _, srv := range snap.Upstream.Servers {
				_ = srv.Address
			}
		}
	}()

	wg.Wait()
}
