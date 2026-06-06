package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	domainemail "scout-app/internal/domain/email"
)

type Sender struct {
	host       string
	port       string
	user       string
	pass       string
	from       string
	unitType   string
	unitNumber string
	templates  *Templates
}

func NewSender(host, port, user, pass, from, unitType, unitNumber string, templates *Templates) *Sender {
	return &Sender{
		host:       host,
		port:       port,
		user:       user,
		pass:       pass,
		from:       from,
		unitType:   unitType,
		unitNumber: unitNumber,
		templates:  templates,
	}
}

func (s *Sender) SendOTP(ctx context.Context, to, code string) error {
	subject, body, err := s.templates.RenderOTP(code, s.unitType, s.unitNumber)
	if err != nil {
		return err
	}

	msg := buildMessage(s.from, to, subject, body)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.pass, s.host)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg))
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
