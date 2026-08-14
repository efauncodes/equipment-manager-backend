package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenCreatesAndMigratesPersistentDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "equipment.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var migrations int
	if err := second.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 1 {
		t.Fatalf("migrations = %d, want 1", migrations)
	}
	var foreignKeys int
	if err := second.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	for _, table := range []string{"users", "equipment", "issuances", "returns", "magic_links", "sessions", "equipment_history"} {
		var name string
		if err := second.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("table %s: %v", table, err)
		}
	}
}

func TestDataSurvivesCloseAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "equipment.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = first.Exec(`INSERT INTO users(id,email,display_name,role,is_active,created_at,updated_at) VALUES('user-1','member@example.com','Member','mitglied',1,'2026-08-14 10:00:00','2026-08-14 10:00:00')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var email string
	if err := second.QueryRow(`SELECT email FROM users WHERE id='user-1'`).Scan(&email); err != nil {
		t.Fatal(err)
	}
	if email != "member@example.com" {
		t.Fatalf("email = %q", email)
	}
}
