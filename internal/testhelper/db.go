package testhelper

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	startOnce sync.Once
	globalDB  *sql.DB
)

func StartDB() *sql.DB {
	startOnce.Do(func() {
		db, err := startContainer()
		if err != nil {
			log.Fatalf("testhelper: start database container: %v", err)
		}
		globalDB = db
	})
	return globalDB
}

func startContainer() (*sql.DB, error) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "cockroachdb/cockroach:latest",
		ExposedPorts: []string{"26257/tcp"},
		Cmd:          []string{"start-single-node", "--insecure"},
		AutoRemove:   true,
		WaitingFor: wait.ForLog("CockroachDB node starting").
			WithStartupTimeout(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start cockroachdb container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "26257")
	if err != nil {
		return nil, fmt.Errorf("get mapped port: %w", err)
	}

	dsn := fmt.Sprintf("postgres://root@%s:%s/defaultdb?sslmode=disable", host, port.Port())

	var db *sql.DB
	for i := 0; i < 30; i++ {
		db, err = sql.Open("pgx", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to cockroachdb: %w", err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}

	migrationsDir := findMigrationsDir()
	if migrationsDir == "" {
		db.Close()
		return nil, fmt.Errorf("cannot find migrations directory")
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func findMigrationsDir() string {
	candidates := []string{
		filepath.Join("..", "migrations"),
		filepath.Join("..", "..", "migrations"),
		filepath.Join("..", "..", "..", "migrations"),
		filepath.Join(os.Getenv("PWD"), "migrations"),
	}
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

func TruncateAll(t TB, db *sql.DB) {
	t.Helper()
	tables := []string{
		"app_config",
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
	ctx := context.Background()
	for _, table := range tables {
		if _, err := db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("failed to truncate %s: %v", table, err)
		}
	}
}

type TB interface {
	Helper()
	Fatalf(format string, args ...interface{})
}
