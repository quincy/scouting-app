package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"scout-app/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	envFile := flag.String("env", "", "path to .env file")
	flag.Parse()

	if *envFile != "" {
		if err := config.LoadFile(*envFile); err != nil {
			log.Fatalf("Failed to load env file: %v", err)
		}
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	password := "password123"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}
	hashed := string(hash)

	type seedUser struct {
		Email    string
		Password string
		Name     string
		Type     string
		Status   string
	}

	inactiveUsers := []seedUser{
		{Email: "inactive.adult@test.local", Password: password, Name: "Inactive Adult", Type: "adult", Status: "inactive"},
		{Email: "inactive.youth@test.local", Password: password, Name: "Inactive Youth", Type: "youth", Status: "inactive"},
	}

	parentUser := seedUser{Email: "active.parent@test.local", Password: password, Name: "Active Parent", Type: "adult", Status: "active"}

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	fmt.Println("=== Created Test Users ===")
	fmt.Println()

	createUser := func(u seedUser) (userID, profileID string) {
		err := tx.QueryRow(
			`INSERT INTO users (password_hash) VALUES ($1) RETURNING id`,
			hashed,
		).Scan(&userID)
		if err != nil {
			log.Fatalf("Failed to create user %s: %v", u.Email, err)
		}

		err = tx.QueryRow(
			`INSERT INTO profiles (first_name, last_name, email, member_type, status, user_id, birthdate)
			 VALUES ($1, $2, $3, $4, $5, $6, '2000-01-01') RETURNING id`,
			u.Name, "User", u.Email, u.Type, u.Status, userID,
		).Scan(&profileID)
		if err != nil {
			log.Fatalf("Failed to create profile for %s: %v", u.Email, err)
		}

		fmt.Printf("  Email:    %s\n", u.Email)
		fmt.Printf("  Password: %s\n", u.Password)
		fmt.Printf("  Name:     %s\n", u.Name)
		fmt.Printf("  Status:   %s\n", u.Status)
		fmt.Println()
		return
	}

	for _, u := range inactiveUsers {
		createUser(u)
	}

	parentUserID, parentProfileID := createUser(parentUser)

	var parentRoleID string
	err = tx.QueryRow(`SELECT id FROM roles WHERE name = 'parent'`).Scan(&parentRoleID)
	if err != nil {
		log.Fatalf("Failed to find parent role: %v", err)
	}
	_, err = tx.Exec(
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`,
		parentUserID, parentRoleID,
	)
	if err != nil {
		log.Fatalf("Failed to assign parent role: %v", err)
	}

	var youthProfileID string
	err = tx.QueryRow(
		`SELECT id FROM profiles WHERE email = $1`, "inactive.youth@test.local",
	).Scan(&youthProfileID)
	if err != nil {
		log.Fatalf("Failed to find inactive youth profile: %v", err)
	}

	_, err = tx.Exec(
		`INSERT INTO parent_youth_links (parent_profile_id, youth_profile_id, status)
		 VALUES ($1, $2, 'approved')`,
		parentProfileID, youthProfileID,
	)
	if err != nil {
		log.Fatalf("Failed to create parent-youth link: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit: %v", err)
	}

	fmt.Println("=== Notes ===")
	fmt.Println("- Inactive users cannot log in (admins can bypass).")
	fmt.Println("- Inactive profiles hidden from sign-up dropdowns.")
	fmt.Println("- Active Parent can see Inactive Youth in their sign-up dropdown (but sign-up will be blocked).")
	fmt.Println()
}
