package postgres

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type Store struct {
	DB               *sql.DB
	User             *UserRepository
	Profile          *ProfileRepository
	Event            *EventRepository
	RBAC             *RBACRepository
	OTPCode          *OTPCodeRepository
	ParentYouthLink  *ParentYouthLinkRepository
	ScoutbookSession *ScoutbookSessionRepository
	AppConfig        *AppConfigRepository
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		DB:               db,
		User:             NewUserRepository(db),
		Profile:          NewProfileRepository(db),
		Event:            NewEventRepository(db),
		RBAC:             NewRBACRepository(db),
		OTPCode:          NewOTPCodeRepository(db),
		ParentYouthLink:  NewParentYouthLinkRepository(db),
		ScoutbookSession: NewScoutbookSessionRepository(db),
		AppConfig:        NewAppConfigRepository(db),
	}
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func coalesceTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}
