package email

import (
	"strings"
	"testing"
)

func TestNewTemplates(t *testing.T) {
	tmpl, err := NewTemplates()
	if err != nil {
		t.Fatalf("NewTemplates failed: %v", err)
	}
	if tmpl == nil {
		t.Fatal("expected non-nil Templates")
	}
}

func TestRenderOTP(t *testing.T) {
	tmpl, err := NewTemplates()
	if err != nil {
		t.Fatalf("NewTemplates failed: %v", err)
	}

	subject, body, err := tmpl.RenderOTP("123456", "Troop", "077", "otp-uuid-123")
	if err != nil {
		t.Fatalf("RenderOTP failed: %v", err)
	}

	if !strings.Contains(subject, "Troop 077 Verification Code") {
		t.Errorf("expected subject to contain 'Troop 077 Verification Code', got %q", subject)
	}

	if !strings.Contains(body, "123456") {
		t.Errorf("expected body to contain the code, got: %s", body)
	}

	if !strings.Contains(body, "Troop 077") {
		t.Errorf("expected body to contain unit info, got: %s", body)
	}

	if !strings.Contains(body, "30 minutes") {
		t.Errorf("expected body to mention 30 minute expiry, got: %s", body)
	}

	if !strings.Contains(body, "/register/verify?otp_id=otp-uuid-123") {
		t.Errorf("expected body to contain verify link, got: %s", body)
	}
}

func TestBuildMessage(t *testing.T) {
	msg := buildMessage("from@test.com", "to@test.com", "Subject Line", "Hello, this is the body.")

	if !strings.Contains(msg, "From: from@test.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(msg, "To: to@test.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(msg, "Subject: Subject Line") {
		t.Error("missing Subject header")
	}
	if !strings.Contains(msg, "Content-Type: text/plain; charset=UTF-8") {
		t.Error("missing Content-Type header")
	}
	if !strings.Contains(msg, "\n\nHello, this is the body.") {
		t.Error("body not found after headers")
	}
}
