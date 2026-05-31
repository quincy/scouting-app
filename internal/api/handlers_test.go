package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestHealthCheckHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/healthcheck", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	HealthCheckHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "OK"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestDeepCheckHandler_UnhealthyDB(t *testing.T) {
	badDB, err := sql.Open("pgx", "postgres://invalid:invalid@127.0.0.1:1/bad")
	if err != nil {
		t.Fatal(err)
	}
	defer badDB.Close()

	req := httptest.NewRequest("GET", "/deepcheck", nil)
	rr := httptest.NewRecorder()
	DeepCheckHandler(badDB)(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}

	var resp DeepCheckResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "unhealthy" {
		t.Errorf("expected overall status unhealthy, got %s", resp.Status)
	}
	if len(resp.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(resp.Dependencies))
	}
	if resp.Dependencies[0].Name != "database" {
		t.Errorf("expected dependency name 'database', got %s", resp.Dependencies[0].Name)
	}
	if resp.Dependencies[0].Status != "unhealthy" {
		t.Errorf("expected DB status unhealthy, got %s", resp.Dependencies[0].Status)
	}
	if resp.Dependencies[0].Error == "" {
		t.Error("expected error message in response")
	}
}
