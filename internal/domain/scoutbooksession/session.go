package scoutbooksession

import "time"

type Session struct {
	ID         string
	Token      string
	PersonGUID string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}
