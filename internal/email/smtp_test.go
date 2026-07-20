package email

import (
	"bytes"
	"context"
	"log"
	"net"
	"net/textproto"
	"strings"
	"testing"

	"scout-app/internal/domain/appconfig"
)

func TestSendAdminNotification_ViaSMTPServer(t *testing.T) {
	tmpl, err := NewTemplates()
	if err != nil {
		t.Fatalf("NewTemplates failed: %v", err)
	}

	received := make(chan string, 1)
	server := startFakeSMTPServer(t, received)

	appCfg := appconfig.NewInMemoryRepository()
	appCfg.Set(context.Background(), appconfig.KeyUnitType, "Troop")
	appCfg.Set(context.Background(), appconfig.KeyUnitNumber, "077")
	appCfg.Set(context.Background(), appconfig.KeySMTPHost, "localhost")
	appCfg.Set(context.Background(), appconfig.KeySMTPPort, server.port)
	appCfg.Set(context.Background(), appconfig.KeySMTPUser, "user")
	appCfg.Set(context.Background(), appconfig.KeySMTPPass, "pass")
	appCfg.Set(context.Background(), appconfig.KeySMTPFrom, "sender@test.com")
	sender := NewSender(appCfg, tmpl)

	err = sender.SendAdminNotification(context.Background(), []string{"admin1@test.com", "admin2@test.com"}, "Test Subject", "Test body content")
	if err != nil {
		t.Fatalf("SendAdminNotification failed: %v", err)
	}

	raw := <-received

	if !strings.Contains(raw, "From: sender@test.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(raw, "To: admin1@test.com, admin2@test.com") {
		t.Error("missing or wrong To header")
	}
	if !strings.Contains(raw, "Subject: Test Subject") {
		t.Error("missing or wrong Subject")
	}
	if !strings.Contains(raw, "Test body content") {
		t.Error("missing body content")
	}
	if !strings.Contains(raw, "Content-Type: text/plain; charset=UTF-8") {
		t.Error("missing Content-Type header")
	}
}

func TestSendOTP_ViaSMTPServer(t *testing.T) {
	tmpl, err := NewTemplates()
	if err != nil {
		t.Fatalf("NewTemplates failed: %v", err)
	}

	received := make(chan string, 1)
	server := startFakeSMTPServer(t, received)

	appCfg := appconfig.NewInMemoryRepository()
	appCfg.Set(context.Background(), appconfig.KeyUnitType, "Troop")
	appCfg.Set(context.Background(), appconfig.KeyUnitNumber, "077")
	appCfg.Set(context.Background(), appconfig.KeySMTPHost, "localhost")
	appCfg.Set(context.Background(), appconfig.KeySMTPPort, server.port)
	appCfg.Set(context.Background(), appconfig.KeySMTPUser, "user")
	appCfg.Set(context.Background(), appconfig.KeySMTPPass, "pass")
	appCfg.Set(context.Background(), appconfig.KeySMTPFrom, "sender@test.com")
	sender := NewSender(appCfg, tmpl)

	err = sender.SendOTP(context.Background(), "recipient@test.com", "654321", "otp-uuid-456")
	if err != nil {
		t.Fatalf("SendOTP failed: %v", err)
	}

	raw := <-received

	if !strings.Contains(raw, "From: sender@test.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(raw, "To: recipient@test.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(raw, "Subject: Your Troop 077 Verification Code") {
		t.Error("missing or wrong Subject")
	}
	if !strings.Contains(raw, "654321") {
		t.Error("missing OTP code in body")
	}
	if !strings.Contains(raw, "Content-Type: text/plain; charset=UTF-8") {
		t.Error("missing Content-Type header")
	}
	if !strings.Contains(raw, "otp-uuid-456") {
		t.Error("missing OTP ID in email body")
	}
}

type fakeSMTPServer struct {
	port string
}

func startFakeSMTPServer(t *testing.T, received chan<- string) *fakeSMTPServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	srv := &fakeSMTPServer{port: portFromAddr(ln.Addr().String())}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		tp := textproto.NewConn(conn)
		tp.PrintfLine("220 localhost ESMTP")

		for {
			line, err := tp.ReadLine()
			if err != nil {
				return
			}

			switch {
			case strings.HasPrefix(line, "EHLO"):
				tp.PrintfLine("250-localhost")
				tp.PrintfLine("250 AUTH LOGIN PLAIN")
			case strings.HasPrefix(line, "HELO"):
				tp.PrintfLine("250 localhost")
			case strings.HasPrefix(line, "AUTH"):
				tp.PrintfLine("235 2.7.0 Authentication successful")
			case strings.HasPrefix(line, "MAIL FROM"):
				tp.PrintfLine("250 2.1.0 Ok")
			case strings.HasPrefix(line, "RCPT TO"):
				tp.PrintfLine("250 2.1.5 Ok")
			case strings.HasPrefix(line, "DATA"):
				tp.PrintfLine("354 End data with <CR><LF>.<CR><LF>")
				msg, err := tp.ReadDotLines()
				if err != nil {
					return
				}
				received <- strings.Join(msg, "\n")
				tp.PrintfLine("250 2.0.0 Ok: queued")
			case strings.HasPrefix(line, "QUIT"):
				tp.PrintfLine("221 2.0.0 Bye")
				return
			}
		}
	}()

	return srv
}

func TestCheckSMTPConfig_Warning(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	repo := appconfig.NewInMemoryRepository()
	CheckSMTPConfig(t.Context(), repo)

	if !strings.Contains(buf.String(), "WARNING") {
		t.Error("expected warning when host=localhost and from is empty")
	}
}

func TestCheckSMTPConfig_NoWarningWhenHostSet(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	repo := appconfig.NewInMemoryRepository()
	repo.Set(t.Context(), appconfig.KeySMTPHost, "smtp.example.com")
	CheckSMTPConfig(t.Context(), repo)

	if buf.Len() > 0 {
		t.Errorf("expected no warning, got: %s", buf.String())
	}
}

func TestCheckSMTPConfig_NoWarningWhenFromSet(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	repo := appconfig.NewInMemoryRepository()
	repo.Set(t.Context(), appconfig.KeySMTPFrom, "from@example.com")
	CheckSMTPConfig(t.Context(), repo)

	if buf.Len() > 0 {
		t.Errorf("expected no warning, got: %s", buf.String())
	}
}

func portFromAddr(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}
