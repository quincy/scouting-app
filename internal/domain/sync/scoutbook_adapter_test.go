package sync

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"scout-app/internal/scoutbook"
)

func TestScoutbookAdapter_ErrorPropagation(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewScoutbookClientAdapter(scoutbook.NewClient(server.URL, "token", "org-123"))
	_, err := client.FetchRoster(ctx, EndpointAdults)
	if err == nil {
		t.Fatal("expected error from server, got nil")
	}
}

func TestScoutbookAdapter_MobilePhoneFallback(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"users":[
			{"memberId":100,"firstName":"Jane","lastName":"Doe","personGuid":"guid-3","homePhone":"","mobilePhone":"555-9999"}
		]}`)
	}))
	defer server.Close()

	client := NewScoutbookClientAdapter(scoutbook.NewClient(server.URL, "token", "org-123"))
	members, err := client.FetchRoster(ctx, EndpointAdults)
	if err != nil {
		t.Fatalf("FetchRoster failed: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Phone != "555-9999" {
		t.Errorf("expected Phone 555-9999 (MobilePhone fallback), got %q", members[0].Phone)
	}
}

func TestScoutbookAdapter_UnknownMemberType(t *testing.T) {
	ctx := t.Context()
	client := NewScoutbookClientAdapter(scoutbook.NewClient("http://example.com", "token", "org-123"))
	members, err := client.FetchRoster(ctx, "unknown")
	if err != nil {
		t.Fatalf("FetchRoster for unknown type: %v", err)
	}
	if members != nil {
		t.Errorf("expected nil members for unknown type, got %v", members)
	}
}

func TestScoutbookAdapter_FetchYouths(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations/v2/units/org-123/youths" {
			t.Errorf("expected path /organizations/v2/units/org-123/youths, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"users":[
			{"memberId":200,"firstName":"Jimmy","lastName":"Jones","personGuid":"guid-2"}
		]}`)
	}))
	defer server.Close()

	client := NewScoutbookClientAdapter(scoutbook.NewClient(server.URL, "token", "org-123"))
	members, err := client.FetchRoster(ctx, EndpointYouths)
	if err != nil {
		t.Fatalf("FetchRoster failed: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].MemberID != "200" {
		t.Errorf("expected MemberID 200, got %q", members[0].MemberID)
	}
}

func TestScoutbookAdapter_FetchAdults(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations/v2/units/org-123/adults" {
			t.Errorf("expected path /organizations/v2/units/org-123/adults, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"users":[
			{"memberId":100,"firstName":"John","lastName":"Doe","nickName":"Johnny","gender":"M","personGuid":"guid-1","email":"john@example.com","homePhone":"555-0100","dateOfBirth":"1990-01-15","isAdult":true,"positions":[{"position":"Scoutmaster"}]}
		]}`)
	}))
	defer server.Close()

	client := NewScoutbookClientAdapter(scoutbook.NewClient(server.URL, "token", "org-123"))
	members, err := client.FetchRoster(ctx, EndpointAdults)
	if err != nil {
		t.Fatalf("FetchRoster failed: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	m := members[0]
	if m.MemberID != "100" {
		t.Errorf("expected MemberID 100, got %q", m.MemberID)
	}
	if m.FirstName != "John" {
		t.Errorf("expected FirstName John, got %q", m.FirstName)
	}
	if m.LastName != "Doe" {
		t.Errorf("expected LastName Doe, got %q", m.LastName)
	}
	if m.Nickname != "Johnny" {
		t.Errorf("expected Nickname Johnny, got %q", m.Nickname)
	}
	if m.Gender != "M" {
		t.Errorf("expected Gender M, got %q", m.Gender)
	}
	if m.PersonGUID != "guid-1" {
		t.Errorf("expected PersonGUID guid-1, got %q", m.PersonGUID)
	}
	if m.Email != "john@example.com" {
		t.Errorf("expected Email john@example.com, got %q", m.Email)
	}
	if m.Phone != "555-0100" {
		t.Errorf("expected Phone 555-0100 (HomePhone), got %q", m.Phone)
	}
	if m.BirthDate != "1990-01-15" {
		t.Errorf("expected BirthDate 1990-01-15, got %q", m.BirthDate)
	}
	if m.Positions != "Scoutmaster" {
		t.Errorf("expected Positions Scoutmaster, got %q", m.Positions)
	}
}
