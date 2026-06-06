package postgres

import (
	"context"
	"database/sql"
	"errors"

	"scout-app/internal/domain/user"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	if u.ID == "" {
		u.ID = newUUID()
	}
	now := coalesceTime(u.CreatedAt)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, password_hash, created_at, updated_at) VALUES ($1, $2, $3, $3)`,
		u.ID, u.PasswordHash, now,
	)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	u := &user.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, password_hash, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("user not found")
	}
	return u, err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	u := &user.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT u.id, u.password_hash, u.created_at
		 FROM users u
		 JOIN profiles p ON p.user_id = u.id
		 WHERE p.email = $1`, email,
	).Scan(&u.ID, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("user not found")
	}
	return u, err
}

var _ user.Repository = (*UserRepository)(nil)
