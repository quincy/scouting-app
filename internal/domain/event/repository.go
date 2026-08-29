package event

import (
	"context"

	"scout-app/internal/domain/profile"
)

type Repository interface {
	Create(ctx context.Context, e *Event) error
	GetByID(ctx context.Context, id string) (*Event, error)
	ListUpcoming(ctx context.Context, limit int, offset int) ([]*ListItem, error)
	ListPast(ctx context.Context, limit int, offset int) ([]*ListItem, error)
	ListUpcomingByProfileID(ctx context.Context, profileID string, limit int, offset int) ([]*ListItem, error)
	ListPastByProfileID(ctx context.Context, profileID string, limit int, offset int) ([]*ListItem, error)
	SignUp(ctx context.Context, eventID string, profileID string) error
	Withdraw(ctx context.Context, eventID string, profileID string) error
	Update(ctx context.Context, e *Event) error
	Delete(ctx context.Context, id string) error
	GetAttendees(ctx context.Context, eventID string) ([]*profile.Profile, error)
	AddDriver(ctx context.Context, eventID string, profileID string, seatbeltCount int) error
	RemoveDriver(ctx context.Context, eventID string, profileID string) error
	UpdateDriverSeatbeltCount(ctx context.Context, eventID string, profileID string, seatbeltCount int) error
	GetDrivers(ctx context.Context, eventID string) ([]DriverResponsibility, error)
	GetSeatbeltSummary(ctx context.Context, eventID string) (*SeatbeltSummary, error)
	AssignResponsibility(ctx context.Context, eventID string, profileID string, responsibility Responsibility) error
	RemoveResponsibility(ctx context.Context, eventID string, profileID string, responsibility Responsibility) error
	GetResponsibilities(ctx context.Context, eventID string) ([]ResponsibilityAssignment, error)
	GetResponsibilityHolder(ctx context.Context, eventID string, responsibility Responsibility) (*ResponsibilityHolder, error)
	CreateCookingPatrol(ctx context.Context, eventID string, isAdult bool) (*CookingPatrol, error)
	DeleteCookingPatrol(ctx context.Context, patrolID string) error
	AssignCookingPatrolMember(ctx context.Context, eventID string, patrolID string, profileID string) error
	RemoveCookingPatrolMember(ctx context.Context, eventID string, profileID string) error
	SetCookingPatrolCook(ctx context.Context, eventID string, patrolID string, profileID string) error
	ClearCookingPatrolCook(ctx context.Context, patrolID string) error
	ListCookingPatrols(ctx context.Context, eventID string) ([]*CookingPatrol, error)
}
