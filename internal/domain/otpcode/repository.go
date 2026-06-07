package otpcode

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, otp *OTPCode) error
	GetByID(ctx context.Context, id string) (*OTPCode, error)
	CountByEmailSince(ctx context.Context, email string, since time.Time) (int, error)
	MarkUsedIfUnused(ctx context.Context, id string) (bool, error)
	IncrementAttempts(ctx context.Context, id string) (int, error)
	InvalidateByEmail(ctx context.Context, email string) error
	DeleteExpired(ctx context.Context) error
}
