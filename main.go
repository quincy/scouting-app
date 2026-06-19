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
	"scout-app/internal/storage/mock"
	"scout-app/internal/storage/postgres"

	"github.com/gorilla/mux"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var db *sql.DB
	if !cfg.UseMockStorage {
		if cfg.DatabaseURL == "" {
			log.Fatal("DATABASE_URL environment variable is required")
		}

		db, err = storage.OpenDB(cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				log.Printf("error closing database: %v", err)
			}
		}()

		if cfg.AutoMigrate {
			log.Println("Running database migrations...")
			if err := storage.RunMigrations(db, migrations, "migrations"); err != nil {
				log.Fatalf("Migration failed: %v", err)
			}
			log.Println("Migrations complete")
		}
	}

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

	if cfg.UseMockStorage {
		mockUserRepo := mock.NewUserRepository()
		mockProfileRepo := mock.NewProfileRepository()
		mockParentYouthLinkRepo := mock.NewParentYouthLinkRepository()
		mockRBACRepo := mock.NewRBACRepository()
		mockEventRepo := mock.NewEventRepository(mockProfileRepo)
		mockOTPRepo := mock.NewOTPCodeRepository()
		mockAppConfigRepo := mock.NewAppConfigRepository()
		mockEmailSvc := mock.NewEmailService()

		userRepo = mockUserRepo
		profileRepo = mockProfileRepo
		parentYouthLinkRepo = mockParentYouthLinkRepo
		rbacRepo = mockRBACRepo
		eventRepo = mockEventRepo
		otpRepo = mockOTPRepo
		appConfigRepo = mockAppConfigRepo
		emailSvc = mockEmailSvc

		ctx := context.Background()
		if err := mockRBACRepo.SeedRoles(ctx); err != nil {
			log.Fatalf("SeedRoles failed: %v", err)
		}

		hasher := &auth.MockHasher{}
		authService := auth.NewAuthService(userRepo, rbacRepo, hasher, sessionStore)

		if err := authService.SeedAdminUser(ctx); err != nil {
			log.Fatalf("SeedAdminUser failed: %v", err)
		}
		log.Println("Seeded admin user: admin@scout.local / password")

		adminUser, err := mockUserRepo.GetByEmail(ctx, "admin@scout.local")
		if err != nil {
			log.Fatalf("GetByEmail admin: %v", err)
		}
		adminProfile := &profile.Profile{
			FirstName:  "Admin",
			LastName:   "User",
			Email:      "admin@scout.local",
			MemberType: profile.MemberTypeAdult,
			Status:     profile.StatusActive,
			UserID:     &adminUser.ID,
		}
		if err := mockProfileRepo.Create(ctx, adminProfile); err != nil {
			log.Fatalf("Create admin profile: %v", err)
		}
		log.Println("Created admin profile")

		if err := mockAppConfigRepo.Set(ctx, appconfig.KeyOnboardingComplete, "true"); err != nil {
			log.Fatalf("Set onboarding complete: %v", err)
		}

		now := time.Now()
		seedEvents := []*event.Event{
			{Title: "Campout at Lake George", Description: "Weekend camping trip with swimming, hiking, and campfire stories.", Location: "Lake George", StartTime: now.Add(72 * time.Hour), EndTime: now.Add(96 * time.Hour), CostCents: 1500, Type: "campout"},
			{Title: "Knot-Tying Workshop", Description: "Learn essential scout knots including square, clove hitch, and bowline.", Location: "Scout Hall", StartTime: now.Add(720 * time.Hour), EndTime: now.Add(722 * time.Hour), CostCents: 0, Type: "campout"},
			{Title: "River Cleanup", Description: "Community service event to clean up the riverside trail.", Location: "River Park", StartTime: now.Add(-48 * time.Hour), EndTime: now.Add(-46 * time.Hour), CostCents: 0, Type: "campout"},
		}
		for _, e := range seedEvents {
			if err := mockEventRepo.Create(ctx, e); err != nil {
				log.Fatalf("Create event failed: %v", err)
			}
		}
		log.Println("Seeded 3 example events")

		if err := mockEventRepo.SignUp(ctx, seedEvents[0].ID, adminProfile.ID); err != nil {
			log.Fatalf("SignUp admin: %v", err)
		}
		log.Println("Signed up admin to Campout at Lake George")

		youthProfile := &profile.Profile{
			FirstName:  "Alex",
			LastName:   "Youth",
			Email:      "alex.youth@scout.local",
			MemberType: profile.MemberTypeYouth,
			Status:     profile.StatusActive,
		}
		if err := mockProfileRepo.Create(ctx, youthProfile); err != nil {
			log.Fatalf("Create youth profile: %v", err)
		}
		log.Println("Created youth profile: Alex Youth")

		link := &parentyouthlink.ParentYouthConnection{
			ParentProfileID: adminProfile.ID,
			YouthProfileID:  youthProfile.ID,
			Status:          parentyouthlink.StatusApproved,
		}
		if err := mockParentYouthLinkRepo.Create(ctx, link); err != nil {
			log.Fatalf("Create parent-youth link: %v", err)
		}
		log.Println("Linked Alex Youth to admin")

		if err := mockEventRepo.SignUp(ctx, seedEvents[0].ID, youthProfile.ID); err != nil {
			log.Fatalf("SignUp youth: %v", err)
		}
		log.Println("Signed up Alex Youth to Campout at Lake George")

		youthProfile2 := &profile.Profile{
			FirstName:  "Bailey",
			LastName:   "Scout",
			Email:      "bailey.scout@scout.local",
			MemberType: profile.MemberTypeYouth,
			Status:     profile.StatusActive,
		}
		if err := mockProfileRepo.Create(ctx, youthProfile2); err != nil {
			log.Fatalf("Create youth profile 2: %v", err)
		}
		link2 := &parentyouthlink.ParentYouthConnection{
			ParentProfileID: adminProfile.ID,
			YouthProfileID:  youthProfile2.ID,
			Status:          parentyouthlink.StatusApproved,
		}
		if err := mockParentYouthLinkRepo.Create(ctx, link2); err != nil {
			log.Fatalf("Create parent-youth link 2: %v", err)
		}
		log.Println("Created and linked Bailey Scout (not signed up)")
		log.Println("Login as admin@scout.local / password to manage Alex Youth and Bailey Scout via linked profiles")
	} else {
		store := postgres.NewStore(db)
		userRepo = store.User
		profileRepo = store.Profile
		parentYouthLinkRepo = store.ParentYouthLink
		rbacRepo = store.RBAC
		eventRepo = store.Event
		otpRepo = postgres.NewOTPCodeRepository(db)
		appConfigRepo = store.AppConfig

		if cfg.SeedDevData {
			ctx := context.Background()

			onboarded, err := appConfigRepo.Get(ctx, appconfig.KeyOnboardingComplete)
			if err != nil || onboarded != "true" {
				log.Println("Onboarding not complete, skipping dev data seed")
			} else {
				now := time.Now()
				seedEvents := []*event.Event{
					{Title: "Campout at Lake George", Description: "Weekend camping trip with swimming, hiking, and campfire stories.", Location: "Lake George", StartTime: now.Add(72 * time.Hour), EndTime: now.Add(96 * time.Hour), CostCents: 1500, Type: "campout"},
					{Title: "Knot-Tying Workshop", Description: "Learn essential scout knots including square, clove hitch, and bowline.", Location: "Scout Hall", StartTime: now.Add(720 * time.Hour), EndTime: now.Add(722 * time.Hour), CostCents: 0, Type: "campout"},
					{Title: "River Cleanup", Description: "Community service event to clean up the riverside trail.", Location: "River Park", StartTime: now.Add(-48 * time.Hour), EndTime: now.Add(-46 * time.Hour), CostCents: 0, Type: "campout"},
				}
				for _, e := range seedEvents {
					if err := eventRepo.Create(ctx, e); err != nil {
						log.Fatalf("Create event failed: %v", err)
					}
				}
				log.Println("Seeded 3 example events")
			}
		}
	}

	// Auth
	hasher := &auth.BCryptHasher{}
	authService := auth.NewAuthService(userRepo, rbacRepo, hasher, sessionStore)

	// Scoutbook sync
	scoutbookClient := scoutbook.NewClient(cfg.ScoutbookAPIBaseURL, cfg.ScoutbookToken, "")
	syncSvc := sync.NewService(profileRepo, rbacRepo, sync.NewScoutbookClientAdapter(scoutbookClient))
	syncHandler := api.NewSyncHandler(syncSvc, scoutbookClient, appConfigRepo)

	adminHandler := api.NewAdminHandler(profileRepo, parentYouthLinkRepo, authService)

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

	familyConnectionsHandler := api.NewFamilyConnectionsHandler(profileRepo, parentYouthLinkRepo, authService, rbacRepo)

	onboardingHandler := api.NewOnboardingHandler(
		profileRepo, userRepo, rbacRepo, appConfigRepo, hasher, sessionStore,
	)

	router := mux.NewRouter()
	router.HandleFunc("/healthcheck", api.HealthCheckHandler).Methods("GET")

	if !cfg.UseMockStorage {
		router.HandleFunc("/deepcheck", api.DeepCheckHandler(db)).Methods("GET")
	}

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

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
	app.Handle("/events/{id}", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.EventDetail)).Methods("GET")
	app.Handle("/events/{id}/signup", api.RequirePermission(authService, rbacRepo, "event:signup", eventHandler.SignUp)).Methods("POST")
	app.Handle("/events/{id}/withdraw", api.RequirePermission(authService, rbacRepo, "event:withdraw", eventHandler.Withdraw)).Methods("POST")

	app.Handle("/admin", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.AdminPage)).Methods("GET")
	app.Handle("/admin/roster", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.RosterPage)).Methods("GET")
	app.Handle("/admin/connections", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.ConnectionsPage)).Methods("GET")
	app.Handle("/admin/connections/{id}/approve", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.ApproveConnection)).Methods("POST")
	app.Handle("/admin/connections/{id}/reject", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.RejectConnection)).Methods("POST")
	app.Handle("/admin/connections/{id}/remove", api.RequirePermission(authService, rbacRepo, "event:create", adminHandler.RemoveConnection)).Methods("POST")
	app.Handle("/admin/markdown-preview", api.RequirePermission(authService, rbacRepo, "event:create", eventHandler.MarkdownPreview)).Methods("POST")

	app.Handle("/admin/sync", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.AdminPage)).Methods("GET")
	app.Handle("/admin/sync/token", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.StoreToken)).Methods("POST")
	app.Handle("/admin/sync", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.Sync)).Methods("POST")
	app.Handle("/admin/sync/revert", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.Revert)).Methods("POST")

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx := context.Background()
				if err := otpRepo.DeleteExpired(ctx); err != nil {
					log.Printf("OTP cleanup: %v", err)
				}
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
