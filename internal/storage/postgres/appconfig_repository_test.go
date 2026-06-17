package postgres

import (
	"context"
	"testing"
)

func TestAppConfigRepository(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateTables(t, "app_config")

	repo := NewAppConfigRepository(testDB)
	ctx := context.Background()

	t.Run("Get unknown key returns empty string", func(t *testing.T) {
		val, err := repo.Get(ctx, "NONEXISTENT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "" {
			t.Errorf("expected empty string, got %q", val)
		}
	})

	t.Run("Set and Get round-trip", func(t *testing.T) {
		if err := repo.Set(ctx, "TEST_KEY", "test_value"); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		val, err := repo.Get(ctx, "TEST_KEY")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if val != "test_value" {
			t.Errorf("expected %q, got %q", "test_value", val)
		}
	})

	t.Run("Set overwrites existing value", func(t *testing.T) {
		if err := repo.Set(ctx, "TEST_KEY", "new_value"); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		val, err := repo.Get(ctx, "TEST_KEY")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if val != "new_value" {
			t.Errorf("expected %q, got %q", "new_value", val)
		}
	})

	t.Run("All returns all entries", func(t *testing.T) {
		_ = repo.Set(ctx, "KEY_A", "val_a")
		_ = repo.Set(ctx, "KEY_B", "val_b")

		all, err := repo.All(ctx)
		if err != nil {
			t.Fatalf("All failed: %v", err)
		}
		if all["KEY_A"] != "val_a" || all["KEY_B"] != "val_b" {
			t.Errorf("unexpected values: %v", all)
		}
	})
}
