package main

import (
	"flag"
	"log"
	"os"

	"scout-app/internal/config"
	"scout-app/internal/storage"
	"scout-app/migrations"
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

	db, err := storage.OpenDB(dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Running database migrations...")
	if err := storage.RunMigrations(db, migrations.FS, "."); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations complete")
}
