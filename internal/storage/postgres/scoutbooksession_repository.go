package postgres

import (
	"context"
	"database/sql"
	"errors"

	"scout-app/internal/domain/scoutbooksession"
)

type ScoutbookSessionRepository struct {
	db *sql.DB
}

func NewScoutbookSessionRepository(db *sql.DB) *ScoutbookSessionRepository {
	return &ScoutbookSessionRepository{db: db}
}

func (r *ScoutbookSessionRepository) Create(ctx context.Context, s *scoutbooksession.Session) error {
	if s.ID == "" {
		s.ID = newUUID()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO scoutbook_sessions (id, token, person_guid, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		s.ID, s.Token, s.PersonGUID, s.ExpiresAt, coalesceTime(s.CreatedAt),
	)
	return err
}

func (r *ScoutbookSessionRepository) GetLatest(ctx context.Context) (*scoutbooksession.Session, error) {
	s := &scoutbooksession.Session{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, token, person_guid, expires_at, created_at
		 FROM scoutbook_sessions
		 ORDER BY created_at DESC LIMIT 1`,
	).Scan(&s.ID, &s.Token, &s.PersonGUID, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("no sessions found")
	}
	return s, err
}

var _ scoutbooksession.Repository = (*ScoutbookSessionRepository)(nil)
