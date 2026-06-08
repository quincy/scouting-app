package scoutbook

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAdults(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/organizations/v2/units/org-123/adults" {
			t.Errorf("expected path /organizations/v2/units/org-123/adults, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"users":[
			{"memberId":100,"firstName":"John","lastName":"Doe","nickName":"Johnny","gender":"M","personGuid":"guid-adult-1","email":"john@example.com","homePhone":"555-0100","dateOfBirth":"1990-01-15","isAdult":true,"positions":[{"position":"Scoutmaster"},{"position":"Troop Admin"}]},
			{"memberId":101,"firstName":"Jane","lastName":"Smith","personGuid":"guid-adult-2","email":"jane@example.com","isAdult":true}
		]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org-123")
	result, err := client.FetchRoster(ctx, EndpointUnitAdults)
	if err != nil {
		t.Fatalf("FetchRoster failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 adults, got %d", len(result))
	}
	if result[0].MemberID != "100" || result[0].FirstName != "John" {
		t.Errorf("unexpected first member: %+v", result[0])
	}
	if result[0].Email != "john@example.com" {
		t.Errorf("expected email, got %s", result[0].Email)
	}
	if result[0].HomePhone != "555-0100" {
		t.Errorf("expected home phone, got %s", result[0].HomePhone)
	}
	if result[0].BirthDate != "1990-01-15" {
		t.Errorf("expected birthdate, got %s", result[0].BirthDate)
	}
	if !result[0].IsAdult {
		t.Errorf("expected IsAdult=true")
	}
	if result[0].NickName != "Johnny" {
		t.Errorf("expected nickName Johnny, got %s", result[0].NickName)
	}
	if result[0].Gender != "M" {
		t.Errorf("expected gender M, got %s", result[0].Gender)
	}
	if result[0].Positions != "Scoutmaster, Troop Admin" {
		t.Errorf("expected positions, got %s", result[0].Positions)
	}
}

func TestFetchYouths(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/organizations/v2/units/org-123/youths" {
			t.Errorf("expected path /organizations/v2/units/org-123/youths, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"users":[
			{"memberId":200,"firstName":"Jimmy","lastName":"Jones","personGuid":"guid-youth-1","isAdult":false}
		]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "org-123")
	result, err := client.FetchRoster(ctx, EndpointUnitYouths)
	if err != nil {
		t.Fatalf("FetchRoster failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 youth, got %d", len(result))
	}
	if result[0].MemberID != "200" {
		t.Errorf("unexpected member: %+v", result[0])
	}
	if result[0].IsAdult {
		t.Errorf("expected IsAdult=false for youth")
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
	_, err := client.FetchRoster(ctx, EndpointUnitAdults)
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}
