package email

import "context"

type Service interface {
	SendOTP(ctx context.Context, to, code string, otpID string) error
	SendAdminNotification(ctx context.Context, to []string, subject, body string) error
}
