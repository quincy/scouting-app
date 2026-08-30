package event

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const TentNamePrefix = "Tent "

func TentNextName(n int) string {
	return fmt.Sprintf("%s%d", TentNamePrefix, n)
}

func TentNumber(name string) (int, bool) {
	if !strings.HasPrefix(name, TentNamePrefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(name, TentNamePrefix))
	if err != nil {
		return 0, false
	}
	return n, true
}

type Tent struct {
	ID        string
	EventID   string
	Name      string
	CreatedAt time.Time
	Members   []TentMember
}

type TentMember struct {
	EventID     string
	TentID      string
	ProfileID   string
	ProfileName string
	CreatedAt   time.Time
}
