package postgres

import (
	"context"
	"testing"
	"time"

	"scout-app/internal/domain/scoutbooksession"
)

func TestPostgresScoutbookSessionRepository_CreateAndGetLatest(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewScoutbookSessionRepository(testDB)
	ctx := context.Background()

	s1 := &scoutbooksession.Session{
		Token:      "token-1",
		PersonGUID: "guid-1",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}
	s2 := &scoutbooksession.Session{
		Token:      "token-2",
		PersonGUID: "guid-2",
		ExpiresAt:  time.Now().Add(2 * time.Hour),
	}

	if err := repo.Create(ctx, s1); err != nil {
		t.Fatalf("Create s1 failed: %v", err)
	}
	if err := repo.Create(ctx, s2); err != nil {
		t.Fatalf("Create s2 failed: %v", err)
	}

	latest, err := repo.GetLatest(ctx)
	if err != nil {
		t.Fatalf("GetLatest failed: %v", err)
	}
	if latest.Token != "token-2" {
		t.Errorf("expected latest token token-2, got %s", latest.Token)
	}
}

func TestPostgresScoutbookSessionRepository_GetLatest_Empty(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewScoutbookSessionRepository(testDB)
	ctx := context.Background()

	_, err := repo.GetLatest(ctx)
	if err == nil {
		t.Error("expected error for empty sessions, got nil")
	}
}
