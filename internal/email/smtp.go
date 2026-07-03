package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"scout-app/internal/domain/appconfig"
	domainemail "scout-app/internal/domain/email"
)

type Sender struct {
	host          string
	port          string
	user          string
	pass          string
	from          string
	appConfigRepo appconfig.Repository
	templates     *Templates
}

func NewSender(host, port, user, pass, from string, appConfigRepo appconfig.Repository, templates *Templates) *Sender {
	return &Sender{
		host:          host,
		port:          port,
		user:          user,
		pass:          pass,
		from:          from,
		appConfigRepo: appConfigRepo,
		templates:     templates,
	}
}

func (s *Sender) loadUnitInfo(ctx context.Context) (unitType, unitNumber string) {
	unitType, _ = s.appConfigRepo.Get(ctx, appconfig.KeyUnitType)
	if unitType == "" {
		unitType = "Troop"
	}
	unitNumber, _ = s.appConfigRepo.Get(ctx, appconfig.KeyUnitNumber)
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
	msg := buildMessage(s.from, strings.Join(to, ", "), subject, body)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.pass, s.host)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return smtp.SendMail(addr, auth, s.from, to, []byte(msg))
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

var _ domainemail.Service = (*Sender)(nil)
