package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"scout-app/internal/domain/profile"
)

type ProfileRepository struct {
	db *sql.DB
}

func NewProfileRepository(db *sql.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

func (r *ProfileRepository) columns() string {
	return `id, bsa_id, first_name, last_name, nickname, gender, email, phone, birthdate, member_type, status, user_id, positions, created_at, updated_at`
}

func (r *ProfileRepository) scanRow(p *profile.Profile, s interface{ Scan(dest ...any) error }) error {
	return s.Scan(&p.ID, &p.BSAID, &p.FirstName, &p.LastName, &p.Nickname, &p.Gender,
		&p.Email, &p.Phone, &p.Birthdate, &p.MemberType, &p.Status, &p.UserID, &p.Positions,
		&p.CreatedAt, &p.UpdatedAt)
}

func (r *ProfileRepository) Create(ctx context.Context, p *profile.Profile) error {
	if p.ID == "" {
		p.ID = newUUID()
	}
	now := coalesceTime(p.CreatedAt)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO profiles (id, bsa_id, first_name, last_name, nickname, gender, email, phone, birthdate, member_type, status, user_id, positions, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)`,
		p.ID, p.BSAID, p.FirstName, p.LastName, p.Nickname, p.Gender,
		p.Email, p.Phone, p.Birthdate, p.MemberType, p.Status, p.UserID, p.Positions, now,
	)
	return err
}

func (r *ProfileRepository) GetByID(ctx context.Context, id string) (*profile.Profile, error) {
	p := &profile.Profile{}
	err := r.scanRow(p, r.db.QueryRowContext(ctx,
		`SELECT `+r.columns()+` FROM profiles WHERE id = $1`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("profile not found")
	}
	return p, err
}

func (r *ProfileRepository) GetByEmail(ctx context.Context, email string) (*profile.Profile, error) {
	p := &profile.Profile{}
	err := r.scanRow(p, r.db.QueryRowContext(ctx,
		`SELECT `+r.columns()+` FROM profiles WHERE email = $1`, email,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("profile not found")
	}
	return p, err
}

func (r *ProfileRepository) GetByBSAID(ctx context.Context, bsaID string) (*profile.Profile, error) {
	if bsaID == "" {
		return nil, errors.New("profile not found")
	}
	p := &profile.Profile{}
	err := r.scanRow(p, r.db.QueryRowContext(ctx,
		`SELECT `+r.columns()+` FROM profiles WHERE bsa_id = $1`, bsaID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("profile not found")
	}
	return p, err
}

func (r *ProfileRepository) GetByUserID(ctx context.Context, userID string) (*profile.Profile, error) {
	p := &profile.Profile{}
	err := r.scanRow(p, r.db.QueryRowContext(ctx,
		`SELECT `+r.columns()+` FROM profiles WHERE user_id = $1`, userID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("profile not found for user")
	}
	return p, err
}

func (r *ProfileRepository) ListAll(ctx context.Context) ([]*profile.Profile, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+r.columns()+` FROM profiles`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*profile.Profile
	for rows.Next() {
		p := &profile.Profile{}
		if err := r.scanRow(p, rows); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *ProfileRepository) ListByStatus(ctx context.Context, status profile.Status) ([]*profile.Profile, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+r.columns()+` FROM profiles WHERE status = $1`, status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*profile.Profile
	for rows.Next() {
		p := &profile.Profile{}
		if err := r.scanRow(p, rows); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *ProfileRepository) Update(ctx context.Context, p *profile.Profile) error {
	now := coalesceTime(time.Now())
	_, err := r.db.ExecContext(ctx,
		`UPDATE profiles SET bsa_id = $1, first_name = $2, last_name = $3, nickname = $4, gender = $5,
		 email = $6, phone = $7, birthdate = $8, member_type = $9, status = $10, user_id = $11, positions = $12, updated_at = $13
		 WHERE id = $14`,
		p.BSAID, p.FirstName, p.LastName, p.Nickname, p.Gender,
		p.Email, p.Phone, p.Birthdate, p.MemberType, p.Status, p.UserID, p.Positions, now, p.ID,
	)
	return err
}

var _ profile.Repository = (*ProfileRepository)(nil)
