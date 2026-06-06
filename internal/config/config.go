package config

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Addr                string
	UseMockStorage      bool
	DatabaseURL         string
	AutoMigrate         bool
	SessionSecret       string
	ScoutbookAPIBaseURL string
	ScoutbookOrgGUID    string
	ScoutbookToken      string
	SMTPHost            string
	SMTPPort            string
	SMTPUser            string
	SMTPPass            string
	SMTPFrom            string
}

func Load() (*Config, error) {
	envFile := flag.String("env", "", "path to .env file")
	flag.Parse()
	if *envFile != "" {
		if err := loadFile(*envFile); err != nil {
			return nil, fmt.Errorf("loading env file: %w", err)
		}
	}
	return ConfigFromEnv()
}

func ConfigFromEnv() (*Config, error) {
	cfg := &Config{
		Addr:                getEnv("ADDR", ":8080"),
		UseMockStorage:      getEnv("USE_MOCK_STORAGE", "") == "true",
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		AutoMigrate:         getEnv("AUTO_MIGRATE", "") == "true",
		SessionSecret:       getEnv("SESSION_SECRET", ""),
		ScoutbookAPIBaseURL: getEnv("SCOUTBOOK_API_BASE_URL", "https://api.scouting.org"),
		ScoutbookOrgGUID:    getEnv("SCOUTBOOK_ORG_GUID", ""),
		ScoutbookToken:      getEnv("SCOUTBOOK_TOKEN", ""),
		SMTPHost:            getEnv("SMTP_HOST", "localhost"),
		SMTPPort:            getEnv("SMTP_PORT", "1025"),
		SMTPUser:            getEnv("SMTP_USER", ""),
		SMTPPass:            getEnv("SMTP_PASS", ""),
		SMTPFrom:            getEnv("SMTP_FROM", ""),
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}
	return cfg, nil
}

func loadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
