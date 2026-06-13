package parentyouthlink

import "context"

type Repository interface {
	Create(ctx context.Context, conn *ParentYouthConnection) error
	GetByID(ctx context.Context, id string) (*ParentYouthConnection, error)
	ListAll(ctx context.Context) ([]*ParentYouthConnection, error)
	ListByParent(ctx context.Context, parentProfileID string) ([]*ParentYouthConnection, error)
	ListByYouth(ctx context.Context, youthProfileID string) ([]*ParentYouthConnection, error)
	ListByStatus(ctx context.Context, status Status) ([]*ParentYouthConnection, error)
	UpdateStatus(ctx context.Context, id string, status Status, approvedBy string) error
}
