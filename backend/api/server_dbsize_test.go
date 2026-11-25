package api

import (
	"testing"

	"goaway/backend/database"
	"goaway/backend/settings"
)

func TestGetDBSizeMBStateless(t *testing.T) {
	db, err := database.InitializeWithOptions(true)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &settings.Config{}
	cfg.Misc.Stateless = true
	api := &API{DBConn: db, Config: cfg}

	size, err := api.getDBSizeMB()
	if err != nil {
		t.Fatalf("stateless size: %v", err)
	}
	if size <= 0 {
		t.Fatalf("expected in-memory size > 0, got %v", size)
	}
}
