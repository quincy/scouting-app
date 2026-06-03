package otpcode

import "context"

type Repository interface {
	Create(ctx context.Context, otp *OTPCode) error
	GetByEmailAndCode(ctx context.Context, email string, code string) (*OTPCode, error)
	MarkUsed(ctx context.Context, id string) error
}
