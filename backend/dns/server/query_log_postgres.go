package server

import (
	"database/sql"
	"fmt"
	model "goaway/backend/dns/server/models"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

const createQueryLogsTableSQL = `
CREATE TABLE IF NOT EXISTS goaway_query_logs (
	timestamp TEXT NOT NULL,
	client_ip TEXT NOT NULL,
	client_name TEXT NOT NULL,
	domain TEXT NOT NULL,
	query_type TEXT NOT NULL,
	status TEXT NOT NULL,
	dnssec_status TEXT NOT NULL,
	protocol TEXT NOT NULL,
	blocked BOOLEAN NOT NULL,
	cached BOOLEAN NOT NULL,
	response_time_ms DOUBLE PRECISION NOT NULL
);`

const insertQueryLogSQL = `
INSERT INTO goaway_query_logs (
	timestamp, client_ip, client_name, domain, query_type,
	status, dnssec_status, protocol, blocked, cached, response_time_ms
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);`

var (
	pgSinkMu    sync.Mutex
	pgSinkPools = map[string]*sql.DB{}
)

func getPostgresLogDB(dsn string) (*sql.DB, error) {
	pgSinkMu.Lock()
	defer pgSinkMu.Unlock()

	if db, ok := pgSinkPools[dsn]; ok {
		return db, nil
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres log sink: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if _, err := db.Exec(createQueryLogsTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize goaway_query_logs table: %w", err)
	}

	pgSinkPools[dsn] = db
	return db, nil
}

func insertQueryLogBatch(db *sql.DB, entries []model.RequestLogEntry) error {
	if _, err := db.Exec(createQueryLogsTableSQL); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(insertQueryLogSQL)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		rec := toExportedQueryLog(e)
		if _, err := stmt.Exec(
			rec.Timestamp,
			rec.ClientIP,
			rec.ClientName,
			rec.Domain,
			rec.QueryType,
			rec.Status,
			rec.DNSSECStatus,
			rec.Protocol,
			rec.Blocked,
			rec.Cached,
			rec.ResponseTimeMs,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func writeQueryLogPostgres(dsn string, entries []model.RequestLogEntry) error {
	db, err := getPostgresLogDB(dsn)
	if err != nil {
		return err
	}
	return insertQueryLogBatch(db, entries)
}
