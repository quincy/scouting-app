package scoutbook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAdults(t *testing.T) {
	ctx := t.Context()
	adults := []RosterMember{
		{MemberID: "100", FirstName: "John", LastName: "Doe", PersonGUID: "guid-adult-1"},
		{MemberID: "101", FirstName: "Jane", LastName: "Smith", PersonGUID: "guid-adult-2"},
	}
	server := newRosterTestServer(t, "/organizations/v2/org-123/orgAdults", adults)
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org-123")
	result, err := client.FetchRoster(ctx, EndpointAdults)
	if err != nil {
		t.Fatalf("FetchRoster failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 adults, got %d", len(result))
	}
	if result[0].MemberID != "100" || result[0].FirstName != "John" {
		t.Errorf("unexpected first member: %+v", result[0])
	}
}

func TestFetchYouths(t *testing.T) {
	ctx := t.Context()
	youths := []RosterMember{
		{MemberID: "200", FirstName: "Jimmy", LastName: "Jones", PersonGUID: "guid-youth-1"},
	}
	server := newRosterTestServer(t, "/organizations/v2/org-123/orgYouths", youths)
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org-123")
	result, err := client.FetchRoster(ctx, EndpointYouths)
	if err != nil {
		t.Fatalf("FetchRoster failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 youth, got %d", len(result))
	}
	if result[0].MemberID != "200" {
		t.Errorf("unexpected member: %+v", result[0])
	}
}

func TestFetchProfile(t *testing.T) {
	ctx := t.Context()
	profile := PersonProfile{
		Email:        "john@example.com",
		PrimaryPhone: "555-0100",
		BirthDate:    "1990-01-15",
	}
	server := newProfileTestServer(t, "/persons/v2/guid-adult-1/personprofile", profile)
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org-123")
	result, err := client.FetchProfile(ctx, "guid-adult-1")
	if err != nil {
		t.Fatalf("FetchProfile failed: %v", err)
	}

	if result.Email != "john@example.com" {
		t.Errorf("expected email john@example.com, got %s", result.Email)
	}
	if result.PrimaryPhone != "555-0100" {
		t.Errorf("expected phone 555-0100, got %s", result.PrimaryPhone)
	}
	if result.BirthDate != "1990-01-15" {
		t.Errorf("expected birthdate 1990-01-15, got %s", result.BirthDate)
	}
}

func TestFetchProfile_NilWhenNotFound(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org-123")
	result, err := client.FetchProfile(ctx, "guid-adult-1")
	if err != nil {
		t.Fatalf("FetchProfile failed: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil profile, got %+v", result)
	}
}

func TestClient_InvalidEndpoint(t *testing.T) {
	ctx := t.Context()
	client := NewClient("http://example.com", "test-token", "org-123")
	_, err := client.FetchRoster(ctx, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid endpoint, got nil")
	}
}

func TestClient_ServerError(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org-123")
	_, err := client.FetchRoster(ctx, EndpointAdults)
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func newRosterTestServer(t *testing.T, expectedPath string, members []RosterMember) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rosterResponse{Data: members})
	}))
}

func newProfileTestServer(t *testing.T, expectedPath string, profile PersonProfile) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}))
}
