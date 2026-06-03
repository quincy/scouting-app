package scoutbooksession

import "context"

type Repository interface {
	Create(ctx context.Context, session *Session) error
	GetLatest(ctx context.Context) (*Session, error)
}
