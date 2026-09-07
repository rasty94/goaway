package cluster

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"
)

const (
	HeaderTimestamp = "X-Cluster-Timestamp"
	HeaderSignature = "X-Cluster-Signature"
	// SignatureWindow bounds clock skew between nodes and the replay window.
	// ponytail: no nonce cache; a replay inside the window re-applies an idempotent event. Add a nonce LRU if that ever matters.
	SignatureWindow = 30 * time.Second
)

func sign(secret, timestamp, path string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// SignRequest stamps the request with a timestamp and an HMAC-SHA256 over timestamp+path+body.
func SignRequest(req *http.Request, secret string, body []byte) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, sign(secret, ts, req.URL.Path, body))
}

// VerifyRequest checks the signature headers against path, body and secret. Fails closed on empty secret.
func VerifyRequest(h http.Header, secret, path string, body []byte, now time.Time) error {
	if secret == "" {
		return errors.New("cluster secret not configured")
	}
	ts := h.Get(HeaderTimestamp)
	sig := h.Get(HeaderSignature)
	if ts == "" || sig == "" {
		return errors.New("missing cluster signature")
	}
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("invalid cluster timestamp")
	}
	if d := now.Sub(time.Unix(unix, 0)); d > SignatureWindow || d < -SignatureWindow {
		return errors.New("cluster timestamp outside allowed window")
	}
	if !hmac.Equal([]byte(sig), []byte(sign(secret, ts, path, body))) {
		return errors.New("invalid cluster signature")
	}
	return nil
}
