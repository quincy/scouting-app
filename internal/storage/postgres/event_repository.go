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
	e.CreatedAt = now
	e.UpdatedAt = now
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

func (r *EventRepository) ListUpcomingByProfileID(ctx context.Context, profileID string, limit int, offset int) ([]*event.ListItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.title, e.location, e.start_time, e.end_time, e.type, COUNT(ea2.profile_id) AS attendee_count
		 FROM events e
		 JOIN event_attendees ea ON ea.event_id = e.id AND ea.profile_id = $1 AND ea.status = 'signed_up'
		 LEFT JOIN event_attendees ea2 ON ea2.event_id = e.id AND ea2.status = 'signed_up'
		 WHERE e.end_time > NOW()
		 GROUP BY e.id
		 ORDER BY e.start_time ASC
		 LIMIT $2 OFFSET $3`, profileID, limit, offset,
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

func (r *EventRepository) ListPastByProfileID(ctx context.Context, profileID string, limit int, offset int) ([]*event.ListItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.title, e.location, e.start_time, e.end_time, e.type, COUNT(ea2.profile_id) AS attendee_count
		 FROM events e
		 JOIN event_attendees ea ON ea.event_id = e.id AND ea.profile_id = $1 AND ea.status = 'signed_up'
		 LEFT JOIN event_attendees ea2 ON ea2.event_id = e.id AND ea2.status = 'signed_up'
		 WHERE e.end_time <= NOW()
		 GROUP BY e.id
		 ORDER BY e.start_time DESC
		 LIMIT $2 OFFSET $3`, profileID, limit, offset,
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

func (r *EventRepository) AddDriver(ctx context.Context, eventID string, profileID string, seatbeltCount int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO event_attendee_responsibilities (event_id, profile_id, responsibility, seatbelt_count, created_at, updated_at)
		 VALUES ($1, $2, 'driver', $3, NOW(), NOW())
		 ON CONFLICT (event_id, profile_id, responsibility) DO UPDATE SET seatbelt_count = $3, updated_at = NOW()`,
		eventID, profileID, seatbeltCount,
	)
	return err
}

func (r *EventRepository) RemoveDriver(ctx context.Context, eventID string, profileID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM event_attendee_responsibilities
		 WHERE event_id = $1 AND profile_id = $2 AND responsibility = 'driver'`,
		eventID, profileID,
	)
	return err
}

func (r *EventRepository) UpdateDriverSeatbeltCount(ctx context.Context, eventID string, profileID string, seatbeltCount int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE event_attendee_responsibilities
		 SET seatbelt_count = $3, updated_at = NOW()
		 WHERE event_id = $1 AND profile_id = $2 AND responsibility = 'driver'`,
		eventID, profileID, seatbeltCount,
	)
	return err
}

func (r *EventRepository) GetDrivers(ctx context.Context, eventID string) ([]event.DriverResponsibility, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ear.event_id, ear.profile_id,
		        CASE WHEN p.nickname != '' THEN p.nickname || ' (' || p.first_name || ') ' || p.last_name
		             ELSE p.first_name || ' ' || p.last_name
		        END AS profile_name,
		        ear.seatbelt_count, ear.created_at, ear.updated_at
		 FROM event_attendee_responsibilities ear
		 JOIN profiles p ON p.id = ear.profile_id
		 WHERE ear.event_id = $1 AND ear.responsibility = 'driver'`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drivers []event.DriverResponsibility
	for rows.Next() {
		var d event.DriverResponsibility
		if err := rows.Scan(&d.EventID, &d.ProfileID, &d.ProfileName, &d.SeatbeltCount, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		drivers = append(drivers, d)
	}
	return drivers, rows.Err()
}

func (r *EventRepository) GetSeatbeltSummary(ctx context.Context, eventID string) (*event.SeatbeltSummary, error) {
	var summary event.SeatbeltSummary
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(ear.seatbelt_count), 0) AS total_seatbelts,
		        (SELECT COUNT(*) FROM event_attendees WHERE event_id = $1 AND status = 'signed_up') AS required_seatbelts
		 FROM event_attendee_responsibilities ear
		 WHERE ear.event_id = $1 AND ear.responsibility = 'driver'`,
		eventID,
	).Scan(&summary.TotalSeatbelts, &summary.RequiredSeatbelts)
	if err != nil {
		return nil, err
	}
	summary.Available = summary.TotalSeatbelts
	summary.Sufficient = summary.Available >= summary.RequiredSeatbelts
	return &summary, nil
}

func (r *EventRepository) AssignResponsibility(ctx context.Context, eventID string, profileID string, responsibility event.Responsibility) error {
	if event.IsSingleton(responsibility) {
		var currentHolderID string
		err := r.db.QueryRowContext(ctx,
			`SELECT ear.profile_id
			 FROM event_attendee_responsibilities ear
			 WHERE ear.event_id = $1 AND ear.responsibility = $2 AND ear.profile_id != $3`,
			eventID, string(responsibility), profileID,
		).Scan(&currentHolderID)
		if err == nil {
			var currentHolderName string
			r.db.QueryRowContext(ctx,
				`SELECT CASE WHEN p.nickname != '' THEN p.nickname || ' (' || p.first_name || ') ' || p.last_name
				            ELSE p.first_name || ' ' || p.last_name
				       END
				 FROM profiles p WHERE p.id = $1`, currentHolderID,
			).Scan(&currentHolderName)
			return event.ErrSingletonConflict{
				Responsibility:     responsibility,
				CurrentHolderID:    currentHolderID,
				CurrentHolderName:  currentHolderName,
				RequestedProfileID: profileID,
			}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO event_attendee_responsibilities (event_id, profile_id, responsibility, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())
		 ON CONFLICT (event_id, profile_id, responsibility) DO UPDATE SET updated_at = NOW()`,
		eventID, profileID, string(responsibility),
	)
	return err
}

func (r *EventRepository) RemoveResponsibility(ctx context.Context, eventID string, profileID string, responsibility event.Responsibility) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM event_attendee_responsibilities
		 WHERE event_id = $1 AND profile_id = $2 AND responsibility = $3`,
		eventID, profileID, string(responsibility),
	)
	return err
}

func (r *EventRepository) GetResponsibilities(ctx context.Context, eventID string) ([]event.ResponsibilityAssignment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ear.event_id, ear.profile_id,
		        CASE WHEN p.nickname != '' THEN p.nickname || ' (' || p.first_name || ') ' || p.last_name
		             ELSE p.first_name || ' ' || p.last_name
		        END AS profile_name,
		        ear.responsibility, ear.created_at, ear.updated_at
		 FROM event_attendee_responsibilities ear
		 JOIN profiles p ON p.id = ear.profile_id
		 WHERE ear.event_id = $1
		 ORDER BY ear.responsibility`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []event.ResponsibilityAssignment
	for rows.Next() {
		var ra event.ResponsibilityAssignment
		var resp string
		if err := rows.Scan(&ra.EventID, &ra.ProfileID, &ra.ProfileName, &resp, &ra.CreatedAt, &ra.UpdatedAt); err != nil {
			return nil, err
		}
		ra.Responsibility = event.Responsibility(resp)
		result = append(result, ra)
	}
	return result, rows.Err()
}

func (r *EventRepository) GetResponsibilityHolder(ctx context.Context, eventID string, responsibility event.Responsibility) (*event.ResponsibilityHolder, error) {
	var h event.ResponsibilityHolder
	err := r.db.QueryRowContext(ctx,
		`SELECT ear.profile_id,
		        CASE WHEN p.nickname != '' THEN p.nickname || ' (' || p.first_name || ') ' || p.last_name
		             ELSE p.first_name || ' ' || p.last_name
		        END AS profile_name
		 FROM event_attendee_responsibilities ear
		 JOIN profiles p ON p.id = ear.profile_id
		 WHERE ear.event_id = $1 AND ear.responsibility = $2`,
		eventID, string(responsibility),
	).Scan(&h.ProfileID, &h.ProfileName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *EventRepository) CreateCookingPatrol(ctx context.Context, eventID string, isAdult bool) (*event.CookingPatrol, error) {
	p := &event.CookingPatrol{
		ID:      newUUID(),
		EventID: eventID,
		Name:    event.CookingPatrolAdultsName,
		IsAdult: isAdult,
		Members: []event.CookingPatrolMember{},
	}
	if isAdult {
		if err := r.db.QueryRowContext(ctx,
			`INSERT INTO event_cooking_patrols (id, event_id, name, is_adult)
			 VALUES ($1, $2, $3, TRUE)
			 RETURNING created_at`,
			p.ID, p.EventID, p.Name,
		).Scan(&p.CreatedAt); err != nil {
			return nil, err
		}
		return p, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT name FROM event_cooking_patrols
		 WHERE event_id = $1 AND is_adult = FALSE
		 FOR UPDATE`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	highest := 0
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		if n, ok := event.CookingPatrolNumber(name); ok && n > highest {
			highest = n
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	p.Name = event.CookingPatrolNextName(highest + 1)
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO event_cooking_patrols (id, event_id, name, is_adult)
		 VALUES ($1, $2, $3, FALSE)
		 RETURNING created_at`,
		p.ID, p.EventID, p.Name,
	).Scan(&p.CreatedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *EventRepository) ListCookingPatrols(ctx context.Context, eventID string) ([]*event.CookingPatrol, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, is_adult, created_at
		 FROM event_cooking_patrols
		 WHERE event_id = $1
		 ORDER BY created_at, id`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	patrols := []*event.CookingPatrol{}
	for rows.Next() {
		p := &event.CookingPatrol{}
		if err := rows.Scan(&p.ID, &p.Name, &p.IsAdult, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.EventID = eventID
		p.Members = []event.CookingPatrolMember{}
		patrols = append(patrols, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	memberRows, err := r.db.QueryContext(ctx,
		`SELECT m.event_id, m.patrol_id, m.profile_id,
		        CASE WHEN p.nickname != '' THEN p.nickname || ' (' || p.first_name || ') ' || p.last_name
		             ELSE p.first_name || ' ' || p.last_name
		        END AS profile_name,
		        m.is_cook, m.created_at
		 FROM event_cooking_patrol_members m
		 JOIN profiles p ON p.id = m.profile_id
		 WHERE m.event_id = $1
		 ORDER BY m.created_at, m.profile_id`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer memberRows.Close()

	membersByPatrol := make(map[string][]event.CookingPatrolMember)
	for memberRows.Next() {
		var m event.CookingPatrolMember
		if err := memberRows.Scan(&m.EventID, &m.PatrolID, &m.ProfileID, &m.ProfileName, &m.IsCook, &m.CreatedAt); err != nil {
			return nil, err
		}
		membersByPatrol[m.PatrolID] = append(membersByPatrol[m.PatrolID], m)
	}
	if err := memberRows.Err(); err != nil {
		return nil, err
	}

	for _, p := range patrols {
		p.Members = membersByPatrol[p.ID]
		if p.Members == nil {
			p.Members = []event.CookingPatrolMember{}
		}
	}
	return patrols, nil
}

func (r *EventRepository) DeleteCookingPatrol(ctx context.Context, patrolID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM event_cooking_patrols WHERE id = $1`, patrolID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("cooking patrol not found")
	}
	return nil
}

func (r *EventRepository) AssignCookingPatrolMember(ctx context.Context, eventID string, patrolID string, profileID string) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO event_cooking_patrol_members (event_id, profile_id, patrol_id, is_cook, created_at, updated_at)
		 SELECT $1, $2, p.id, FALSE, NOW(), NOW()
		 FROM event_cooking_patrols p
		 WHERE p.id = $3 AND p.event_id = $1
		 ON CONFLICT (event_id, profile_id) DO UPDATE SET patrol_id = EXCLUDED.patrol_id, is_cook = FALSE, updated_at = NOW()`,
		eventID, profileID, patrolID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("cooking patrol not found")
	}
	return nil
}

func (r *EventRepository) RemoveCookingPatrolMember(ctx context.Context, eventID string, profileID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM event_cooking_patrol_members WHERE event_id = $1 AND profile_id = $2`,
		eventID, profileID,
	)
	return err
}

func (r *EventRepository) SetCookingPatrolCook(ctx context.Context, eventID string, patrolID string, profileID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var member bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM event_cooking_patrol_members
			WHERE event_id = $1 AND patrol_id = $2 AND profile_id = $3
		 )`,
		eventID, patrolID, profileID,
	).Scan(&member); err != nil {
		return err
	}
	if !member {
		return errors.New("profile is not a member of this cooking patrol")
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE event_cooking_patrol_members
		 SET is_cook = FALSE, updated_at = NOW()
		 WHERE event_id = $1 AND patrol_id = $2 AND is_cook`,
		eventID, patrolID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE event_cooking_patrol_members
		 SET is_cook = TRUE, updated_at = NOW()
		 WHERE event_id = $1 AND patrol_id = $2 AND profile_id = $3`,
		eventID, patrolID, profileID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *EventRepository) ClearCookingPatrolCook(ctx context.Context, patrolID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE event_cooking_patrol_members SET is_cook = FALSE, updated_at = NOW() WHERE patrol_id = $1`,
		patrolID,
	)
	return err
}

var _ event.Repository = (*EventRepository)(nil)
