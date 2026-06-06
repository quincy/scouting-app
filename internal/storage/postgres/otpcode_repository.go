package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"scout-app/internal/domain/otpcode"
)

type OTPCodeRepository struct {
	db *sql.DB
}

func NewOTPCodeRepository(db *sql.DB) *OTPCodeRepository {
	return &OTPCodeRepository{db: db}
}

func (r *OTPCodeRepository) Create(ctx context.Context, otp *otpcode.OTPCode) error {
	if otp.ID == "" {
		otp.ID = newUUID()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO otp_codes (id, email, code, expires_at, used, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		otp.ID, otp.Email, otp.Code, otp.ExpiresAt, otp.Used, coalesceTime(otp.CreatedAt),
	)
	return err
}

func (r *OTPCodeRepository) GetByEmailAndCode(ctx context.Context, email string, code string) (*otpcode.OTPCode, error) {
	o := &otpcode.OTPCode{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, code, expires_at, used, created_at
		 FROM otp_codes
		 WHERE email = $1 AND code = $2 AND used = false AND expires_at > $3
		 ORDER BY created_at DESC LIMIT 1`,
		email, code, time.Now(),
	).Scan(&o.ID, &o.Email, &o.Code, &o.ExpiresAt, &o.Used, &o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("otp not found, already used, or expired")
	}
	return o, err
}

func (r *OTPCodeRepository) MarkUsed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE otp_codes SET used = true WHERE id = $1`, id,
	)
	return err
}

var _ otpcode.Repository = (*OTPCodeRepository)(nil)
