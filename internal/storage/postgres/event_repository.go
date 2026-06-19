package postgres

import (
	"context"
	"database/sql"
	"errors"

	"scout-app/internal/domain/event"
	"scout-app/internal/domain/profile"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Create(ctx context.Context, e *event.Event) error {
	if e.ID == "" {
		e.ID = newUUID()
	}
	now := coalesceTime(e.CreatedAt)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO events (id, title, description, location, start_time, end_time, cost_cents, type, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
		e.ID, e.Title, e.Description, e.Location, e.StartTime, e.EndTime, e.CostCents, e.Type, now,
	)
	return err
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (*event.Event, error) {
	e := &event.Event{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, description, location, start_time, end_time, cost_cents, cost_decimal, type, created_at, updated_at
		 FROM events WHERE id = $1`, id,
	).Scan(&e.ID, &e.Title, &e.Description, &e.Location, &e.StartTime, &e.EndTime, &e.CostCents, &e.CostDecimal, &e.Type, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("event not found")
	}
	return e, err
}

func (r *EventRepository) Update(ctx context.Context, e *event.Event) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE events
		 SET title = $2, description = $3, location = $4, start_time = $5, end_time = $6, cost_cents = $7, type = $8, updated_at = NOW()
		 WHERE id = $1`,
		e.ID, e.Title, e.Description, e.Location, e.StartTime, e.EndTime, e.CostCents, e.Type,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("event not found")
	}
	return nil
}

func (r *EventRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM events WHERE id = $1`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("event not found")
	}
	return nil
}

func (r *EventRepository) ListUpcoming(ctx context.Context, limit int, offset int) ([]*event.ListItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.title, e.location, e.start_time, e.end_time, e.type, COUNT(ea.profile_id) AS attendee_count
		 FROM events e
		 LEFT JOIN event_attendees ea ON ea.event_id = e.id AND ea.status = 'signed_up'
		 WHERE e.end_time > NOW()
		 GROUP BY e.id
		 ORDER BY e.start_time ASC
		 LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*event.ListItem
	for rows.Next() {
		li := &event.ListItem{}
		if err := rows.Scan(&li.ID, &li.Title, &li.Location, &li.StartTime, &li.EndTime, &li.Type, &li.AttendeeCount); err != nil {
			return nil, err
		}
		items = append(items, li)
	}
	return items, rows.Err()
}

func (r *EventRepository) ListPast(ctx context.Context, limit int, offset int) ([]*event.ListItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.title, e.location, e.start_time, e.end_time, e.type, COUNT(ea.profile_id) AS attendee_count
		 FROM events e
		 LEFT JOIN event_attendees ea ON ea.event_id = e.id AND ea.status = 'signed_up'
		 WHERE e.end_time <= NOW()
		 GROUP BY e.id
		 ORDER BY e.start_time DESC
		 LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*event.ListItem
	for rows.Next() {
		li := &event.ListItem{}
		if err := rows.Scan(&li.ID, &li.Title, &li.Location, &li.StartTime, &li.EndTime, &li.Type, &li.AttendeeCount); err != nil {
			return nil, err
		}
		items = append(items, li)
	}
	return items, rows.Err()
}

func (r *EventRepository) SignUp(ctx context.Context, eventID string, profileID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO event_attendees (event_id, profile_id, status, created_at, updated_at)
		 VALUES ($1, $2, 'signed_up', NOW(), NOW())
		 ON CONFLICT (event_id, profile_id) DO UPDATE SET status = 'signed_up', updated_at = NOW()`,
		eventID, profileID,
	)
	return err
}

func (r *EventRepository) Withdraw(ctx context.Context, eventID string, profileID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE event_attendees SET status = 'canceled', updated_at = NOW()
		 WHERE event_id = $1 AND profile_id = $2 AND status = 'signed_up'`,
		eventID, profileID,
	)
	return err
}

func (r *EventRepository) GetAttendees(ctx context.Context, eventID string) ([]*profile.Profile, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.bsa_id, p.first_name, p.last_name, p.email, p.phone, p.birthdate,
		        p.member_type, p.status, p.user_id, p.created_at, p.updated_at
		 FROM profiles p
		 JOIN event_attendees ea ON ea.profile_id = p.id
		 WHERE ea.event_id = $1 AND ea.status = 'signed_up'`, eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*profile.Profile
	for rows.Next() {
		p := &profile.Profile{}
		if err := rows.Scan(&p.ID, &p.BSAID, &p.FirstName, &p.LastName, &p.Email, &p.Phone,
			&p.Birthdate, &p.MemberType, &p.Status, &p.UserID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

var _ event.Repository = (*EventRepository)(nil)
