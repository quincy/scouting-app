package mock

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/email"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/otpcode"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
	"scout-app/internal/domain/scoutbooksession"
	"scout-app/internal/domain/user"
	"sort"
	"sync"
	"time"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type UserRepository struct {
	mu    sync.RWMutex
	users map[string]*user.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]*user.User),
	}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.users {
		if existing.Email == u.Email {
			return fmt.Errorf("email %q already exists", u.Email)
		}
	}
	if u.ID == "" {
		u.ID = newUUID()
	}
	r.users[u.ID] = u
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

type RBACRepository struct {
	mu              sync.RWMutex
	roles           map[string]*rbac.Role
	permissions     map[string]*rbac.Permission
	userRoles       map[string][]string
	rolePermissions map[string][]string
}

func NewRBACRepository() *RBACRepository {
	return &RBACRepository{
		roles:           make(map[string]*rbac.Role),
		permissions:     make(map[string]*rbac.Permission),
		userRoles:       make(map[string][]string),
		rolePermissions: make(map[string][]string),
	}
}

func (r *RBACRepository) SeedRoles(ctx context.Context) error {
	return auth.SeedRoles(ctx, r)
}

func (r *RBACRepository) CreateRole(ctx context.Context, role *rbac.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rl := range r.roles {
		if rl.Name == role.Name {
			role.ID = rl.ID
			return nil
		}
	}
	if role.ID == "" {
		role.ID = newUUID()
	}
	r.roles[role.ID] = role
	return nil
}

func (r *RBACRepository) CreatePermission(ctx context.Context, perm *rbac.Permission) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.permissions {
		if p.Name == perm.Name {
			perm.ID = p.ID
			return nil
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
	if _, ok := r.roles[roleID]; !ok {
		return errors.New("role not found")
	}
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
	if _, ok := r.roles[roleID]; !ok {
		return errors.New("role not found")
	}
	if _, ok := r.permissions[permID]; !ok {
		return errors.New("permission not found")
	}
	for _, pid := range r.rolePermissions[roleID] {
		if pid == permID {
			return nil
		}
	}
	r.rolePermissions[roleID] = append(r.rolePermissions[roleID], permID)
	return nil
}

func (r *RBACRepository) RemoveRoleFromUser(ctx context.Context, userID string, roleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rids := r.userRoles[userID]
	for i, rid := range rids {
		if rid == roleID {
			r.userRoles[userID] = append(rids[:i], rids[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *RBACRepository) GetUserRoles(ctx context.Context, userID string) ([]*rbac.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rids := r.userRoles[userID]
	var roles []*rbac.Role
	for _, rid := range rids {
		if role, ok := r.roles[rid]; ok {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func (r *RBACRepository) GetUserPermissions(ctx context.Context, userID string) ([]*rbac.Permission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rids := r.userRoles[userID]
	permSet := make(map[string]bool)
	var permissions []*rbac.Permission
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

func (r *RBACRepository) GetRoleByName(ctx context.Context, name string) (*rbac.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, role := range r.roles {
		if role.Name == name {
			return role, nil
		}
	}
	return nil, fmt.Errorf("role %q not found", name)
}

type EventRepository struct {
	mu        sync.RWMutex
	events    map[string]*event.Event
	attendees map[string][]*profile.Profile
	profiles  *ProfileRepository
}

func NewEventRepository(profiles *ProfileRepository) *EventRepository {
	return &EventRepository{
		events:    make(map[string]*event.Event),
		attendees: make(map[string][]*profile.Profile),
		profiles:  profiles,
	}
}

func (r *EventRepository) SeedEvents(events []*event.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range events {
		if e.ID == "" {
			e.ID = newUUID()
		}
		r.events[e.ID] = e
	}
}

func (r *EventRepository) Create(ctx context.Context, e *event.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.ID == "" {
		e.ID = newUUID()
	}
	clone := *e
	r.events[clone.ID] = &clone
	return nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (*event.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.events[id]
	if !ok {
		return nil, errors.New("event not found")
	}
	return e, nil
}

func (r *EventRepository) ListUpcoming(ctx context.Context, limit int, offset int) ([]*event.ListItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now()
	var filtered []*event.Event
	for _, e := range r.events {
		if e.EndTime.After(now) {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartTime.Before(filtered[j].StartTime)
	})
	return r.toListItems(filtered, limit, offset), nil
}

func (r *EventRepository) ListPast(ctx context.Context, limit int, offset int) ([]*event.ListItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now()
	var filtered []*event.Event
	for _, e := range r.events {
		if !e.EndTime.After(now) {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartTime.After(filtered[j].StartTime)
	})
	return r.toListItems(filtered, limit, offset), nil
}

func (r *EventRepository) attendeesCount(eventID string) int {
	return len(r.attendees[eventID])
}

func (r *EventRepository) AttendeesMap() map[string][]*profile.Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string][]*profile.Profile, len(r.attendees))
	for k, v := range r.attendees {
		profiles := make([]*profile.Profile, len(v))
		copy(profiles, v)
		result[k] = profiles
	}
	return result
}

func (r *EventRepository) toListItems(events []*event.Event, limit int, offset int) []*event.ListItem {
	if offset >= len(events) {
		return []*event.ListItem{}
	}
	start := offset
	end := offset + limit
	if end > len(events) {
		end = len(events)
	}
	slice := events[start:end]
	items := make([]*event.ListItem, len(slice))
	for i, e := range slice {
		items[i] = &event.ListItem{
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

func (r *EventRepository) SignUp(ctx context.Context, eventID string, profileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.events[eventID]; !ok {
		return errors.New("event not found")
	}
	p, err := r.profiles.GetByID(ctx, profileID)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}
	for _, existing := range r.attendees[eventID] {
		if existing.ID == profileID {
			return nil
		}
	}
	r.attendees[eventID] = append(r.attendees[eventID], p)
	return nil
}

func (r *EventRepository) Withdraw(ctx context.Context, eventID string, profileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.events[eventID]; !ok {
		return errors.New("event not found")
	}
	attendees := r.attendees[eventID]
	for i, p := range attendees {
		if p.ID == profileID {
			r.attendees[eventID] = append(attendees[:i], attendees[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *EventRepository) GetAttendees(ctx context.Context, eventID string) ([]*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.events[eventID]; !ok {
		return nil, errors.New("event not found")
	}
	result := make([]*profile.Profile, len(r.attendees[eventID]))
	for i, p := range r.attendees[eventID] {
		clone := *p
		result[i] = &clone
	}
	return result, nil
}

type ProfileRepository struct {
	mu       sync.RWMutex
	profiles map[string]*profile.Profile
}

func NewProfileRepository() *ProfileRepository {
	return &ProfileRepository{
		profiles: make(map[string]*profile.Profile),
	}
}

func (r *ProfileRepository) Create(ctx context.Context, p *profile.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.BSAID != "" {
		for _, existing := range r.profiles {
			if existing.BSAID == p.BSAID {
				return fmt.Errorf("profile with BSA ID %q already exists", p.BSAID)
			}
		}
	}
	if p.ID == "" {
		p.ID = newUUID()
	}
	clone := *p
	r.profiles[clone.ID] = &clone
	return nil
}

func (r *ProfileRepository) GetByID(ctx context.Context, id string) (*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.profiles[id]
	if !ok {
		return nil, errors.New("profile not found")
	}
	clone := *p
	return &clone, nil
}

func (r *ProfileRepository) GetByEmail(ctx context.Context, email string) (*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.profiles {
		if p.Email == email {
			clone := *p
			return &clone, nil
		}
	}
	return nil, errors.New("profile not found")
}

func (r *ProfileRepository) GetByBSAID(ctx context.Context, bsaID string) (*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.profiles {
		if p.BSAID == bsaID {
			clone := *p
			return &clone, nil
		}
	}
	return nil, errors.New("profile not found")
}

func (r *ProfileRepository) GetByUserID(ctx context.Context, userID string) (*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.profiles {
		if p.UserID != nil && *p.UserID == userID {
			clone := *p
			return &clone, nil
		}
	}
	return nil, errors.New("profile not found for user")
}

func (r *ProfileRepository) ListAll(ctx context.Context) ([]*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*profile.Profile
	for _, p := range r.profiles {
		clone := *p
		result = append(result, &clone)
	}
	return result, nil
}

func (r *ProfileRepository) ListByStatus(ctx context.Context, status profile.Status) ([]*profile.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*profile.Profile
	for _, p := range r.profiles {
		if p.Status == status {
			clone := *p
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (r *ProfileRepository) Update(ctx context.Context, p *profile.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.profiles[p.ID]; !ok {
		return errors.New("profile not found")
	}
	clone := *p
	r.profiles[clone.ID] = &clone
	return nil
}

type OTPCodeRepository struct {
	mu    sync.RWMutex
	codes map[string]*otpcode.OTPCode
}

func NewOTPCodeRepository() *OTPCodeRepository {
	return &OTPCodeRepository{
		codes: make(map[string]*otpcode.OTPCode),
	}
}

func (r *OTPCodeRepository) Create(ctx context.Context, otp *otpcode.OTPCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if otp.ID == "" {
		otp.ID = newUUID()
	}
	clone := *otp
	clone.Used = false
	clone.Attempts = 0
	r.codes[clone.ID] = &clone
	return nil
}

func (r *OTPCodeRepository) GetByID(ctx context.Context, id string) (*otpcode.OTPCode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.codes[id]
	if !ok {
		return nil, errors.New("otp not found")
	}
	clone := *c
	return &clone, nil
}

func (r *OTPCodeRepository) CountByEmailSince(ctx context.Context, email string, since time.Time) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int
	for _, c := range r.codes {
		if c.Email == email && c.CreatedAt.After(since) {
			count++
		}
	}
	return count, nil
}

func (r *OTPCodeRepository) MarkUsedIfUnused(ctx context.Context, id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.codes[id]
	if !ok {
		return false, nil
	}
	if c.Used {
		return false, nil
	}
	c.Used = true
	return true, nil
}

func (r *OTPCodeRepository) IncrementAttempts(ctx context.Context, id string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.codes[id]
	if !ok {
		return 0, errors.New("otp not found")
	}
	c.Attempts++
	return c.Attempts, nil
}

func (r *OTPCodeRepository) InvalidateByEmail(ctx context.Context, email string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.codes {
		if c.Email == email && !c.Used && !c.IsExpired() {
			c.Used = true
		}
	}
	return nil
}

func (r *OTPCodeRepository) DeleteExpired(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, c := range r.codes {
		if c.IsExpired() {
			delete(r.codes, id)
		}
	}
	return nil
}

type ParentYouthLinkRepository struct {
	mu    sync.RWMutex
	links map[string]*parentyouthlink.ParentYouthConnection
}

func NewParentYouthLinkRepository() *ParentYouthLinkRepository {
	return &ParentYouthLinkRepository{
		links: make(map[string]*parentyouthlink.ParentYouthConnection),
	}
}

func (r *ParentYouthLinkRepository) Create(ctx context.Context, link *parentyouthlink.ParentYouthConnection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if link.ID == "" {
		link.ID = newUUID()
	}
	clone := *link
	r.links[clone.ID] = &clone
	return nil
}

func (r *ParentYouthLinkRepository) GetByID(ctx context.Context, id string) (*parentyouthlink.ParentYouthConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.links[id]
	if !ok {
		return nil, errors.New("link not found")
	}
	clone := *l
	return &clone, nil
}

func (r *ParentYouthLinkRepository) ListAll(ctx context.Context) ([]*parentyouthlink.ParentYouthConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*parentyouthlink.ParentYouthConnection
	for _, l := range r.links {
		clone := *l
		result = append(result, &clone)
	}
	return result, nil
}

func (r *ParentYouthLinkRepository) ListByParent(ctx context.Context, parentProfileID string) ([]*parentyouthlink.ParentYouthConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*parentyouthlink.ParentYouthConnection
	for _, l := range r.links {
		if l.ParentProfileID == parentProfileID {
			clone := *l
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (r *ParentYouthLinkRepository) ListByYouth(ctx context.Context, youthProfileID string) ([]*parentyouthlink.ParentYouthConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*parentyouthlink.ParentYouthConnection
	for _, l := range r.links {
		if l.YouthProfileID == youthProfileID {
			clone := *l
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (r *ParentYouthLinkRepository) ListByStatus(ctx context.Context, status parentyouthlink.Status) ([]*parentyouthlink.ParentYouthConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*parentyouthlink.ParentYouthConnection
	for _, l := range r.links {
		if l.Status == status {
			clone := *l
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (r *ParentYouthLinkRepository) UpdateStatus(ctx context.Context, id string, status parentyouthlink.Status, approvedBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.links[id]
	if !ok {
		return errors.New("link not found")
	}
	l.Status = status
	l.ApprovedBy = &approvedBy
	now := time.Now()
	l.ApprovedAt = &now
	return nil
}

type ScoutbookSessionRepository struct {
	mu       sync.RWMutex
	sessions map[string]*scoutbooksession.Session
}

func NewScoutbookSessionRepository() *ScoutbookSessionRepository {
	return &ScoutbookSessionRepository{
		sessions: make(map[string]*scoutbooksession.Session),
	}
}

func (r *ScoutbookSessionRepository) Create(ctx context.Context, s *scoutbooksession.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.ID == "" {
		s.ID = newUUID()
	}
	clone := *s
	r.sessions[clone.ID] = &clone
	return nil
}

func (r *ScoutbookSessionRepository) GetLatest(ctx context.Context) (*scoutbooksession.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var latest *scoutbooksession.Session
	for _, s := range r.sessions {
		if latest == nil || s.CreatedAt.After(latest.CreatedAt) {
			clone := *s
			latest = &clone
		}
	}
	if latest == nil {
		return nil, errors.New("no sessions found")
	}
	return latest, nil
}

type EmailService struct {
	SentOTPs []EmailOTP
	mu       sync.RWMutex
}

type EmailOTP struct {
	To    string
	Code  string
	OTPID string
}

func NewEmailService() *EmailService {
	return &EmailService{}
}

func (s *EmailService) SendOTP(ctx context.Context, to, code string, otpID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SentOTPs = append(s.SentOTPs, EmailOTP{To: to, Code: code, OTPID: otpID})
	return nil
}

var _ email.Service = (*EmailService)(nil)
