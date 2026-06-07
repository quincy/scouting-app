package postgres

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"scout-app/internal/domain/otpcode"
)

func TestPostgresOTPCodeRepository_CreateAndGetByID(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewOTPCodeRepository(testDB)
	ctx := context.Background()

	hash := sha256.Sum256([]byte("123456"))
	code := &otpcode.OTPCode{
		Email:     "test@example.com",
		CodeHash:  hash[:],
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	if err := repo.Create(ctx, code); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if code.ID == "" {
		t.Error("expected generated ID")
	}

	fetched, err := repo.GetByID(ctx, code.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if string(fetched.CodeHash) != string(hash[:]) {
		t.Error("expected matching code hash")
	}
}

func TestPostgresOTPCodeRepository_MarkUsedIfUnused(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewOTPCodeRepository(testDB)
	ctx := context.Background()

	hash := sha256.Sum256([]byte("111111"))
	code := &otpcode.OTPCode{
		Email:     "used@test.com",
		CodeHash:  hash[:],
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	repo.Create(ctx, code)

	ok, err := repo.MarkUsedIfUnused(ctx, code.ID)
	if err != nil {
		t.Fatalf("MarkUsedIfUnused failed: %v", err)
	}
	if !ok {
		t.Error("expected MarkUsedIfUnused to return true")
	}

	// Second call should return false (already used)
	ok, err = repo.MarkUsedIfUnused(ctx, code.ID)
	if err != nil {
		t.Fatalf("MarkUsedIfUnused failed: %v", err)
	}
	if ok {
		t.Error("expected MarkUsedIfUnused to return false for already used code")
	}
}
