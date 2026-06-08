package profile

import "testing"

func TestDisplayName_WithNickname(t *testing.T) {
	p := &Profile{
		FirstName: "Robert",
		LastName:  "Smith",
		Nickname:  "Bob",
	}
	want := "Bob (Robert) Smith"
	if got := p.DisplayName(); got != want {
		t.Errorf("DisplayName() = %q, want %q", got, want)
	}
}

func TestDisplayName_WithoutNickname(t *testing.T) {
	p := &Profile{
		FirstName: "John",
		LastName:  "Doe",
	}
	want := "John Doe"
	if got := p.DisplayName(); got != want {
		t.Errorf("DisplayName() = %q, want %q", got, want)
	}
}

func TestDisplayName_EmptyNickname(t *testing.T) {
	p := &Profile{
		FirstName: "Jane",
		LastName:  "Doe",
		Nickname:  "",
	}
	want := "Jane Doe"
	if got := p.DisplayName(); got != want {
		t.Errorf("DisplayName() = %q, want %q", got, want)
	}
}
