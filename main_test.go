package main

import "testing"

func TestSQLiteURI(t *testing.T) {
	got := sqliteURI("src/session/whatsmeow.db")
	want := "file:src/session/whatsmeow.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	if got != want {
		t.Fatalf("sqliteURI() = %q, want %q", got, want)
	}
}
