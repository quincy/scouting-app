package storage

import (
	"context"
	"testing"
	"time"

	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
	"scout-app/internal/storage/postgres"
	"scout-app/internal/testhelper"
)

func TestUserRepository_CreateAndGetByID(t *testing.T) {
	db := testhelper.StartDB()
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	u := &user.User{
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}

	err := repo.Create(ctx, u)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if u.ID == "" {
		t.Errorf("expected generated UUID ID to be set on user")
	}

	fetched, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("failed to get user by ID: %v", err)
	}

	if fetched.PasswordHash != u.PasswordHash {
		t.Errorf("expected password_hash %q, got %q", u.PasswordHash, fetched.PasswordHash)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := testhelper.StartDB()
	t.Cleanup(func() { testhelper.TruncateAll(t, db) })

	store := postgres.NewStore(db)
	ctx := context.Background()

	u := &user.User{
		Email:        "unique@example.com",
		PasswordHash: "pwd",
		CreatedAt:    time.Now(),
	}

	if err := store.User.Create(ctx, u); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	p := &profile.Profile{
		FirstName:  "Test",
		LastName:   "User",
		Email:      "unique@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &u.ID,
	}
	if err := store.Profile.Create(ctx, p); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	fetched, err := store.User.GetByEmail(ctx, "unique@example.com")
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}

	if fetched.ID != u.ID {
		t.Errorf("expected fetched ID %s, got %s", u.ID, fetched.ID)
	}

	_, err = store.User.GetByEmail(ctx, "nonexistent@example.com")
	if err == nil {
		t.Error("expected error when fetching non-existent email, got nil")
	}
}
