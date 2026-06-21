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
	otpTemplate           *template.Template
	adminNotificationTmpl *template.Template
}

type otpData struct {
	Code       string
	UnitType   string
	UnitNumber string
	OTPID      string
	VerifyURL  string
}

type adminNotificationData struct {
	AdminURL string
}

func NewTemplates() (*Templates, error) {
	otp, err := template.ParseFS(templateFS, "templates/otp_verification.txt")
	if err != nil {
		return nil, fmt.Errorf("parse OTP template: %w", err)
	}
	admin, err := template.ParseFS(templateFS, "templates/admin_notification.txt")
	if err != nil {
		return nil, fmt.Errorf("parse admin notification template: %w", err)
	}
	return &Templates{otpTemplate: otp, adminNotificationTmpl: admin}, nil
}

func (t *Templates) RenderOTP(code, unitType, unitNumber, otpID string) (subject, body string, err error) {
	var buf bytes.Buffer
	if err := t.otpTemplate.Execute(&buf, otpData{
		Code:       code,
		UnitType:   unitType,
		UnitNumber: unitNumber,
		OTPID:      otpID,
		VerifyURL:  "http://localhost:8080/register/verify?otp_id=" + otpID,
	}); err != nil {
		return "", "", fmt.Errorf("render OTP email: %w", err)
	}
	full := buf.String()
	subject = extractSubject(full)
	body = stripSubject(full)
	return subject, body, nil
}

func (t *Templates) RenderAdminNotification(adminURL string) (subject, body string, err error) {
	var buf bytes.Buffer
	if err := t.adminNotificationTmpl.Execute(&buf, adminNotificationData{
		AdminURL: adminURL,
	}); err != nil {
		return "", "", fmt.Errorf("render admin notification email: %w", err)
	}
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
