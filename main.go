package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"scout-app/internal/api"
	"scout-app/internal/domain"
	"scout-app/internal/storage"
	"scout-app/internal/storage/mock"

	"github.com/gorilla/mux"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	useMock := os.Getenv("USE_MOCK_STORAGE") == "true"

	var db *sql.DB
	var err error
	if !useMock {
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			log.Fatal("DATABASE_URL environment variable is required")
		}

		db, err = storage.OpenDB(databaseURL)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				log.Printf("error closing database: %v", err)
			}
		}()

		if os.Getenv("AUTO_MIGRATE") == "true" {
			log.Println("Running database migrations...")
			if err := storage.RunMigrations(db, migrations, "migrations"); err != nil {
				log.Fatalf("Migration failed: %v", err)
			}
			log.Println("Migrations complete")
		}
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if useMock && sessionSecret == "" {
		sessionSecret = "dev-secret-key"
	}
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET environment variable is required")
	}

	// Repositories
	eventRepo := mock.NewEventRepository()
	userRepo := mock.NewUserRepository()
	rbacRepo := mock.NewRBACRepository()

	// Auth
	hasher := &domain.BCryptHasher{}
	authService := domain.NewAuthService(userRepo, rbacRepo, hasher, sessionSecret)

	if useMock {
		ctx := context.Background()
		if err := rbacRepo.SeedRoles(ctx); err != nil {
			log.Fatalf("SeedRoles failed: %v", err)
		}
		if err := authService.SeedAdminUser(ctx); err != nil {
			log.Fatalf("SeedAdminUser failed: %v", err)
		}
		log.Println("Seeded admin user: admin@scout.local / password")
	}

	router := mux.NewRouter()
	router.HandleFunc("/healthcheck", api.HealthCheckHandler).Methods("GET")

	if !useMock {
		router.HandleFunc("/deepcheck", api.DeepCheckHandler(db)).Methods("GET")
	}

	authHandler := api.NewAuthHandler(authService)
	router.HandleFunc("/login", authHandler.LoginPage).Methods("GET")
	router.HandleFunc("/login", authHandler.Login).Methods("POST")
	router.HandleFunc("/logout", api.RequireAuth(authService, authHandler.Logout)).Methods("POST")

	eventHandler := api.NewEventHandler(eventRepo)
	router.Handle("/events", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListEvents)).Methods("GET")
	router.Handle("/events/upcoming", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListUpcoming)).Methods("GET")
	router.Handle("/events/past", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListPast)).Methods("GET")

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Server ListenAndServe: %v", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Waiting for SIGINT or SIGTERM")
	<-sigs
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server Shutdown: %v", err)
	}
	fmt.Println("Server gracefully stopped")
}
