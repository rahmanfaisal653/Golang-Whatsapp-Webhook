package main

import (
	"context"
	"path/filepath"
	"testing"

	"go.mau.fi/whatsmeow/store/sqlstore"
)

func TestSQLiteStoreOpensWithoutCGO(t *testing.T) {
	store, err := sqlstore.New(
		context.Background(),
		"sqlite",
		sqliteURI(filepath.Join(t.TempDir(), "whatsmeow.db")),
		nil,
	)
	if err != nil {
		t.Fatalf("open SQLite store: %v", err)
	}
	defer store.Close()
}
