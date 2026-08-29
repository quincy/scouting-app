package event

import "testing"

func TestCookingPatrolNextName(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "Cooking 1"},
		{2, "Cooking 2"},
		{10, "Cooking 10"},
	}
	for _, tc := range cases {
		if got := CookingPatrolNextName(tc.n); got != tc.want {
			t.Errorf("CookingPatrolNextName(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestCookingPatrolNumber(t *testing.T) {
	cases := []struct {
		name string
		want int
		ok   bool
	}{
		{"Cooking 1", 1, true},
		{"Cooking 10", 10, true},
		{"Adults", 0, false},
		{"", 0, false},
		{"Cooking", 0, false},
		{"Cooking x", 0, false},
	}
	for _, tc := range cases {
		got, ok := CookingPatrolNumber(tc.name)
		if got != tc.want || ok != tc.ok {
			t.Errorf("CookingPatrolNumber(%q) = (%d, %v), want (%d, %v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
