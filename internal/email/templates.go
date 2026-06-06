package email

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.txt
var templateFS embed.FS

type Templates struct {
	otpTemplate *template.Template
}

type otpData struct {
	Code       string
	UnitType   string
	UnitNumber string
}

func NewTemplates() (*Templates, error) {
	otp, err := template.ParseFS(templateFS, "templates/otp_verification.txt")
	if err != nil {
		return nil, fmt.Errorf("parse OTP template: %w", err)
	}
	return &Templates{otpTemplate: otp}, nil
}

func (t *Templates) RenderOTP(code, unitType, unitNumber string) (subject, body string, err error) {
	var buf bytes.Buffer
	if err := t.otpTemplate.Execute(&buf, otpData{Code: code, UnitType: unitType, UnitNumber: unitNumber}); err != nil {
		return "", "", fmt.Errorf("render OTP email: %w", err)
	}
	// First line is Subject: header, rest is body
	full := buf.String()
	subject = extractSubject(full)
	body = stripSubject(full)
	return subject, body, nil
}

func extractSubject(full string) string {
	for _, line := range bytes.Split([]byte(full), []byte("\n")) {
		s := string(line)
		if rest, ok := strings.CutPrefix(s, "Subject:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func stripSubject(full string) string {
	// Remove the Subject: line
	idx := bytes.Index([]byte(full), []byte("\n"))
	if idx == -1 {
		return full
	}
	return full[idx+1:]
}
