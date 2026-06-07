package otpcode

import (
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"time"
)

const CodeLength = 6
const CodeExpiry = 10 * time.Minute
const MaxAttempts = 5

type OTPCode struct {
	ID        string
	Email     string
	CodeHash  []byte
	ExpiresAt time.Time
	Used      bool
	Attempts  int
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

func NewOTPCode(email string) (plainCode string, otp *OTPCode, err error) {
	code, err := GenerateCode()
	if err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256([]byte(code))
	return code, &OTPCode{
		Email:     email,
		CodeHash:  hash[:],
		ExpiresAt: time.Now().Add(CodeExpiry),
		Attempts:  0,
		CreatedAt: time.Now(),
	}, nil
}

func (o *OTPCode) IsExpired() bool {
	return time.Now().After(o.ExpiresAt)
}

func (o *OTPCode) IsValid() bool {
	return !o.IsExpired() && !o.Used && o.Attempts < MaxAttempts
}
