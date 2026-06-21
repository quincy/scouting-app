package email

import (
	"context"
	"net"
	"net/textproto"
	"strings"
	"testing"
)

func TestSendAdminNotification_ViaSMTPServer(t *testing.T) {
	tmpl, err := NewTemplates()
	if err != nil {
		t.Fatalf("NewTemplates failed: %v", err)
	}

	received := make(chan string, 1)
	server := startFakeSMTPServer(t, received)

	sender := NewSender("localhost", server.port, "user", "pass", "sender@test.com", "Troop", "077", tmpl)

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

	sender := NewSender("localhost", server.port, "user", "pass", "sender@test.com", "Troop", "077", tmpl)

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

func portFromAddr(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}
