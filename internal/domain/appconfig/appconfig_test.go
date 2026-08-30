package appconfig

import (
	"context"
	"errors"
	"os"
	"testing"
)

type errorRepository struct{}

func (e *errorRepository) Get(ctx context.Context, key string) (string, error) {
	return "", errors.New("db error")
}
func (e *errorRepository) Set(ctx context.Context, key, value string) error   { return nil }
func (e *errorRepository) All(ctx context.Context) (map[string]string, error) { return nil, nil }

func setenv(t *testing.T, key, val string) {
	t.Helper()
	os.Setenv(key, val)
	t.Cleanup(func() { os.Unsetenv(key) })
}

func TestInMemoryRepository(t *testing.T) {
	repo := NewInMemoryRepository()

	t.Run("Get unknown key returns empty string", func(t *testing.T) {
		val, err := repo.Get(context.Background(), "NONEXISTENT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "" {
			t.Errorf("expected empty string, got %q", val)
		}
	})

	t.Run("Set and Get round-trip", func(t *testing.T) {
		if err := repo.Set(context.Background(), "KEY_ONE", "value1"); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		val, err := repo.Get(context.Background(), "KEY_ONE")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if val != "value1" {
			t.Errorf("expected %q, got %q", "value1", val)
		}
	})

	t.Run("Set overwrites existing key", func(t *testing.T) {
		if err := repo.Set(context.Background(), "KEY_ONE", "value2"); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		val, err := repo.Get(context.Background(), "KEY_ONE")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if val != "value2" {
			t.Errorf("expected %q, got %q", "value2", val)
		}
	})

	t.Run("All returns all key-value pairs", func(t *testing.T) {
		repo2 := NewInMemoryRepository()
		_ = repo2.Set(context.Background(), "A", "1")
		_ = repo2.Set(context.Background(), "B", "2")

		all, err := repo2.All(context.Background())
		if err != nil {
			t.Fatalf("All failed: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("expected 2 entries, got %d", len(all))
		}
		if all["A"] != "1" || all["B"] != "2" {
			t.Errorf("unexpected values: %v", all)
		}
	})
}

func TestGetWithHierarchy_EnvVarTakesPriority(t *testing.T) {
	ctx := t.Context()
	repo := NewInMemoryRepository()
	_ = repo.Set(ctx, "TEST_KEY", "db-value")

	setenv(t, "TEST_ENV", "env-value")

	got := GetWithHierarchy(ctx, repo, "TEST_ENV", "TEST_KEY", "default-val")
	if got != "env-value" {
		t.Errorf("expected env-value, got %q", got)
	}
}

func TestGetWithHierarchy_DBValueUsedWhenNoEnvVar(t *testing.T) {
	ctx := t.Context()
	repo := NewInMemoryRepository()
	_ = repo.Set(ctx, "TEST_KEY", "db-value")

	got := GetWithHierarchy(ctx, repo, "TEST_ENV_UNSET", "TEST_KEY", "default-val")
	if got != "db-value" {
		t.Errorf("expected db-value, got %q", got)
	}
}

func TestGetWithHierarchy_DefaultWhenNeitherSet(t *testing.T) {
	ctx := t.Context()
	repo := NewInMemoryRepository()

	got := GetWithHierarchy(ctx, repo, "TEST_ENV_UNSET", "TEST_KEY_UNSET", "default-val")
	if got != "default-val" {
		t.Errorf("expected default-val, got %q", got)
	}
}

func TestGetWithHierarchy_DBErrorFallsBackToDefault(t *testing.T) {
	repo := &errorRepository{}
	got := GetWithHierarchy(t.Context(), repo, "TEST_ENV_UNSET", "TEST_KEY", "default-val")
	if got != "default-val" {
		t.Errorf("expected default-val, got %q", got)
	}
}

func TestGetWithHierarchy_EmptyDBValueFallsBackToDefault(t *testing.T) {
	ctx := t.Context()
	repo := NewInMemoryRepository()
	_ = repo.Set(ctx, "TEST_KEY", "")

	got := GetWithHierarchy(ctx, repo, "TEST_ENV_UNSET", "TEST_KEY", "default-val")
	if got != "default-val" {
		t.Errorf("expected default-val, got %q", got)
	}
}

func TestDefaultKeyNames(t *testing.T) {
	if KeyOnboardingComplete != "ONBOARDING_COMPLETE" {
		t.Errorf("expected ONBOARDING_COMPLETE, got %q", KeyOnboardingComplete)
	}
	if KeyScoutbookOrgGUID != "SCOUTBOOK_ORG_GUID" {
		t.Errorf("expected SCOUTBOOK_ORG_GUID, got %q", KeyScoutbookOrgGUID)
	}
	if KeyUnitType != "UNIT_TYPE" {
		t.Errorf("expected UNIT_TYPE, got %q", KeyUnitType)
	}
	if KeyUnitNumber != "UNIT_NUMBER" {
		t.Errorf("expected UNIT_NUMBER, got %q", KeyUnitNumber)
	}
	if KeyDefaultTimezone != "DEFAULT_TIMEZONE" {
		t.Errorf("expected DEFAULT_TIMEZONE, got %q", KeyDefaultTimezone)
	}
	if KeyMaxTentAgeGap != "MAX_TENT_AGE_GAP" {
		t.Errorf("expected MAX_TENT_AGE_GAP, got %q", KeyMaxTentAgeGap)
	}
}
