package postgres

import (
	"context"
	"testing"
	"time"

	"scout-app/internal/domain/otpcode"
)

func TestPostgresOTPCodeRepository_CreateAndGet(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewOTPCodeRepository(testDB)
	ctx := context.Background()

	code := &otpcode.OTPCode{
		Email:     "test@example.com",
		Code:      "123456",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	if err := repo.Create(ctx, code); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if code.ID == "" {
		t.Error("expected generated ID")
	}

	fetched, err := repo.GetByEmailAndCode(ctx, "test@example.com", "123456")
	if err != nil {
		t.Fatalf("GetByEmailAndCode failed: %v", err)
	}
	if fetched.Code != "123456" {
		t.Errorf("expected code %q, got %q", "123456", fetched.Code)
	}
}

func TestPostgresOTPCodeRepository_Expired(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewOTPCodeRepository(testDB)
	ctx := context.Background()

	code := &otpcode.OTPCode{
		Email:     "expired@test.com",
		Code:      "654321",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	repo.Create(ctx, code)

	_, err := repo.GetByEmailAndCode(ctx, "expired@test.com", "654321")
	if err == nil {
		t.Error("expected error for expired code, got nil")
	}
}

func TestPostgresOTPCodeRepository_MarkUsed(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewOTPCodeRepository(testDB)
	ctx := context.Background()

	code := &otpcode.OTPCode{
		Email:     "used@test.com",
		Code:      "111111",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	repo.Create(ctx, code)

	if err := repo.MarkUsed(ctx, code.ID); err != nil {
		t.Fatalf("MarkUsed failed: %v", err)
	}

	_, err := repo.GetByEmailAndCode(ctx, "used@test.com", "111111")
	if err == nil {
		t.Error("expected error for used code, got nil")
	}
}
