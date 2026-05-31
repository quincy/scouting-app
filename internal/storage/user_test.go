package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain"
	"scout-app/internal/storage/mock"
)

func TestUserRepository_CreateAndGetByID(t *testing.T) {
	repo := mock.NewUserRepository()
	ctx := context.Background()

	user := &domain.User{
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if user.ID == "" {
		t.Errorf("expected generated UUID ID to be set on user")
	}

	fetched, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get user by ID: %v", err)
	}

	if fetched.Email != user.Email {
		t.Errorf("expected email %q, got %q", user.Email, fetched.Email)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	repo := mock.NewUserRepository()
	ctx := context.Background()

	user := &domain.User{
		Email:        "unique@example.com",
		PasswordHash: "pwd",
		CreatedAt:    time.Now(),
	}

	_ = repo.Create(ctx, user)

	fetched, err := repo.GetByEmail(ctx, "unique@example.com")
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}

	if fetched.ID != user.ID {
		t.Errorf("expected fetched ID %s, got %s", user.ID, fetched.ID)
	}

	// Test non-existent user
	_, err = repo.GetByEmail(ctx, "nonexistent@example.com")
	if err == nil {
		t.Error("expected error when fetching non-existent email, got nil")
	}
}

func TestUserRepository_DuplicateEmail(t *testing.T) {
	repo := mock.NewUserRepository()
	ctx := context.Background()

	user1 := &domain.User{
		Email:        "duplicate@example.com",
		PasswordHash: "hash1",
		CreatedAt:    time.Now(),
	}

	user2 := &domain.User{
		Email:        "duplicate@example.com",
		PasswordHash: "hash2",
		CreatedAt:    time.Now(),
	}

	err := repo.Create(ctx, user1)
	if err != nil {
		t.Fatalf("failed to create first user: %v", err)
	}

	err = repo.Create(ctx, user2)
	if err == nil {
		t.Fatal("expected error when creating user with duplicate email, got nil")
	}

	if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected domain error for duplicate email, got: %v", err)
	}
}
