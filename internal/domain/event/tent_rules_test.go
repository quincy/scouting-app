package event

import (
	"reflect"
	"testing"
	"time"
)

var testEventStart = time.Date(2026, time.June, 20, 9, 0, 0, 0, time.UTC)

func idate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func violationByCode(t *testing.T, violations []Violation, code ViolationCode) (string, bool) {
	t.Helper()
	for _, v := range violations {
		if v.Code == code {
			return v.Message, true
		}
	}
	return "", false
}

func TestValidateTent_NeverAlone(t *testing.T) {
	solo := []TentScout{{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2012, time.January, 1)}}

	t.Run("single scout is a violation", func(t *testing.T) {
		violations := ValidateTent(solo, testEventStart, 2, nil)
		if len(violations) != 1 {
			t.Fatalf("expected 1 violation, got %d: %v", len(violations), violations)
		}
		if violations[0].Code != ViolationNeverAlone {
			t.Errorf("expected never-alone violation, got %q", violations[0].Code)
		}
		if violations[0].Message != "Nate would sleep alone; every tent needs at least 2 scouts" {
			t.Errorf("unexpected message: %q", violations[0].Message)
		}
	})

	t.Run("empty tent is not a violation", func(t *testing.T) {
		if violations := ValidateTent(nil, testEventStart, 2, nil); len(violations) != 0 {
			t.Errorf("expected no violations for an empty tent, got %v", violations)
		}
	})
}

func TestValidateTent_Allowed(t *testing.T) {
	scouts := []TentScout{
		{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2012, time.January, 1)},
		{ID: "e1", Name: "Eli", Gender: "M", Birthdate: idate(2014, time.January, 1)},
	}

	if violations := ValidateTent(scouts, testEventStart, 2, nil); len(violations) != 0 {
		t.Errorf("expected no violations, got %v", violations)
	}

	t.Run("age gap exactly at the max is allowed", func(t *testing.T) {
		scouts := []TentScout{
			{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2010, time.January, 1)},
			{ID: "e1", Name: "Eli", Gender: "M", Birthdate: idate(2012, time.January, 1)},
		}
		if violations := ValidateTent(scouts, testEventStart, 2, nil); len(violations) != 0 {
			t.Errorf("expected no violations when diff equals max, got %v", violations)
		}
	})
}

func TestValidateTent_AgeGap(t *testing.T) {
	scouts := []TentScout{
		{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2010, time.January, 1)},
		{ID: "e1", Name: "Eli", Gender: "M", Birthdate: idate(2014, time.January, 1)},
	}

	violations := ValidateTent(scouts, testEventStart, 2, nil)
	msg, ok := violationByCode(t, violations, ViolationAgeGap)
	if !ok {
		t.Fatalf("expected age gap violation, got %v", violations)
	}
	if msg != "Nate is 4 years older than Eli (max 2)" {
		t.Errorf("unexpected message: %q", msg)
	}
}

func TestValidateTent_ZeroAgeGap(t *testing.T) {
	equalAges := []TentScout{
		{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2012, time.January, 1)},
		{ID: "e1", Name: "Eli", Gender: "M", Birthdate: idate(2012, time.January, 1)},
	}
	t.Run("same-age scouts allowed with max gap 0", func(t *testing.T) {
		if violations := ValidateTent(equalAges, testEventStart, 0, nil); len(violations) != 0 {
			t.Errorf("expected no violations, got %v", violations)
		}
	})
	t.Run("any age difference violates max gap 0", func(t *testing.T) {
		scouts := []TentScout{
			{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2011, time.January, 1)},
			{ID: "e1", Name: "Eli", Gender: "M", Birthdate: idate(2012, time.January, 1)},
		}
		violations := ValidateTent(scouts, testEventStart, 0, nil)
		if _, ok := violationByCode(t, violations, ViolationAgeGap); !ok {
			t.Errorf("expected age gap violation, got %v", violations)
		}
	})
}

func TestValidateTent_MixedGender(t *testing.T) {
	scouts := []TentScout{
		{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2010, time.January, 1)},
		{ID: "e1", Name: "Eli", Gender: "F", Birthdate: idate(2012, time.January, 1)},
	}

	violations := ValidateTent(scouts, testEventStart, 2, nil)
	msg, ok := violationByCode(t, violations, ViolationMixedGender)
	if !ok {
		t.Fatalf("expected mixed gender violation, got %v", violations)
	}
	if msg != "Mixed-gender tent: Nate, Eli must share the same gender" {
		t.Errorf("unexpected message: %q", msg)
	}

	t.Run("gender and age gap violations both reported", func(t *testing.T) {
		scouts := []TentScout{
			{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2010, time.January, 1)},
			{ID: "e1", Name: "Eli", Gender: "F", Birthdate: idate(2014, time.January, 1)},
		}
		violations := ValidateTent(scouts, testEventStart, 2, nil)
		if len(violations) != 2 {
			t.Fatalf("expected 2 violations, got %d: %v", len(violations), violations)
		}
		if _, ok := violationByCode(t, violations, ViolationMixedGender); !ok {
			t.Errorf("expected mixed gender violation in %v", violations)
		}
		if _, ok := violationByCode(t, violations, ViolationAgeGap); !ok {
			t.Errorf("expected age gap violation in %v", violations)
		}
	})

	t.Run("three distinct genders produce a single gender violation", func(t *testing.T) {
		scouts := []TentScout{
			{ID: "n1", Name: "Nate", Gender: "M", Birthdate: idate(2012, time.January, 1)},
			{ID: "e1", Name: "Eli", Gender: "F", Birthdate: idate(2012, time.January, 1)},
			{ID: "x1", Name: "Xan", Gender: "X", Birthdate: idate(2012, time.January, 1)},
		}
		violations := ValidateTent(scouts, testEventStart, 2, nil)
		if len(violations) != 1 {
			t.Fatalf("expected exactly 1 gender violation, got %d: %v", len(violations), violations)
		}
		msg, _ := violationByCode(t, violations, ViolationMixedGender)
		if msg != "Mixed-gender tent: Nate, Eli, Xan must share the same gender" {
			t.Errorf("unexpected message: %q", msg)
		}
	})
}

func TestValidateTent_SiblingExemption(t *testing.T) {
	siblings := SiblingPairs([]ApprovedParentLink{
		{ParentProfileID: "mom", YouthProfileID: "s1"},
		{ParentProfileID: "mom", YouthProfileID: "s2"},
	})

	t.Run("siblings exempt from the age gap rule", func(t *testing.T) {
		scouts := []TentScout{
			{ID: "s1", Name: "Sam", Gender: "M", Birthdate: idate(2005, time.January, 1)},
			{ID: "s2", Name: "Sue", Gender: "M", Birthdate: idate(2012, time.January, 1)},
		}
		if violations := ValidateTent(scouts, testEventStart, 2, siblings); len(violations) != 0 {
			t.Errorf("expected no violations for siblings, got %v", violations)
		}
	})

	t.Run("siblings still must share a gender", func(t *testing.T) {
		scouts := []TentScout{
			{ID: "s1", Name: "Sam", Gender: "M", Birthdate: idate(2005, time.January, 1)},
			{ID: "s2", Name: "Sue", Gender: "F", Birthdate: idate(2012, time.January, 1)},
		}
		violations := ValidateTent(scouts, testEventStart, 2, siblings)
		if _, ok := violationByCode(t, violations, ViolationMixedGender); !ok {
			t.Errorf("expected mixed gender violation for siblings, got %v", violations)
		}
		if _, ok := violationByCode(t, violations, ViolationAgeGap); ok {
			t.Errorf("did not expect age gap violation for siblings, got %v", violations)
		}
	})

	t.Run("non-sibling pairs in the tent still checked", func(t *testing.T) {
		scouts := []TentScout{
			{ID: "s1", Name: "Sam", Gender: "M", Birthdate: idate(2005, time.January, 1)},
			{ID: "s2", Name: "Sue", Gender: "M", Birthdate: idate(2012, time.January, 1)},
			{ID: "x1", Name: "Xan", Gender: "M", Birthdate: idate(2008, time.January, 1)},
		}
		violations := ValidateTent(scouts, testEventStart, 2, siblings)
		if len(violations) != 2 {
			t.Fatalf("expected 2 age gap violations, got %d: %v", len(violations), violations)
		}
		for _, want := range []string{
			"Sam is 3 years older than Xan (max 2)",
			"Xan is 4 years older than Sue (max 2)",
		} {
			if !containsMessage(violations, want) {
				t.Errorf("expected violation %q in %v", want, violations)
			}
		}
	})
}

func TestValidateTent_PairwiseAcrossAllScouts(t *testing.T) {
	scouts := []TentScout{
		{ID: "a1", Name: "Al", Gender: "M", Birthdate: idate(2008, time.January, 1)},
		{ID: "b1", Name: "Bo", Gender: "M", Birthdate: idate(2007, time.January, 1)},
		{ID: "c1", Name: "Cy", Gender: "M", Birthdate: idate(2005, time.January, 1)},
	}

	violations := ValidateTent(scouts, testEventStart, 2, nil)
	if len(violations) != 1 {
		t.Fatalf("expected 1 age gap violation, got %d: %v", len(violations), violations)
	}
	if msg, _ := violationByCode(t, violations, ViolationAgeGap); msg != "Cy is 3 years older than Al (max 2)" {
		t.Errorf("unexpected message: %q", msg)
	}
}

func containsMessage(violations []Violation, want string) bool {
	for _, v := range violations {
		if v.Message == want {
			return true
		}
	}
	return false
}

func TestSiblingPairs(t *testing.T) {
	t.Run("scouts sharing one approved parent are siblings", func(t *testing.T) {
		links := []ApprovedParentLink{
			{ParentProfileID: "mom", YouthProfileID: "a"},
			{ParentProfileID: "mom", YouthProfileID: "b"},
		}
		pairs := SiblingPairs(links)
		if !pairs[ScoutPair{A: "a", B: "b"}] {
			t.Errorf("expected (a, b) to be siblings, got pairs %v", pairs)
		}
	})

	t.Run("sibling pair keys are normalized by scout id order", func(t *testing.T) {
		links := []ApprovedParentLink{
			{ParentProfileID: "mom", YouthProfileID: "b"},
			{ParentProfileID: "mom", YouthProfileID: "a"},
		}
		pairs := SiblingPairs(links)
		if !pairs[ScoutPair{A: "a", B: "b"}] {
			t.Errorf("expected normalized (a, b) pair, got pairs %v", pairs)
		}
	})

	t.Run("scouts sharing two parents are still one pair", func(t *testing.T) {
		links := []ApprovedParentLink{
			{ParentProfileID: "mom", YouthProfileID: "a"},
			{ParentProfileID: "mom", YouthProfileID: "b"},
			{ParentProfileID: "dad", YouthProfileID: "a"},
			{ParentProfileID: "dad", YouthProfileID: "b"},
		}
		pairs := SiblingPairs(links)
		if len(pairs) != 1 {
			t.Errorf("expected exactly 1 sibling pair, got %d: %v", len(pairs), pairs)
		}
	})

	t.Run("scouts with different parents are not siblings", func(t *testing.T) {
		links := []ApprovedParentLink{
			{ParentProfileID: "mom-a", YouthProfileID: "a"},
			{ParentProfileID: "mom-b", YouthProfileID: "b"},
		}
		pairs := SiblingPairs(links)
		if len(pairs) != 0 {
			t.Errorf("expected no sibling pairs, got %v", pairs)
		}
	})

	t.Run("parent with a single child forms no pair", func(t *testing.T) {
		links := []ApprovedParentLink{
			{ParentProfileID: "mom", YouthProfileID: "a"},
		}
		pairs := SiblingPairs(links)
		if len(pairs) != 0 {
			t.Errorf("expected no sibling pairs, got %v", pairs)
		}
	})

	t.Run("three siblings produce all pairs", func(t *testing.T) {
		links := []ApprovedParentLink{
			{ParentProfileID: "mom", YouthProfileID: "a"},
			{ParentProfileID: "mom", YouthProfileID: "b"},
			{ParentProfileID: "mom", YouthProfileID: "c"},
		}
		pairs := SiblingPairs(links)
		want := []ScoutPair{
			{A: "a", B: "b"},
			{A: "a", B: "c"},
			{A: "b", B: "c"},
		}
		if len(pairs) != len(want) {
			t.Errorf("expected %d pairs, got %d: %v", len(want), len(pairs), pairs)
		}
		for _, wp := range want {
			if !pairs[ScoutPair{A: wp.A, B: wp.B}] {
				t.Errorf("expected pair %v to be present, got %v", wp, pairs)
			}
		}
	})

	t.Run("empty links form no pairs", func(t *testing.T) {
		if pairs := SiblingPairs(nil); len(pairs) != 0 {
			t.Errorf("expected no pairs for empty links, got %v", pairs)
		}
	})

	t.Run("no pairs are formed between a parent and their own children only", func(t *testing.T) {
		links := []ApprovedParentLink{
			{ParentProfileID: "x", YouthProfileID: "a"},
			{ParentProfileID: "x", YouthProfileID: "b"},
			{ParentProfileID: "y", YouthProfileID: "b"},
			{ParentProfileID: "y", YouthProfileID: "c"},
		}
		pairs := SiblingPairs(links)
		want := map[ScoutPair]bool{
			{A: "a", B: "b"}: true,
			{A: "b", B: "c"}: true,
		}
		if !reflect.DeepEqual(pairs, want) {
			t.Errorf("expected %v, got %v", want, pairs)
		}
	})
}

func TestAgeWholeYears(t *testing.T) {
	eventStart := time.Date(2026, time.June, 20, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		birthdate time.Time
		at        time.Time
		want      int
	}{
		{
			name:      "birthday already passed this year",
			birthdate: time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      16,
		},
		{
			name:      "birthday later this year",
			birthdate: time.Date(2010, time.December, 1, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      15,
		},
		{
			name:      "birthday is today",
			birthdate: time.Date(2010, time.June, 20, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      16,
		},
		{
			name:      "day before birthday",
			birthdate: time.Date(2010, time.June, 21, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      15,
		},
		{
			name:      "same month day after",
			birthdate: time.Date(2010, time.June, 19, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      16,
		},
		{
			name:      "birthday time of day does not matter",
			birthdate: time.Date(2010, time.June, 20, 23, 59, 0, 0, time.UTC),
			at:        time.Date(2026, time.June, 20, 1, 0, 0, 0, time.UTC),
			want:      16,
		},
		{
			name:      "leap year birthday on feb 28 in non-leap year",
			birthdate: time.Date(2012, time.February, 29, 0, 0, 0, 0, time.UTC),
			at:        time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC),
			want:      14,
		},
		{
			name:      "leap year birthday on feb 27 in non-leap year",
			birthdate: time.Date(2012, time.February, 29, 0, 0, 0, 0, time.UTC),
			at:        time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC),
			want:      13,
		},
		{
			name:      "leap year birthday on feb 29 in leap year",
			birthdate: time.Date(2012, time.February, 29, 0, 0, 0, 0, time.UTC),
			at:        time.Date(2028, time.February, 29, 12, 0, 0, 0, time.UTC),
			want:      16,
		},
		{
			name:      "birthdate after reference date",
			birthdate: time.Date(2030, time.March, 1, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      0,
		},
		{
			name:      "same day birth",
			birthdate: time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      0,
		},
		{
			name:      "one year not yet completed",
			birthdate: time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      0,
		},
		{
			name:      "one year exactly",
			birthdate: time.Date(2025, time.June, 20, 0, 0, 0, 0, time.UTC),
			at:        eventStart,
			want:      1,
		},
		{
			name:      "leap year birthday not yet reached in a leap year",
			birthdate: time.Date(2012, time.February, 29, 0, 0, 0, 0, time.UTC),
			at:        time.Date(2024, time.February, 28, 12, 0, 0, 0, time.UTC),
			want:      11,
		},
		{
			name:      "leap year birthday reached on feb 29 of a leap year",
			birthdate: time.Date(2012, time.February, 29, 0, 0, 0, 0, time.UTC),
			at:        time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC),
			want:      12,
		},
		{
			name:      "century non-leap year birthday on feb 28",
			birthdate: time.Date(1996, time.February, 29, 0, 0, 0, 0, time.UTC),
			at:        time.Date(2100, time.February, 28, 12, 0, 0, 0, time.UTC),
			want:      104,
		},
		{
			name:      "century non-leap year birthday on feb 27",
			birthdate: time.Date(1996, time.February, 29, 0, 0, 0, 0, time.UTC),
			at:        time.Date(2100, time.February, 27, 12, 0, 0, 0, time.UTC),
			want:      103,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AgeWholeYears(tc.birthdate, tc.at); got != tc.want {
				t.Errorf("AgeWholeYears(%s, %s) = %d, want %d", tc.birthdate, tc.at, got, tc.want)
			}
		})
	}
}
