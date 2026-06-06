package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults_ErrorWithoutSessionSecret(t *testing.T) {
	unsetAll(t)
	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error when SESSION_SECRET is not set, got nil")
	}
}

func TestDefaults_DefaultValues(t *testing.T) {
	t.Setenv("SESSION_SECRET", "test-secret")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv failed: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("expected Addr :8080, got %q", cfg.Addr)
	}
	if cfg.SMTPHost != "localhost" {
		t.Errorf("expected SMTPHost localhost, got %q", cfg.SMTPHost)
	}
	if cfg.SMTPPort != "1025" {
		t.Errorf("expected SMTPPort 1025, got %q", cfg.SMTPPort)
	}
	if cfg.ScoutbookAPIBaseURL != "https://api.scouting.org" {
		t.Errorf("expected ScoutbookAPIBaseURL https://api.scouting.org, got %q", cfg.ScoutbookAPIBaseURL)
	}
}

func TestEnvTakesPrecedenceOverDotenv(t *testing.T) {
	t.Setenv("ADDR", ":9090")
	t.Setenv("SESSION_SECRET", "os-secret")

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("ADDR=:3000\nSESSION_SECRET=file-secret\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := loadFile(envPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv failed: %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("expected Addr :9090 (OS env takes precedence), got %q", cfg.Addr)
	}
	if cfg.SessionSecret != "os-secret" {
		t.Errorf("expected SessionSecret os-secret (OS env takes precedence), got %q", cfg.SessionSecret)
	}
}

func TestLoadFile_ParsesCorrectly(t *testing.T) {
	unsetAll(t)

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := `
# This is a comment
ADDR=:5000
USE_MOCK_STORAGE=true

DATABASE_URL=postgres://localhost/test
SESSION_SECRET=from-file-secret
SCOUTBOOK_API_BASE_URL=https://custom.api
SMTP_HOST=smtp.example.com
SMTP_PORT=587
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := loadFile(envPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv failed: %v", err)
	}
	if cfg.Addr != ":5000" {
		t.Errorf("expected Addr :5000, got %q", cfg.Addr)
	}
	if !cfg.UseMockStorage {
		t.Error("expected UseMockStorage true")
	}
	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("expected DatabaseURL postgres://localhost/test, got %q", cfg.DatabaseURL)
	}
	if cfg.SessionSecret != "from-file-secret" {
		t.Errorf("expected SessionSecret from-file-secret, got %q", cfg.SessionSecret)
	}
	if cfg.ScoutbookAPIBaseURL != "https://custom.api" {
		t.Errorf("expected ScoutbookAPIBaseURL https://custom.api, got %q", cfg.ScoutbookAPIBaseURL)
	}
	if cfg.SMTPHost != "smtp.example.com" {
		t.Errorf("expected SMTPHost smtp.example.com, got %q", cfg.SMTPHost)
	}
	if cfg.SMTPPort != "587" {
		t.Errorf("expected SMTPPort 587, got %q", cfg.SMTPPort)
	}
}

func TestCommentsAndBlankLinesIgnored(t *testing.T) {
	unsetAll(t)

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "\n\n\n# first\n\n# second\nSESSION_SECRET=present\n\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := loadFile(envPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv failed: %v", err)
	}
	if cfg.SessionSecret != "present" {
		t.Errorf("expected SessionSecret present, got %q", cfg.SessionSecret)
	}
}

func TestSessionSecretFromEnv(t *testing.T) {
	t.Setenv("SESSION_SECRET", "env-secret")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv failed: %v", err)
	}
	if cfg.SessionSecret != "env-secret" {
		t.Errorf("expected SessionSecret env-secret, got %q", cfg.SessionSecret)
	}
}

func TestSessionSecretFromDotenv(t *testing.T) {
	unsetAll(t)

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("SESSION_SECRET=dotenv-secret\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := loadFile(envPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv failed: %v", err)
	}
	if cfg.SessionSecret != "dotenv-secret" {
		t.Errorf("expected SessionSecret dotenv-secret, got %q", cfg.SessionSecret)
	}
}

func TestScoutbookAPIBaseURLDefault(t *testing.T) {
	unsetAll(t)
	t.Setenv("SESSION_SECRET", "test-secret")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv failed: %v", err)
	}
	if cfg.ScoutbookAPIBaseURL != "https://api.scouting.org" {
		t.Errorf("expected default ScoutbookAPIBaseURL https://api.scouting.org, got %q", cfg.ScoutbookAPIBaseURL)
	}
}

func TestLoadFile_FileNotFound(t *testing.T) {
	err := loadFile("/nonexistent/path/.env")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func unsetAll(t *testing.T) {
	t.Helper()
	keys := []string{
		"ADDR", "USE_MOCK_STORAGE", "DATABASE_URL", "AUTO_MIGRATE",
		"SESSION_SECRET", "SCOUTBOOK_API_BASE_URL", "SCOUTBOOK_ORG_GUID",
		"SCOUTBOOK_TOKEN", "SMTP_HOST", "SMTP_PORT", "SMTP_USER",
		"SMTP_PASS", "SMTP_FROM",
	}
	for _, k := range keys {
		old := os.Getenv(k)
		os.Unsetenv(k)
		if old != "" {
			t.Cleanup(func() { os.Setenv(k, old) })
		}
	}
}
