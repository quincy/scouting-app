package email

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"scout-app/internal/domain/appconfig"
	domainemail "scout-app/internal/domain/email"
)

type Sender struct {
	appConfigRepo appconfig.Repository
	templates     *Templates
}

func NewSender(appConfigRepo appconfig.Repository, templates *Templates) *Sender {
	return &Sender{
		appConfigRepo: appConfigRepo,
		templates:     templates,
	}
}

func (s *Sender) loadSMTPConfig(ctx context.Context) (host, port, user, pass, from string) {
	host = appconfig.GetWithHierarchy(ctx, s.appConfigRepo, "SMTP_HOST", appconfig.KeySMTPHost, "localhost")
	port = appconfig.GetWithHierarchy(ctx, s.appConfigRepo, "SMTP_PORT", appconfig.KeySMTPPort, "1025")
	user = appconfig.GetWithHierarchy(ctx, s.appConfigRepo, "SMTP_USER", appconfig.KeySMTPUser, "")
	pass = appconfig.GetWithHierarchy(ctx, s.appConfigRepo, "SMTP_PASS", appconfig.KeySMTPPass, "")
	from = appconfig.GetWithHierarchy(ctx, s.appConfigRepo, "SMTP_FROM", appconfig.KeySMTPFrom, "")
	return
}

func (s *Sender) loadUnitInfo(ctx context.Context) (unitType, unitNumber string) {
	unitType = appconfig.GetWithHierarchy(ctx, s.appConfigRepo, "UNIT_TYPE", appconfig.KeyUnitType, "Troop")
	unitNumber = appconfig.GetWithHierarchy(ctx, s.appConfigRepo, "UNIT_NUMBER", appconfig.KeyUnitNumber, "")
	return
}

func (s *Sender) SendOTP(ctx context.Context, to, code string, otpID string) error {
	unitType, unitNumber := s.loadUnitInfo(ctx)
	subject, body, err := s.templates.RenderOTP(code, unitType, unitNumber, otpID)
	if err != nil {
		return err
	}
	return s.send(ctx, subject, body, []string{to})
}

func (s *Sender) SendAdminNotification(ctx context.Context, to []string, subject, body string) error {
	return s.send(ctx, subject, body, to)
}

func (s *Sender) send(ctx context.Context, subject, body string, to []string) error {
	host, port, user, pass, from := s.loadSMTPConfig(ctx)

	msg := buildMessage(from, strings.Join(to, ", "), subject, body)
	addr := fmt.Sprintf("%s:%s", host, port)
	auth := smtp.PlainAuth("", user, pass, host)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return smtp.SendMail(addr, auth, from, to, []byte(msg))
}

func buildMessage(from, to, subject, body string) string {
	var b strings.Builder
	b.WriteString("From: ")
	b.WriteString(from)
	b.WriteString("\n")
	b.WriteString("To: ")
	b.WriteString(to)
	b.WriteString("\n")
	b.WriteString("Subject: ")
	b.WriteString(subject)
	b.WriteString("\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8")
	b.WriteString("\n\n")
	b.WriteString(body)
	return b.String()
}

func CheckSMTPConfig(ctx context.Context, repo appconfig.Repository) {
	host := appconfig.GetWithHierarchy(ctx, repo, "SMTP_HOST", appconfig.KeySMTPHost, "localhost")
	from := appconfig.GetWithHierarchy(ctx, repo, "SMTP_FROM", appconfig.KeySMTPFrom, "")
	if host == "localhost" && from == "" {
		log.Println("WARNING: SMTP_HOST is not configured (no env var and no DB value). Email sending will use default localhost:1025.")
	}
}

var _ domainemail.Service = (*Sender)(nil)
