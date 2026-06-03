package otpcode

import (
	"crypto/rand"
	"math/big"
	"time"
)

const CodeLength = 6
const CodeExpiry = 15 * time.Minute

type OTPCode struct {
	ID        string
	Email     string
	Code      string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

func GenerateCode() (string, error) {
	var code [CodeLength]byte
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code[i] = byte('0' + n.Int64())
	}
	return string(code[:]), nil
}

func NewOTPCode(email string) (*OTPCode, error) {
	code, err := GenerateCode()
	if err != nil {
		return nil, err
	}
	return &OTPCode{
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().Add(CodeExpiry),
		CreatedAt: time.Now(),
	}, nil
}

func (o *OTPCode) IsExpired() bool {
	return time.Now().After(o.ExpiresAt)
}

func (o *OTPCode) IsValid() bool {
	return !o.IsExpired() && !o.Used
}
