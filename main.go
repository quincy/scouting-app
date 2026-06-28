package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"scout-app/internal/api"
	"scout-app/internal/config"
	"scout-app/internal/domain/appconfig"
	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/email"
	"scout-app/internal/domain/event"
	"scout-app/internal/domain/otpcode"
	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
	"scout-app/internal/domain/sync"
	"scout-app/internal/domain/user"
	appemail "scout-app/internal/email"
	"scout-app/internal/scoutbook"
	"scout-app/internal/storage"
	"scout-app/internal/storage/postgres"

	"github.com/gorilla/mux"
)

//go:embed static
var staticFS embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := storage.OpenDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("error closing database: %v", err)
		}
	}()

	sessionStore := auth.NewCookieStore(cfg.SessionSecret)

	// Repositories
	var (
		userRepo            user.Repository
		profileRepo         profile.Repository
		parentYouthLinkRepo parentyouthlink.Repository
		rbacRepo            rbac.Repository
		eventRepo           event.Repository
		otpRepo             otpcode.Repository
		appConfigRepo       appconfig.Repository
		emailSvc            email.Service
	)

	store := postgres.NewStore(db)
	userRepo = store.User
	profileRepo = store.Profile
	parentYouthLinkRepo = store.ParentYouthLink
	rbacRepo = store.RBAC
	eventRepo = store.Event
	otpRepo = postgres.NewOTPCodeRepository(db)
	appConfigRepo = store.AppConfig

	// Auth
	hasher := &auth.BCryptHasher{}
	authService := auth.NewAuthService(userRepo, rbacRepo, hasher, sessionStore)

	// Scoutbook sync
	scoutbookClient := scoutbook.NewClient(cfg.ScoutbookAPIBaseURL, cfg.ScoutbookToken, "")
	syncSvc := sync.NewService(profileRepo, rbacRepo, sync.NewScoutbookClientAdapter(scoutbookClient))
	syncHandler := api.NewSyncHandler(syncSvc, scoutbookClient, appConfigRepo)

	adminHandler := api.NewAdminHandler(profileRepo, parentYouthLinkRepo, rbacRepo, authService)

	if emailSvc == nil {
		emailTmpl, err := appemail.NewTemplates()
		if err != nil {
			log.Fatalf("Failed to load email templates: %v", err)
		}
		emailSvc = appemail.NewSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom, cfg.UnitType, cfg.UnitNumber, emailTmpl)
	}

	regHandler := api.NewRegistrationHandler(
		profileRepo, otpRepo, userRepo, rbacRepo, emailSvc, hasher, sessionStore,
	)

	familyConnectionsHandler := api.NewFamilyConnectionsHandler(profileRepo, parentYouthLinkRepo, authService, rbacRepo, emailSvc)

	onboardingHandler := api.NewOnboardingHandler(
		profileRepo, userRepo, rbacRepo, appConfigRepo, hasher, sessionStore,
	)

	router := mux.NewRouter()
	router.HandleFunc("/healthcheck", api.HealthCheckHandler).Methods("GET")
	router.HandleFunc("/deepcheck", api.DeepCheckHandler(db)).Methods("GET")

	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(staticFS)))

	// Onboarding routes (guarded by RedirectIfOnboarded)
	onboardRouter := router.PathPrefix("/onboard").Subrouter()
	onboardRouter.Use(func(next http.Handler) http.Handler {
		return api.RedirectIfOnboarded(appConfigRepo, next)
	})
	onboardRouter.HandleFunc("", onboardingHandler.WelcomePage).Methods("GET")
	onboardRouter.HandleFunc("/personal", onboardingHandler.PersonalPage).Methods("GET")
	onboardRouter.HandleFunc("/personal", onboardingHandler.Personal).Methods("POST")
	onboardRouter.HandleFunc("/unit", onboardingHandler.UnitPage).Methods("GET")
	onboardRouter.HandleFunc("/unit", onboardingHandler.Unit).Methods("POST")
	onboardRouter.HandleFunc("/timezone", onboardingHandler.TimezonePage).Methods("GET")
	onboardRouter.HandleFunc("/timezone", onboardingHandler.Timezone).Methods("POST")
	onboardRouter.HandleFunc("/password", onboardingHandler.PasswordPage).Methods("GET")
	onboardRouter.HandleFunc("/password", onboardingHandler.Password).Methods("POST")
	onboardRouter.HandleFunc("/complete", onboardingHandler.CompletePage).Methods("GET")

	app := router.PathPrefix("").Subrouter()
	app.Use(func(next http.Handler) http.Handler {
		return api.RequireOnboarding(appConfigRepo, next)
	})

	app.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/events", http.StatusFound)
	})

	authHandler := api.NewAuthHandler(authService)
	app.HandleFunc("/login", authHandler.LoginPage).Methods("GET")
	app.HandleFunc("/login", authHandler.Login).Methods("POST")
	app.HandleFunc("/logout", api.RequireAuth(authService, authHandler.Logout)).Methods("POST")

	app.HandleFunc("/register", regHandler.RegisterPage).Methods("GET")
	app.HandleFunc("/register", regHandler.Register).Methods("POST")
	app.HandleFunc("/register/verify", regHandler.VerifyPage).Methods("GET")
	app.HandleFunc("/register/verify", regHandler.Verify).Methods("POST")
	app.HandleFunc("/register/complete", regHandler.CompletePage).Methods("GET")
	app.HandleFunc("/register/complete", regHandler.Complete).Methods("POST")

	app.Handle("/family-connections", api.RequireAuth(authService, familyConnectionsHandler.FamilyConnectionsPage)).Methods("GET")
	app.Handle("/family-connections", api.RequireAuth(authService, familyConnectionsHandler.AddConnection)).Methods("POST")

	eventHandler := api.NewEventHandler(eventRepo, authService, rbacRepo, profileRepo, parentYouthLinkRepo, cfg.UnitType, cfg.UnitNumber)
	api.SetMuxVars(mux.Vars)
	app.Handle("/events", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListEvents)).Methods("GET")
	app.Handle("/events/upcoming", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListUpcoming)).Methods("GET")
	app.Handle("/events/past", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListPast)).Methods("GET")
	app.Handle("/events/create", api.RequirePermission(authService, rbacRepo, "event:create", eventHandler.EventCreateForm)).Methods("GET")
	app.Handle("/events/create", api.RequirePermission(authService, rbacRepo, "event:create", eventHandler.EventCreate)).Methods("POST")
	app.Handle("/events/{id}/edit", api.RequirePermission(authService, rbacRepo, "event:create", eventHandler.EventEditForm)).Methods("GET")
	app.Handle("/events/{id}/edit", api.RequirePermission(authService, rbacRepo, "event:create", eventHandler.EventEdit)).Methods("POST")
	app.Handle("/events/{id}/delete", api.RequirePermission(authService, rbacRepo, "event:create", eventHandler.EventDeleteConfirm)).Methods("GET")
	app.Handle("/events/{id}/delete", api.RequirePermission(authService, rbacRepo, "event:create", eventHandler.EventDelete)).Methods("DELETE")
	app.Handle("/events/{id}", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.EventDetail)).Methods("GET")
	app.Handle("/events/{id}/signup", api.RequirePermission(authService, rbacRepo, "event:signup", eventHandler.SignUp)).Methods("POST")
	app.Handle("/events/{id}/withdraw", api.RequirePermission(authService, rbacRepo, "event:withdraw", eventHandler.Withdraw)).Methods("POST")

	app.Handle("/admin", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.AdminPage)).Methods("GET")
	app.Handle("/admin/roster", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.RosterPage)).Methods("GET")
	app.Handle("/admin/connections", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.ConnectionsPage)).Methods("GET")
	app.Handle("/admin/connections/{id}/approve", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.ApproveConnection)).Methods("POST")
	app.Handle("/admin/connections/{id}/reject", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.RejectConnection)).Methods("POST")
	app.Handle("/admin/connections/{id}/remove", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.RemoveConnection)).Methods("POST")
	app.Handle("/admin/roles", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.RolesPage)).Methods("GET")
	app.Handle("/admin/roles/{id}/grant-admin", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.GrantAdmin)).Methods("POST")
	app.Handle("/admin/roles/{id}/remove-admin", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.RemoveAdmin)).Methods("POST")
	app.Handle("/admin/markdown-preview", api.RequirePermission(authService, rbacRepo, "event:create", eventHandler.MarkdownPreview)).Methods("POST")

	app.Handle("/admin/sync", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.AdminPage)).Methods("GET")
	app.Handle("/admin/sync/token", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.StoreToken)).Methods("POST")
	app.Handle("/admin/sync", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.Sync)).Methods("POST")
	app.Handle("/admin/sync/revert", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.Revert)).Methods("POST")

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			ctx := context.Background()
			if err := otpRepo.DeleteExpired(ctx); err != nil {
				log.Printf("OTP cleanup: %v", err)
			}
		}
	}()

	srv := &http.Server{
		Addr:    cfg.Addr,
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
