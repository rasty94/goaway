package cluster

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	HeaderTimestamp = "X-Cluster-Timestamp"
	HeaderNonce     = "X-Cluster-Nonce"
	HeaderSignature = "X-Cluster-Signature"
	// SignatureWindow bounds clock skew between nodes and the replay window.
	SignatureWindow = 30 * time.Second
)

// NonceCache tracks recently seen cluster nonces within SignatureWindow to prevent replay attacks.
// Its zero value is ready to use.
type NonceCache struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// CheckAndRecord rejects a nonce if it was already seen within SignatureWindow,
// pruning expired entries opportunistically.
func (c *NonceCache) CheckAndRecord(nonce string, now time.Time) error {
	if nonce == "" {
		return errors.New("missing cluster nonce")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.seen == nil {
		c.seen = make(map[string]time.Time)
	}

	cutoff := now.Add(-SignatureWindow)
	for k, ts := range c.seen {
		if ts.Before(cutoff) {
			delete(c.seen, k)
		}
	}

	if _, exists := c.seen[nonce]; exists {
		return errors.New("replayed cluster nonce")
	}
	c.seen[nonce] = now
	return nil
}

func generateNonce() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf[:])
}

func sign(secret, timestamp, nonce, path string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write([]byte(nonce))
	mac.Write([]byte("\n"))
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// SignHeaders stamps HTTP headers with timestamp, random nonce and an HMAC-SHA256 over timestamp+nonce+path+body.
func SignHeaders(h http.Header, secret, path string, body []byte) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce()
	h.Set(HeaderTimestamp, ts)
	h.Set(HeaderNonce, nonce)
	h.Set(HeaderSignature, sign(secret, ts, nonce, path, body))
}

// SignRequest stamps the request with a timestamp, nonce and an HMAC-SHA256 over timestamp+nonce+path+body.
func SignRequest(req *http.Request, secret string, body []byte) {
	SignHeaders(req.Header, secret, req.URL.Path, body)
}

// VerifyRequest checks the signature headers against path, body and secret. Fails closed on empty secret.
func VerifyRequest(h http.Header, secret, path string, body []byte, now time.Time) error {
	if secret == "" {
		return errors.New("cluster secret not configured")
	}
	ts := h.Get(HeaderTimestamp)
	nonce := h.Get(HeaderNonce)
	sig := h.Get(HeaderSignature)
	if ts == "" || nonce == "" || sig == "" {
		return errors.New("missing cluster signature")
	}
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("invalid cluster timestamp")
	}
	if d := now.Sub(time.Unix(unix, 0)); d > SignatureWindow || d < -SignatureWindow {
		return errors.New("cluster timestamp outside allowed window")
	}
	if !hmac.Equal([]byte(sig), []byte(sign(secret, ts, nonce, path, body))) {
		return errors.New("invalid cluster signature")
	}
	return nil
}

// VerifyRequestWithReplay verifies the HMAC signature and rejects replayed nonces within SignatureWindow.
func VerifyRequestWithReplay(h http.Header, secret, path string, body []byte, now time.Time, cache *NonceCache) error {
	if err := VerifyRequest(h, secret, path, body, now); err != nil {
		return err
	}
	if cache != nil {
		return cache.CheckAndRecord(h.Get(HeaderNonce), now)
	}
	return nil
}
