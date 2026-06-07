package otpcode

import (
	"testing"
	"time"
)

func TestGenerateCode_Length(t *testing.T) {
	code, err := GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode failed: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected code length 6, got %d", len(code))
	}
}

func TestGenerateCode_Numeric(t *testing.T) {
	code, err := GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode failed: %v", err)
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("expected digit, got %c", c)
		}
	}
}

func TestNewOTPCode(t *testing.T) {
	plainCode, otp, err := NewOTPCode("test@scout.local")
	if err != nil {
		t.Fatalf("NewOTPCode failed: %v", err)
	}
	if otp.Email != "test@scout.local" {
		t.Errorf("expected email test@scout.local, got %s", otp.Email)
	}
	if len(plainCode) != 6 {
		t.Errorf("expected plain code length 6, got %d", len(plainCode))
	}
	if len(otp.CodeHash) == 0 {
		t.Error("expected CodeHash to be set")
	}
	if otp.Used {
		t.Error("expected new OTP to not be used")
	}
	if otp.Attempts != 0 {
		t.Errorf("expected Attempts to be 0, got %d", otp.Attempts)
	}
	if otp.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if !otp.ExpiresAt.After(time.Now()) {
		t.Error("expected ExpiresAt to be in the future")
	}
}

func TestIsExpired_Past(t *testing.T) {
	otp := &OTPCode{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	if !otp.IsExpired() {
		t.Error("expected OTP with past expiry to be expired")
	}
}

func TestIsExpired_Future(t *testing.T) {
	otp := &OTPCode{ExpiresAt: time.Now().Add(1 * time.Hour)}
	if otp.IsExpired() {
		t.Error("expected OTP with future expiry to not be expired")
	}
}

func TestIsValid_Valid(t *testing.T) {
	otp := &OTPCode{ExpiresAt: time.Now().Add(1 * time.Hour), Used: false}
	if !otp.IsValid() {
		t.Error("expected non-expired, unused OTP to be valid")
	}
}

func TestIsValid_Expired(t *testing.T) {
	otp := &OTPCode{ExpiresAt: time.Now().Add(-1 * time.Hour), Used: false}
	if otp.IsValid() {
		t.Error("expected expired OTP to be invalid")
	}
}

func TestIsValid_Used(t *testing.T) {
	otp := &OTPCode{ExpiresAt: time.Now().Add(1 * time.Hour), Used: true}
	if otp.IsValid() {
		t.Error("expected used OTP to be invalid")
	}
}

func TestGenerateCode_Uniqueness(t *testing.T) {
	codes := make(map[string]bool)
	for range 100 {
		code, err := GenerateCode()
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}
		if codes[code] {
			t.Errorf("duplicate code: %s", code)
		}
		codes[code] = true
	}
}
