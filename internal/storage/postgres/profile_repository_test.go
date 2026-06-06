package postgres

import (
	"context"
	"testing"

	"scout-app/internal/domain/profile"
)

func TestPostgresProfileRepository_CreateAndGetByID(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewProfileRepository(testDB)
	ctx := context.Background()

	p := &profile.Profile{
		FirstName:  "Alice",
		LastName:   "Smith",
		Email:      "alice@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}

	err := repo.Create(ctx, p)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if p.ID == "" {
		t.Error("expected generated ID")
	}

	fetched, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if fetched.FirstName != "Alice" || fetched.LastName != "Smith" {
		t.Errorf("got %s %s, expected Alice Smith", fetched.FirstName, fetched.LastName)
	}
}

func TestPostgresProfileRepository_GetByEmail(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewProfileRepository(testDB)
	ctx := context.Background()

	p := &profile.Profile{
		FirstName:  "Bob",
		LastName:   "Jones",
		Email:      "bob@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	repo.Create(ctx, p)

	fetched, err := repo.GetByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	if fetched.ID != p.ID {
		t.Errorf("expected ID %s, got %s", p.ID, fetched.ID)
	}
}

func TestPostgresProfileRepository_GetByBSAID(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewProfileRepository(testDB)
	ctx := context.Background()

	p := &profile.Profile{
		FirstName:  "Charlie",
		LastName:   "Brown",
		Email:      "charlie@example.com",
		BSAID:      "BSA12345",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	repo.Create(ctx, p)

	fetched, err := repo.GetByBSAID(ctx, "BSA12345")
	if err != nil {
		t.Fatalf("GetByBSAID failed: %v", err)
	}
	if fetched.ID != p.ID {
		t.Errorf("expected ID %s, got %s", p.ID, fetched.ID)
	}

	_, err = repo.GetByBSAID(ctx, "")
	if err == nil {
		t.Error("expected error for empty BSA ID")
	}
}

func TestPostgresProfileRepository_ListByStatus(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewProfileRepository(testDB)
	ctx := context.Background()

	active := &profile.Profile{FirstName: "A", LastName: "Active", Email: "a@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive}
	inactive := &profile.Profile{FirstName: "B", LastName: "Inactive", Email: "b@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusInactive}
	repo.Create(ctx, active)
	repo.Create(ctx, inactive)

	activeProfiles, err := repo.ListByStatus(ctx, profile.StatusActive)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}
	if len(activeProfiles) != 1 {
		t.Errorf("expected 1 active profile, got %d", len(activeProfiles))
	}

	inactiveProfiles, err := repo.ListByStatus(ctx, profile.StatusInactive)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}
	if len(inactiveProfiles) != 1 {
		t.Errorf("expected 1 inactive profile, got %d", len(inactiveProfiles))
	}
}

func TestPostgresProfileRepository_Update(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	repo := NewProfileRepository(testDB)
	ctx := context.Background()

	p := &profile.Profile{
		FirstName:  "Old",
		LastName:   "Name",
		Email:      "old@example.com",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	repo.Create(ctx, p)

	p.FirstName = "New"
	p.LastName = "Name2"
	p.Status = profile.StatusInactive
	if err := repo.Update(ctx, p); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	fetched, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if fetched.FirstName != "New" || fetched.Status != profile.StatusInactive {
		t.Errorf("after update got %s/%s, expected New/inactive", fetched.FirstName, fetched.Status)
	}
}
