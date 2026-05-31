package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type DependencyCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DeepCheckResponse struct {
	Status       string            `json:"status"`
	Dependencies []DependencyCheck `json:"dependencies"`
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := fmt.Fprintf(w, "OK"); err != nil {
		log.Printf("health check write failed: %v", err)
	}
}

func DeepCheckHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		deps := []DependencyCheck{
			checkDB(ctx, db),
		}

		resp := DeepCheckResponse{Status: "healthy", Dependencies: deps}
		for _, d := range deps {
			if d.Status != "healthy" {
				resp.Status = "unhealthy"
				break
			}
		}

		w.Header().Set("Content-Type", "application/json")
		status := http.StatusOK
		if resp.Status == "unhealthy" {
			status = http.StatusServiceUnavailable
		}
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("deep check write failed: %v", err)
		}
	}
}

func checkDB(ctx context.Context, db *sql.DB) DependencyCheck {
	if err := db.PingContext(ctx); err != nil {
		return DependencyCheck{Name: "database", Status: "unhealthy", Error: err.Error()}
	}
	return DependencyCheck{Name: "database", Status: "healthy"}
}
