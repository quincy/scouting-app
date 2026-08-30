package event

import (
	"fmt"
	"strings"
	"time"
)

// ApprovedParentLink is an Approved Parent Youth Connection between a parent
// Profile and a youth Profile. The caller MUST pass only links whose status is
// approved; this type does not carry status and does not verify it, so any
// pending/rejected/revoked connection passed here incorrectly exempts a pair
// from the age-gap rule.
type ApprovedParentLink struct {
	ParentProfileID string
	YouthProfileID  string
}

// ScoutPair is an unordered pair of scout profile IDs.
type ScoutPair struct {
	A string
	B string
}

// SiblingPairs returns the set of unordered scout pairs that share at least
// one parent, i.e., siblings. Pairs are keyed by the smaller scout ID in
// ScoutPair.A so lookups are order-independent. links must contain only
// approved Parent Youth Connections (see ApprovedParentLink).
func SiblingPairs(links []ApprovedParentLink) map[ScoutPair]bool {
	parentToYouth := make(map[string][]string)
	for _, l := range links {
		parentToYouth[l.ParentProfileID] = append(parentToYouth[l.ParentProfileID], l.YouthProfileID)
	}
	pairs := make(map[ScoutPair]bool)
	for _, youths := range parentToYouth {
		for i := 0; i < len(youths); i++ {
			for j := i + 1; j < len(youths); j++ {
				pairs[normalizePair(youths[i], youths[j])] = true
			}
		}
	}
	return pairs
}

func normalizePair(a, b string) ScoutPair {
	if a <= b {
		return ScoutPair{A: a, B: b}
	}
	return ScoutPair{A: b, B: a}
}

// AgeWholeYears returns a scout's age in whole years on the calendar date of
// at. A birthday counts on the day it falls: a scout born on Feb 29 is
// treated as having their birthday on Feb 28 in non-leap years. If the
// birthdate is after at, 0 is returned.
func AgeWholeYears(birthdate, at time.Time) int {
	if at.Before(birthdate) {
		return 0
	}
	years := at.Year() - birthdate.Year()
	bdMonth, bdDay := birthdate.Month(), birthdate.Day()
	if bdMonth == time.February && bdDay == 29 && !isLeapYear(at.Year()) {
		bdDay = 28
	}
	if at.Month() < bdMonth || (at.Month() == bdMonth && at.Day() < bdDay) {
		years--
	}
	return years
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// TentScout is a youth attendee in a tent, with the data needed to apply the
// tenting rules.
type TentScout struct {
	ID        string
	Name      string // human-readable display name
	Gender    string
	Birthdate time.Time
}

// ViolationCode identifies the tenting rule a Violation belongs to, so callers
// can block an assignment or offer an override.
type ViolationCode string

const (
	ViolationNeverAlone  ViolationCode = "never_alone"
	ViolationMixedGender ViolationCode = "mixed_gender"
	ViolationAgeGap      ViolationCode = "age_gap"
)

// Violation is a structured, human-readable reason a tent arrangement is not
// allowed.
type Violation struct {
	Code    ViolationCode
	Message string
}

// ValidateTent applies the tenting rules to a tent of scouts on the event
// start date and returns a list of human-readable violations. The sibling
// pairs (from SiblingPairs) exempt pairs from the age gap rule; the gender
// rule always applies. An empty tent is allowed; a single scout is a
// never-alone violation.
func ValidateTent(scouts []TentScout, eventStart time.Time, maxAgeGap int, siblings map[ScoutPair]bool) []Violation {
	if len(scouts) == 1 {
		return []Violation{{
			Code:    ViolationNeverAlone,
			Message: fmt.Sprintf("%s would sleep alone; every tent needs at least 2 scouts", scouts[0].Name),
		}}
	}
	if len(scouts) == 0 {
		return nil
	}

	var violations []Violation

	genders := make(map[string]bool)
	for _, s := range scouts {
		genders[s.Gender] = true
	}
	if len(genders) > 1 {
		names := make([]string, len(scouts))
		for i, s := range scouts {
			names[i] = s.Name
		}
		violations = append(violations, Violation{
			Code:    ViolationMixedGender,
			Message: fmt.Sprintf("Mixed-gender tent: %s must share the same gender", strings.Join(names, ", ")),
		})
	}

	for i := 0; i < len(scouts); i++ {
		for j := i + 1; j < len(scouts); j++ {
			a, b := scouts[i], scouts[j]
			if siblings[normalizePair(a.ID, b.ID)] {
				continue
			}
			ageA, ageB := AgeWholeYears(a.Birthdate, eventStart), AgeWholeYears(b.Birthdate, eventStart)
			diff := ageA - ageB
			if diff < 0 {
				diff = -diff
			}
			if diff <= maxAgeGap {
				continue
			}
			older, younger := a, b
			if ageB > ageA {
				older, younger = b, a
			}
			violations = append(violations, Violation{
				Code:    ViolationAgeGap,
				Message: fmt.Sprintf("%s is %d years older than %s (max %d)", older.Name, diff, younger.Name, maxAgeGap),
			})
		}
	}

	if len(violations) == 0 {
		return nil
	}
	return violations
}
