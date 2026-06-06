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
	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/event"
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

	// Repositories
	var (
		userRepo            user.Repository
		profileRepo         profile.Repository
		parentYouthLinkRepo parentyouthlink.Repository
		rbacRepo            rbac.Repository
		eventRepo           event.Repository
	)

	if cfg.UseMockStorage {
		mockUserRepo := mock.NewUserRepository()
		mockProfileRepo := mock.NewProfileRepository()
		mockParentYouthLinkRepo := mock.NewParentYouthLinkRepository()
		mockRBACRepo := mock.NewRBACRepository()
		mockEventRepo := mock.NewEventRepository(mockProfileRepo)

		userRepo = mockUserRepo
		profileRepo = mockProfileRepo
		parentYouthLinkRepo = mockParentYouthLinkRepo
		rbacRepo = mockRBACRepo
		eventRepo = mockEventRepo

		ctx := context.Background()
		if err := mockRBACRepo.SeedRoles(ctx); err != nil {
			log.Fatalf("SeedRoles failed: %v", err)
		}

		hasher := &auth.MockHasher{}
		authService := auth.NewAuthService(userRepo, rbacRepo, hasher, cfg.SessionSecret)

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

		if cfg.SeedDevData {
			ctx := context.Background()

			if _, err := profileRepo.GetByEmail(ctx, "admin@scout.local"); err == nil {
				log.Println("Dev data already seeded, skipping")
			} else {
				if _, err := rbacRepo.GetRoleByName(ctx, "admin"); err != nil {
					log.Println("Roles missing (migration seed data was cleared), reseeding...")
					if err := auth.SeedRoles(ctx, rbacRepo); err != nil {
						log.Fatalf("SeedRoles failed: %v", err)
					}
				}

				hasher := &auth.BCryptHasher{}

				hash, err := hasher.Hash("password")
				if err != nil {
					log.Fatalf("Hash password: %v", err)
				}
				adminUser := &user.User{PasswordHash: hash}
				if err := userRepo.Create(ctx, adminUser); err != nil {
					log.Fatalf("Create admin user: %v", err)
				}
				adminRole, err := rbacRepo.GetRoleByName(ctx, "admin")
				if err != nil {
					log.Fatalf("GetRoleByName admin: %v", err)
				}
				if err := rbacRepo.AssignRoleToUser(ctx, adminUser.ID, adminRole.ID); err != nil {
					log.Fatalf("AssignRoleToUser: %v", err)
				}
				log.Println("Seeded admin user: admin@scout.local / password")

				adminProfile := &profile.Profile{
					FirstName:  "Admin",
					LastName:   "User",
					Email:      "admin@scout.local",
					MemberType: profile.MemberTypeAdult,
					Status:     profile.StatusActive,
					UserID:     &adminUser.ID,
				}
				if err := profileRepo.Create(ctx, adminProfile); err != nil {
					log.Fatalf("Create admin profile: %v", err)
				}
				log.Println("Created admin profile")

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

				if err := eventRepo.SignUp(ctx, seedEvents[0].ID, adminProfile.ID); err != nil {
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
				if err := profileRepo.Create(ctx, youthProfile); err != nil {
					log.Fatalf("Create youth profile: %v", err)
				}
				log.Println("Created youth profile: Alex Youth")

				link := &parentyouthlink.ParentYouthConnection{
					ParentProfileID: adminProfile.ID,
					YouthProfileID:  youthProfile.ID,
					Status:          parentyouthlink.StatusApproved,
				}
				if err := parentYouthLinkRepo.Create(ctx, link); err != nil {
					log.Fatalf("Create parent-youth link: %v", err)
				}
				log.Println("Linked Alex Youth to admin")

				if err := eventRepo.SignUp(ctx, seedEvents[0].ID, youthProfile.ID); err != nil {
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
				if err := profileRepo.Create(ctx, youthProfile2); err != nil {
					log.Fatalf("Create youth profile 2: %v", err)
				}
				link2 := &parentyouthlink.ParentYouthConnection{
					ParentProfileID: adminProfile.ID,
					YouthProfileID:  youthProfile2.ID,
					Status:          parentyouthlink.StatusApproved,
				}
				if err := parentYouthLinkRepo.Create(ctx, link2); err != nil {
					log.Fatalf("Create parent-youth link 2: %v", err)
				}
				log.Println("Created and linked Bailey Scout (not signed up)")
				log.Println("Login as admin@scout.local / password to manage Alex Youth and Bailey Scout via linked profiles")
			}
		}
	}

	// Auth
	hasher := &auth.BCryptHasher{}
	authService := auth.NewAuthService(userRepo, rbacRepo, hasher, cfg.SessionSecret)

	// Scoutbook sync
	scoutbookClient := scoutbook.NewClient(cfg.ScoutbookAPIBaseURL, cfg.ScoutbookToken, cfg.ScoutbookOrgGUID)
	syncSvc := sync.NewService(profileRepo, sync.NewScoutbookClientAdapter(scoutbookClient))
	syncHandler := api.NewSyncHandler(syncSvc, scoutbookClient)
	if cfg.ScoutbookOrgGUID == "" {
		log.Fatal("SCOUTBOOK_ORG_GUID must be set")
	}

	emailTmpl, err := appemail.NewTemplates()
	if err != nil {
		log.Fatalf("Failed to load email templates: %v", err)
	}
	emailSvc := appemail.NewSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom, cfg.UnitType, cfg.UnitNumber, emailTmpl)

	_ = emailSvc

	router := mux.NewRouter()
	router.HandleFunc("/healthcheck", api.HealthCheckHandler).Methods("GET")

	if !cfg.UseMockStorage {
		router.HandleFunc("/deepcheck", api.DeepCheckHandler(db)).Methods("GET")
	}

	authHandler := api.NewAuthHandler(authService)
	router.HandleFunc("/login", authHandler.LoginPage).Methods("GET")
	router.HandleFunc("/login", authHandler.Login).Methods("POST")
	router.HandleFunc("/logout", api.RequireAuth(authService, authHandler.Logout)).Methods("POST")

	eventHandler := api.NewEventHandler(eventRepo, authService, profileRepo, parentYouthLinkRepo, cfg.UnitType, cfg.UnitNumber)
	api.SetMuxVars(mux.Vars)
	router.Handle("/events", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListEvents)).Methods("GET")
	router.Handle("/events/upcoming", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListUpcoming)).Methods("GET")
	router.Handle("/events/past", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.ListPast)).Methods("GET")
	router.Handle("/events/{id}", api.RequirePermission(authService, rbacRepo, "event:view", eventHandler.EventDetail)).Methods("GET")
	router.Handle("/events/{id}/signup", api.RequirePermission(authService, rbacRepo, "event:signup", eventHandler.SignUp)).Methods("POST")
	router.Handle("/events/{id}/withdraw", api.RequirePermission(authService, rbacRepo, "event:withdraw", eventHandler.Withdraw)).Methods("POST")

	router.Handle("/admin/sync", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.AdminPage)).Methods("GET")
	router.Handle("/admin/sync/token", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.StoreToken)).Methods("POST")
	router.Handle("/admin/sync", api.RequirePermission(authService, rbacRepo, "event:create", syncHandler.Sync)).Methods("POST")

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
