package main

import (
	"os"
	"testing"

	"scout-app/internal/config"
	"scout-app/internal/testhelper"
)

func TestMain(m *testing.M) {
	testhelper.StartDB()
	os.Exit(m.Run())
}

func TestOpenDatabase(t *testing.T) {
	dsn := testhelper.DSN()
	cfg := &config.Config{DatabaseURL: dsn}
	db := openDatabase(cfg)
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping failed: %v", err)
	}
	db.Close()
}

func TestBuildApp(t *testing.T) {
	cfg := &config.Config{
		Addr:                ":0",
		DatabaseURL:         testhelper.DSN(),
		SessionSecret:       "test-secret-for-testing",
		ScoutbookAPIBaseURL: "http://localhost:9999",
		ScoutbookOrgGUID:    "",
		ScoutbookToken:      "",
		UnitType:            "Troop",
		UnitNumber:          "077",
	}

	db := openDatabase(cfg)
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	defer db.Close()

	router, stopCleanup := buildApp(cfg, db)
	defer stopCleanup()

	if router == nil {
		t.Error("expected non-nil router")
	}
	if stopCleanup == nil {
		t.Error("expected non-nil stopCleanup")
	}
}
