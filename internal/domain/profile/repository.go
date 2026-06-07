package profile

import "context"

type Repository interface {
	Create(ctx context.Context, p *Profile) error
	GetByID(ctx context.Context, id string) (*Profile, error)
	GetByEmail(ctx context.Context, email string) (*Profile, error)
	GetByBSAID(ctx context.Context, bsaID string) (*Profile, error)
	GetByUserID(ctx context.Context, userID string) (*Profile, error)
	ListAll(ctx context.Context) ([]*Profile, error)
	ListByStatus(ctx context.Context, status Status) ([]*Profile, error)
	Update(ctx context.Context, p *Profile) error
}
