package server

import (
	"encoding/csv"
	"encoding/json"
	model "goaway/backend/dns/server/models"
	"goaway/backend/settings"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type exportedQueryLog struct {
	Timestamp      string  `json:"timestamp"`
	ClientIP       string  `json:"client_ip"`
	ClientName     string  `json:"client_name"`
	Domain         string  `json:"domain"`
	QueryType      string  `json:"query_type"`
	Status         string  `json:"status"`
	DNSSECStatus   string  `json:"dnssec_status"`
	Protocol       string  `json:"protocol"`
	Blocked        bool    `json:"blocked"`
	Cached         bool    `json:"cached"`
	ResponseTimeMs float64 `json:"response_time_ms"`
}

func (s *DNSServer) exportQueryLogs(entries []model.RequestLogEntry) {
	if s == nil || s.Config == nil || len(entries) == 0 {
		return
	}
	s.Config.RLockConfig()
	cfg := s.Config.Logging.QueryLogExport
	s.Config.RUnlockConfig()

	if !cfg.Enabled || strings.TrimSpace(cfg.Target) == "" {
		return
	}
	if err := writeQueryLogExport(cfg, entries); err != nil {
		log.Warning("Failed to export query logs: %v", err)
	}
}

func writeQueryLogExport(cfg settings.QueryLogExportConfig, entries []model.RequestLogEntry) error {
	target := strings.TrimSpace(cfg.Target)
	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format == "postgres" || format == "timescaledb" {
		return writeQueryLogPostgres(target, entries)
	}

	var out io.Writer
	var needHeader bool

	if strings.EqualFold(target, "stdout") {
		out = os.Stdout
	} else {
		cleanPath := filepath.Clean(target)
		// #nosec G304 - export target path comes from operator-controlled settings.yaml
		f, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if info, statErr := f.Stat(); statErr == nil && info.Size() == 0 {
			needHeader = true
		}
		out = f
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "csv":
		w := csv.NewWriter(out)
		if needHeader {
			if err := w.Write([]string{
				"timestamp", "client_ip", "client_name", "domain", "query_type",
				"status", "dnssec_status", "protocol", "blocked", "cached", "response_time_ms",
			}); err != nil {
				return err
			}
		}
		for _, e := range entries {
			rec := toExportedQueryLog(e)
			if err := w.Write([]string{
				rec.Timestamp,
				rec.ClientIP,
				rec.ClientName,
				rec.Domain,
				rec.QueryType,
				rec.Status,
				rec.DNSSECStatus,
				rec.Protocol,
				strconv.FormatBool(rec.Blocked),
				strconv.FormatBool(rec.Cached),
				strconv.FormatFloat(rec.ResponseTimeMs, 'f', 3, 64),
			}); err != nil {
				return err
			}
		}
		w.Flush()
		return w.Error()
	default:
		enc := json.NewEncoder(out)
		for _, e := range entries {
			if err := enc.Encode(toExportedQueryLog(e)); err != nil {
				return err
			}
		}
		return nil
	}
}

func toExportedQueryLog(e model.RequestLogEntry) exportedQueryLog {
	clientIP := ""
	clientName := ""
	if e.ClientInfo != nil {
		clientIP = e.ClientInfo.IP.String()
		clientName = e.ClientInfo.Name
	}
	return exportedQueryLog{
		Timestamp:      e.Timestamp.UTC().Format(time.RFC3339Nano),
		ClientIP:       clientIP,
		ClientName:     clientName,
		Domain:         e.Domain,
		QueryType:      e.QueryType,
		Status:         e.Status,
		DNSSECStatus:   e.DNSSECStatus,
		Protocol:       string(e.Protocol),
		Blocked:        e.Blocked,
		Cached:         e.Cached,
		ResponseTimeMs: float64(e.ResponseTime.Microseconds()) / 1000.0,
	}
}
