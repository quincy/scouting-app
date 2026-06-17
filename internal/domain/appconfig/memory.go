package appconfig

import (
	"context"
	"sync"
)

type InMemoryRepository struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		data: make(map[string]string),
	}
}

func (r *InMemoryRepository) Get(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.data[key], nil
}

func (r *InMemoryRepository) Set(ctx context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = value
	return nil
}

func (r *InMemoryRepository) All(ctx context.Context) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]string, len(r.data))
	for k, v := range r.data {
		result[k] = v
	}
	return result, nil
}
