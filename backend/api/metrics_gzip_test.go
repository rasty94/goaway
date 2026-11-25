package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

// Prometheus scrapes with Accept-Encoding: gzip. Go's client undoes one gzip
// layer; the body must then be plain text, not gzipped a second time.
func TestMetricsNotDoubleGzipped(t *testing.T) {
	api := &API{}
	api.initializeRouter()
	api.router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	srv := httptest.NewServer(api.router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics") // Go adds Accept-Encoding: gzip and decodes it
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "go_goroutines ")
}
