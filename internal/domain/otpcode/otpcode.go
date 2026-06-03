package otpcode

import "time"

type OTPCode struct {
	ID        string
	Email     string
	Code      string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}
