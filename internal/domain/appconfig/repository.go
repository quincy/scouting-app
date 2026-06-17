package appconfig

import "context"

type Repository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	All(ctx context.Context) (map[string]string, error)
}
