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
		`INSERT INTO otp_codes (id, email, code_hash, expires_at, used, attempts, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		otp.ID, otp.Email, otp.CodeHash, otp.ExpiresAt, otp.Used, otp.Attempts, coalesceTime(otp.CreatedAt),
	)
	return err
}

func (r *OTPCodeRepository) GetByID(ctx context.Context, id string) (*otpcode.OTPCode, error) {
	o := &otpcode.OTPCode{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, code_hash, expires_at, used, attempts, created_at
		 FROM otp_codes WHERE id = $1`, id,
	).Scan(&o.ID, &o.Email, &o.CodeHash, &o.ExpiresAt, &o.Used, &o.Attempts, &o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("otp not found")
	}
	return o, err
}

func (r *OTPCodeRepository) CountByEmailSince(ctx context.Context, email string, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM otp_codes WHERE email = $1 AND created_at > $2`,
		email, since,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *OTPCodeRepository) MarkUsedIfUnused(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE otp_codes SET used = true WHERE id = $1 AND used = false`, id,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *OTPCodeRepository) IncrementAttempts(ctx context.Context, id string) (int, error) {
	var attempts int
	err := r.db.QueryRowContext(ctx,
		`UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1 RETURNING attempts`, id,
	).Scan(&attempts)
	if err != nil {
		return 0, err
	}
	return attempts, nil
}

func (r *OTPCodeRepository) InvalidateByEmail(ctx context.Context, email string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE otp_codes SET used = true WHERE email = $1 AND used = false AND expires_at > $2`,
		email, time.Now(),
	)
	return err
}

func (r *OTPCodeRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM otp_codes WHERE expires_at < $1`, time.Now(),
	)
	return err
}

var _ otpcode.Repository = (*OTPCodeRepository)(nil)
