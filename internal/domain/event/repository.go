package event

import (
	"context"

	"scout-app/internal/domain/user"
)

type Repository interface {
	Create(ctx context.Context, e *Event) error
	GetByID(ctx context.Context, id string) (*Event, error)
	ListUpcoming(ctx context.Context, limit int, offset int) ([]*ListItem, error)
	ListPast(ctx context.Context, limit int, offset int) ([]*ListItem, error)
	SignUp(ctx context.Context, eventID string, userID string) error
	Withdraw(ctx context.Context, eventID string, userID string) error
	GetAttendees(ctx context.Context, eventID string) ([]*user.User, error)
}
