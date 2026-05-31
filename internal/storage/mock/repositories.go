package mock

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"scout-app/internal/domain"
	"sort"
	"sync"
	"time"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// UserRepository is an in-memory implementation of domain.UserRepository.
type UserRepository struct {
	mu    sync.RWMutex
	users map[string]*domain.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]*domain.User),
	}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.users {
		if u.Email == user.Email {
			return fmt.Errorf("email %q already exists", user.Email)
		}
	}

	if user.ID == "" {
		user.ID = newUUID()
	}
	r.users[user.ID] = user
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

// RBACRepository is an in-memory implementation of domain.RBACRepository.
type RBACRepository struct {
	mu              sync.RWMutex
	roles           map[string]*domain.Role
	permissions     map[string]*domain.Permission
	userRoles       map[string][]string // userID -> []roleID
	rolePermissions map[string][]string // roleID -> []permID
}

func NewRBACRepository() *RBACRepository {
	return &RBACRepository{
		roles:           make(map[string]*domain.Role),
		permissions:     make(map[string]*domain.Permission),
		userRoles:       make(map[string][]string),
		rolePermissions: make(map[string][]string),
	}
}

func (r *RBACRepository) CreateRole(ctx context.Context, role *domain.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rl := range r.roles {
		if rl.Name == role.Name {
			return fmt.Errorf("role %q already exists", role.Name)
		}
	}

	if role.ID == "" {
		role.ID = newUUID()
	}
	r.roles[role.ID] = role
	return nil
}

func (r *RBACRepository) CreatePermission(ctx context.Context, perm *domain.Permission) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, p := range r.permissions {
		if p.Name == perm.Name {
			return fmt.Errorf("permission %q already exists", perm.Name)
		}
	}

	if perm.ID == "" {
		perm.ID = newUUID()
	}
	r.permissions[perm.ID] = perm
	return nil
}

func (r *RBACRepository) AssignRoleToUser(ctx context.Context, userID string, roleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if role exists
	if _, ok := r.roles[roleID]; !ok {
		return errors.New("role not found")
	}

	// Prevent duplicates
	for _, rid := range r.userRoles[userID] {
		if rid == roleID {
			return nil
		}
	}

	r.userRoles[userID] = append(r.userRoles[userID], roleID)
	return nil
}

func (r *RBACRepository) LinkPermissionToRole(ctx context.Context, roleID string, permID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if role and permission exist
	if _, ok := r.roles[roleID]; !ok {
		return errors.New("role not found")
	}
	if _, ok := r.permissions[permID]; !ok {
		return errors.New("permission not found")
	}

	// Prevent duplicates
	for _, pid := range r.rolePermissions[roleID] {
		if pid == permID {
			return nil
		}
	}

	r.rolePermissions[roleID] = append(r.rolePermissions[roleID], permID)
	return nil
}

func (r *RBACRepository) GetUserRoles(ctx context.Context, userID string) ([]*domain.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rids := r.userRoles[userID]
	var roles []*domain.Role
	for _, rid := range rids {
		if role, ok := r.roles[rid]; ok {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func (r *RBACRepository) GetUserPermissions(ctx context.Context, userID string) ([]*domain.Permission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rids := r.userRoles[userID]
	permSet := make(map[string]bool)
	var permissions []*domain.Permission

	for _, rid := range rids {
		pids := r.rolePermissions[rid]
		for _, pid := range pids {
			if !permSet[pid] {
				permSet[pid] = true
				if perm, ok := r.permissions[pid]; ok {
					permissions = append(permissions, perm)
				}
			}
		}
	}
	return permissions, nil
}

// EventRepository is an in-memory implementation of domain.EventRepository.
type EventRepository struct {
	mu        sync.RWMutex
	events    map[string]*domain.Event
	attendees map[string][]string // eventID -> []userID
}

// NewEventRepository creates a new in-memory EventRepository.
func NewEventRepository() *EventRepository {
	return &EventRepository{
		events:    make(map[string]*domain.Event),
		attendees: make(map[string][]string),
	}
}

// SeedEvents pre-populates events into the repository (used in tests).
func (r *EventRepository) SeedEvents(events []*domain.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, e := range events {
		if e.ID == "" {
			e.ID = newUUID()
		}
		r.events[e.ID] = e
	}
}

func (r *EventRepository) Create(ctx context.Context, event *domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if event.ID == "" {
		event.ID = newUUID()
	}
	clone := *event
	r.events[clone.ID] = &clone
	return nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	event, ok := r.events[id]
	if !ok {
		return nil, errors.New("event not found")
	}
	return event, nil
}

func (r *EventRepository) ListUpcoming(ctx context.Context, limit int, offset int) ([]*domain.EventListItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now()
	var filtered []*domain.Event
	for _, e := range r.events {
		if e.EndTime.After(now) {
			filtered = append(filtered, e)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartTime.Before(filtered[j].StartTime)
	})

	return r.toEventListItems(filtered, limit, offset), nil
}

func (r *EventRepository) ListPast(ctx context.Context, limit int, offset int) ([]*domain.EventListItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now()
	var filtered []*domain.Event
	for _, e := range r.events {
		if !e.EndTime.After(now) {
			filtered = append(filtered, e)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartTime.After(filtered[j].StartTime)
	})

	return r.toEventListItems(filtered, limit, offset), nil
}

// attendeesCount returns the number of signed-up attendees for an event.
func (r *EventRepository) attendeesCount(eventID string) int {
	return len(r.attendees[eventID])
}

// toEventListItems converts a sorted slice of events to EventListItem, applying pagination.
func (r *EventRepository) toEventListItems(events []*domain.Event, limit int, offset int) []*domain.EventListItem {
	if offset >= len(events) {
		return []*domain.EventListItem{}
	}

	start := offset
	end := offset + limit
	if end > len(events) {
		end = len(events)
	}

	slice := events[start:end]
	items := make([]*domain.EventListItem, len(slice))
	for i, e := range slice {
		items[i] = &domain.EventListItem{
			ID:            e.ID,
			Title:         e.Title,
			Location:      e.Location,
			StartTime:     e.StartTime,
			EndTime:       e.EndTime,
			Type:          e.Type,
			AttendeeCount: r.attendeesCount(e.ID),
		}
	}
	return items
}

func (r *EventRepository) SignUp(ctx context.Context, eventID string, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.events[eventID]; !ok {
		return errors.New("event not found")
	}

	for _, uid := range r.attendees[eventID] {
		if uid == userID {
			return nil // already signed up
		}
	}

	r.attendees[eventID] = append(r.attendees[eventID], userID)
	return nil
}

func (r *EventRepository) Withdraw(ctx context.Context, eventID string, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.events[eventID]; !ok {
		return errors.New("event not found")
	}

	attendees := r.attendees[eventID]
	for i, uid := range attendees {
		if uid == userID {
			r.attendees[eventID] = append(attendees[:i], attendees[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *EventRepository) GetAttendees(ctx context.Context, eventID string) ([]*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.events[eventID]; !ok {
		return nil, errors.New("event not found")
	}

	// We store user IDs as strings; return empty list since we don't store full users here.
	return []*domain.User{}, nil
}
