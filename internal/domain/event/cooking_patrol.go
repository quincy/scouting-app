package event

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const CookingPatrolAdultsName = "Adults"

const CookingPatrolYouthNamePrefix = "Cooking "

func CookingPatrolNextName(n int) string {
	return fmt.Sprintf("%s%d", CookingPatrolYouthNamePrefix, n)
}

func CookingPatrolNumber(name string) (int, bool) {
	if !strings.HasPrefix(name, CookingPatrolYouthNamePrefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(name, CookingPatrolYouthNamePrefix))
	if err != nil {
		return 0, false
	}
	return n, true
}

type CookingPatrol struct {
	ID        string
	EventID   string
	Name      string
	IsAdult   bool
	CreatedAt time.Time
	Members   []CookingPatrolMember
}

type CookingPatrolMember struct {
	EventID     string
	PatrolID    string
	ProfileID   string
	ProfileName string
	IsCook      bool
	CreatedAt   time.Time
}
