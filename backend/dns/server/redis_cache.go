package server

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"goaway/backend/settings"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"codeberg.org/miekg/dns"
)

const redisIOTimeout = 300 * time.Millisecond

type redisCachedPayload struct {
	Domain       string    `json:"domain"`
	DNSSECStatus string    `json:"dnssecStatus"`
	Source       string    `json:"source"`
	ExpiresAt    time.Time `json:"expiresAt"`
	RecordsB64   []string  `json:"records"`
}

func redisKey(cfg settings.RedisCacheConfig, key string) string {
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = "goaway:dns:"
	}
	return prefix + key
}

func writeRESPCommand(w io.Writer, args ...string) error {
	var sb strings.Builder
	sb.WriteString("*")
	sb.WriteString(strconv.Itoa(len(args)))
	sb.WriteString("\r\n")
	for _, a := range args {
		sb.WriteString("$")
		sb.WriteString(strconv.Itoa(len(a)))
		sb.WriteString("\r\n")
		sb.WriteString(a)
		sb.WriteString("\r\n")
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

func readRESPValue(r *bufio.Reader) (string, bool, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", false, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if line == "" {
		return "", false, fmt.Errorf("empty RESP line")
	}
	switch line[0] {
	case '+':
		return line[1:], true, nil
	case '-':
		return "", false, fmt.Errorf("redis error: %s", line[1:])
	case ':':
		return line[1:], true, nil
	case '$':
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return "", false, err
		}
		if n < 0 {
			return "", false, nil // nil bulk string (cache miss)
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", false, err
		}
		return string(buf[:n]), true, nil
	default:
		return "", false, fmt.Errorf("unsupported RESP prefix %q", line[0])
	}
}

func execRedisCommand(cfg settings.RedisCacheConfig, args ...string) (string, bool, error) {
	conn, err := net.DialTimeout("tcp", cfg.Address, redisIOTimeout)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(redisIOTimeout))

	reader := bufio.NewReader(conn)
	if cfg.Password != "" {
		if err := writeRESPCommand(conn, "AUTH", cfg.Password); err != nil {
			return "", false, err
		}
		if _, _, err := readRESPValue(reader); err != nil {
			return "", false, err
		}
	}
	if cfg.DB > 0 {
		if err := writeRESPCommand(conn, "SELECT", strconv.Itoa(cfg.DB)); err != nil {
			return "", false, err
		}
		if _, _, err := readRESPValue(reader); err != nil {
			return "", false, err
		}
	}
	if err := writeRESPCommand(conn, args...); err != nil {
		return "", false, err
	}
	return readRESPValue(reader)
}

func (s *DNSServer) redisCacheSet(cacheKey, domain string, records []dns.RR, ttl uint32, dnssecStatus, source string) {
	if s == nil || s.Config == nil || len(records) == 0 {
		return
	}
	cfg := s.Config.DNSSnapshot().RedisCache
	if !cfg.Enabled || strings.TrimSpace(cfg.Address) == "" {
		return
	}

	encoded := make([]string, 0, len(records))
	for _, rr := range records {
		m := new(dns.Msg)
		m.Answer = []dns.RR{rr}
		if err := m.Pack(); err != nil {
			continue
		}
		encoded = append(encoded, base64.StdEncoding.EncodeToString(m.Data))
	}
	if len(encoded) == 0 {
		return
	}

	ttlSec := int(ttl)
	if ttlSec <= 0 {
		ttlSec = 60
	}

	payload, err := json.Marshal(redisCachedPayload{
		Domain:       domain,
		DNSSECStatus: dnssecStatus,
		Source:       source,
		ExpiresAt:    time.Now().Add(time.Duration(ttlSec) * time.Second),
		RecordsB64:   encoded,
	})
	if err != nil {
		return
	}

	if _, _, err := execRedisCommand(cfg, "SET", redisKey(cfg, cacheKey), string(payload), "EX", strconv.Itoa(ttlSec)); err != nil {
		log.Debug("Redis cache SET failed for %s: %v", cacheKey, err)
	}
}

func (s *DNSServer) redisCacheGet(cacheKey string) ([]dns.RR, string, string, bool) {
	if s == nil || s.Config == nil {
		return nil, "", "", false
	}
	cfg := s.Config.DNSSnapshot().RedisCache
	if !cfg.Enabled || strings.TrimSpace(cfg.Address) == "" {
		return nil, "", "", false
	}

	raw, found, err := execRedisCommand(cfg, "GET", redisKey(cfg, cacheKey))
	if err != nil || !found {
		return nil, "", "", false
	}

	var payload redisCachedPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, "", "", false
	}
	if time.Now().After(payload.ExpiresAt) {
		return nil, "", "", false
	}

	records := make([]dns.RR, 0, len(payload.RecordsB64))
	for _, b64 := range payload.RecordsB64 {
		wire, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			continue
		}
		m := new(dns.Msg)
		m.Data = wire
		if err := m.Unpack(); err != nil || len(m.Answer) == 0 {
			continue
		}
		records = append(records, m.Answer[0])
	}
	if len(records) == 0 {
		return nil, "", "", false
	}

	remSec := min(86400, max(1, int(time.Until(payload.ExpiresAt).Seconds())))
	// #nosec G115 - remSec is explicitly clamped to [1, 86400]
	remaining := uint32(remSec)
	s.DomainCache.Store(cacheKey, CachedRecord{
		IPAddresses:  records,
		ExpiresAt:    time.Now().Add(time.Duration(remaining) * time.Second),
		CachedAt:     time.Now(),
		OriginalTTL:  remaining,
		Key:          cacheKey,
		Domain:       payload.Domain,
		DNSSECStatus: payload.DNSSECStatus,
		Source:       payload.Source,
	})
	return records, payload.DNSSECStatus, payload.Source, true
}
