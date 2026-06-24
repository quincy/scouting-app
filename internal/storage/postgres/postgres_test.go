package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"scout-app/internal/testhelper"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	testDB = testhelper.StartDB()
	os.Exit(m.Run())
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
	truncateTables(t, tables...)
}
