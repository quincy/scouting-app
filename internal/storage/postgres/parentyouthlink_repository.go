package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"scout-app/internal/domain/parentyouthlink"
)

type ParentYouthLinkRepository struct {
	db *sql.DB
}

func NewParentYouthLinkRepository(db *sql.DB) *ParentYouthLinkRepository {
	return &ParentYouthLinkRepository{db: db}
}

func (r *ParentYouthLinkRepository) Create(ctx context.Context, link *parentyouthlink.ParentYouthConnection) error {
	if link.ID == "" {
		link.ID = newUUID()
	}
	now := coalesceTime(link.CreatedAt)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO parent_youth_links (id, parent_profile_id, youth_profile_id, status, requested_at, approved_at, approved_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		link.ID, link.ParentProfileID, link.YouthProfileID, link.Status, coalesceTime(link.RequestedAt), link.ApprovedAt, link.ApprovedBy, now,
	)
	return err
}

func (r *ParentYouthLinkRepository) GetByID(ctx context.Context, id string) (*parentyouthlink.ParentYouthConnection, error) {
	l := &parentyouthlink.ParentYouthConnection{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, parent_profile_id, youth_profile_id, status, requested_at, approved_at, approved_by, created_at
		 FROM parent_youth_links WHERE id = $1`, id,
	).Scan(&l.ID, &l.ParentProfileID, &l.YouthProfileID, &l.Status, &l.RequestedAt, &l.ApprovedAt, &l.ApprovedBy, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("link not found")
	}
	return l, err
}

func (r *ParentYouthLinkRepository) ListByParent(ctx context.Context, parentProfileID string) ([]*parentyouthlink.ParentYouthConnection, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, parent_profile_id, youth_profile_id, status, requested_at, approved_at, approved_by, created_at
		 FROM parent_youth_links WHERE parent_profile_id = $1`, parentProfileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*parentyouthlink.ParentYouthConnection
	for rows.Next() {
		l := &parentyouthlink.ParentYouthConnection{}
		if err := rows.Scan(&l.ID, &l.ParentProfileID, &l.YouthProfileID, &l.Status, &l.RequestedAt, &l.ApprovedAt, &l.ApprovedBy, &l.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *ParentYouthLinkRepository) ListByStatus(ctx context.Context, status parentyouthlink.Status) ([]*parentyouthlink.ParentYouthConnection, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, parent_profile_id, youth_profile_id, status, requested_at, approved_at, approved_by, created_at
		 FROM parent_youth_links WHERE status = $1`, status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*parentyouthlink.ParentYouthConnection
	for rows.Next() {
		l := &parentyouthlink.ParentYouthConnection{}
		if err := rows.Scan(&l.ID, &l.ParentProfileID, &l.YouthProfileID, &l.Status, &l.RequestedAt, &l.ApprovedAt, &l.ApprovedBy, &l.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *ParentYouthLinkRepository) UpdateStatus(ctx context.Context, id string, status parentyouthlink.Status, approvedBy string) error {
	now := coalesceTime(time.Now())
	_, err := r.db.ExecContext(ctx,
		`UPDATE parent_youth_links SET status = $1, approved_by = $2, approved_at = $3 WHERE id = $4`,
		status, nullString(approvedBy), now, id,
	)
	return err
}

var _ parentyouthlink.Repository = (*ParentYouthLinkRepository)(nil)
