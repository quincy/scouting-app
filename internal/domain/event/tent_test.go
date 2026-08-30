package event

import "testing"

func TestTentNextName(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "Tent 1"},
		{2, "Tent 2"},
		{10, "Tent 10"},
	}
	for _, tc := range cases {
		if got := TentNextName(tc.n); got != tc.want {
			t.Errorf("TentNextName(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestTentNumber(t *testing.T) {
	cases := []struct {
		name string
		want int
		ok   bool
	}{
		{"Tent 1", 1, true},
		{"Tent 10", 10, true},
		{"", 0, false},
		{"Tent", 0, false},
		{"Tent x", 0, false},
		{"Cooking 5", 0, false},
	}
	for _, tc := range cases {
		got, ok := TentNumber(tc.name)
		if got != tc.want || ok != tc.ok {
			t.Errorf("TentNumber(%q) = (%d, %v), want (%d, %v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
