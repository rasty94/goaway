package cluster

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"goaway/backend/settings"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// NewPeerHTTPClient builds an HTTP client configured with optional custom CA and mTLS client
// certificates for node-to-node HA traffic. Falls back to standard TLS 1.2+ defaults when disabled.
func NewPeerHTTPClient(cfg settings.ClusterTLSConfig, timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	tlsConf := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.Enabled {
		if cfg.CACert != "" {
			cleanCA := filepath.Clean(cfg.CACert)
			// #nosec G304 - CA cert path comes from operator-controlled settings.yaml
			pemData, err := os.ReadFile(cleanCA)
			if err != nil {
				return nil, fmt.Errorf("failed to read cluster CA cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pemData) {
				return nil, fmt.Errorf("failed to parse cluster CA cert PEM")
			}
			tlsConf.RootCAs = pool
		}

		if cfg.ClientCert != "" || cfg.ClientKey != "" {
			cert, err := tls.LoadX509KeyPair(filepath.Clean(cfg.ClientCert), filepath.Clean(cfg.ClientKey))
			if err != nil {
				return nil, fmt.Errorf("failed to load cluster mTLS client keypair: %w", err)
			}
			tlsConf.Certificates = []tls.Certificate{cert}
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConf
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}
