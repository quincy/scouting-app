package postgres

import (
	"context"
	"database/sql"

	"scout-app/internal/domain/appconfig"
)

type AppConfigRepository struct {
	db *sql.DB
}

func NewAppConfigRepository(db *sql.DB) *AppConfigRepository {
	return &AppConfigRepository{db: db}
}

func (r *AppConfigRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx,
		`SELECT value FROM app_config WHERE key = $1`, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (r *AppConfigRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_config (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = $2`,
		key, value,
	)
	return err
}

func (r *AppConfigRepository) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM app_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, rows.Err()
}

var _ appconfig.Repository = (*AppConfigRepository)(nil)
