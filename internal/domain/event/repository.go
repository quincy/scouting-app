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
	SignUp(ctx context.Context, eventID string, profileID string) error
	Withdraw(ctx context.Context, eventID string, profileID string) error
	Update(ctx context.Context, e *Event) error
	Delete(ctx context.Context, id string) error
	GetAttendees(ctx context.Context, eventID string) ([]*profile.Profile, error)
}
