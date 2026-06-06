package postgres

import (
	"context"
	"testing"
	"time"

	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
)

func TestPostgresUserRepository_CreateAndGetByID(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	u := &user.User{
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

func TestPostgresUserRepository_GetByEmail(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	userRepo := NewUserRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	u := &user.User{
		PasswordHash: "pwdhash",
		CreatedAt:    time.Now(),
	}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	prof := &profile.Profile{
		FirstName:  "Test",
		LastName:   "User",
		Email:      "test@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &u.ID,
	}
	if err := profileRepo.Create(ctx, prof); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	fetched, err := userRepo.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}

	if fetched.ID != u.ID {
		t.Errorf("expected user ID %s, got %s", u.ID, fetched.ID)
	}

	_, err = userRepo.GetByEmail(ctx, "nonexistent@example.com")
	if err == nil {
		t.Error("expected error when fetching non-existent email, got nil")
	}
}

func TestPostgresUserRepository_GetByID_NotFound(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for non-existent user, got nil")
	}
}
