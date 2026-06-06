package postgres

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://localhost:26257/scoutapp?sslmode=disable"
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Printf("Skipping postgres integration tests: cannot open database: %v", err)
		os.Exit(0)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		log.Printf("Skipping postgres integration tests: cannot ping database: %v", err)
		os.Exit(0)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		db.Close()
		log.Printf("Skipping postgres integration tests: goose dialect: %v", err)
		os.Exit(0)
	}

	migrationsDir := findMigrationsDir()
	if migrationsDir == "" {
		db.Close()
		log.Printf("Skipping postgres integration tests: cannot find migrations directory")
		os.Exit(0)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		db.Close()
		log.Printf("Skipping postgres integration tests: migration failed: %v", err)
		os.Exit(0)
	}

	testDB = db
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func findMigrationsDir() string {
	candidates := []string{
		filepath.Join("..", "..", "..", "migrations"),
		filepath.Join(os.Getenv("PWD"), "migrations"),
	}
	// Try walking up from the test file location
	wd, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(wd, "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		wd = filepath.Dir(wd)
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

func truncateTables(t *testing.T, tables ...string) {
	t.Helper()
	ctx := context.Background()
	for _, table := range tables {
		if _, err := testDB.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("failed to truncate %s: %v", table, err)
		}
	}
}

func truncateAll(t *testing.T) {
	t.Helper()
	tables := []string{
		"event_attendee_responsibilities",
		"event_attendees",
		"events",
		"parent_youth_links",
		"scoutbook_sessions",
		"otp_codes",
		"profiles",
		"user_roles",
		"role_permissions",
		"permissions",
		"roles",
		"users",
	}
	truncateTables(t, tables...)
}
