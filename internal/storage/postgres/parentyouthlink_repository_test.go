package postgres

import (
	"context"
	"testing"

	"scout-app/internal/domain/parentyouthlink"
	"scout-app/internal/domain/profile"
)

func TestPostgresParentYouthLinkRepository_CreateAndGetByID(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	linkRepo := NewParentYouthLinkRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	parent := &profile.Profile{FirstName: "Parent", LastName: "P", Email: "parent@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive}
	youth := &profile.Profile{FirstName: "Youth", LastName: "Y", Email: "youth@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive}
	profileRepo.Create(ctx, parent)
	profileRepo.Create(ctx, youth)

	link := &parentyouthlink.ParentYouthConnection{
		ParentProfileID: parent.ID,
		YouthProfileID:  youth.ID,
		Status:          parentyouthlink.StatusPending,
	}
	if err := linkRepo.Create(ctx, link); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if link.ID == "" {
		t.Error("expected generated ID")
	}

	fetched, err := linkRepo.GetByID(ctx, link.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.ParentProfileID != parent.ID {
		t.Errorf("expected parent ID %s, got %s", parent.ID, fetched.ParentProfileID)
	}
}

func TestPostgresParentYouthLinkRepository_ListByParent(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	linkRepo := NewParentYouthLinkRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	parent := &profile.Profile{FirstName: "Parent2", LastName: "P", Email: "parent2@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive}
	youth1 := &profile.Profile{FirstName: "Y1", LastName: "Y", Email: "y1@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive}
	youth2 := &profile.Profile{FirstName: "Y2", LastName: "Y", Email: "y2@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive}
	profileRepo.Create(ctx, parent)
	profileRepo.Create(ctx, youth1)
	profileRepo.Create(ctx, youth2)

	linkRepo.Create(ctx, &parentyouthlink.ParentYouthConnection{ParentProfileID: parent.ID, YouthProfileID: youth1.ID, Status: parentyouthlink.StatusApproved})
	linkRepo.Create(ctx, &parentyouthlink.ParentYouthConnection{ParentProfileID: parent.ID, YouthProfileID: youth2.ID, Status: parentyouthlink.StatusPending})

	links, err := linkRepo.ListByParent(ctx, parent.ID)
	if err != nil {
		t.Fatalf("ListByParent failed: %v", err)
	}
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}

	pending, err := linkRepo.ListByStatus(ctx, parentyouthlink.StatusPending)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending link, got %d", len(pending))
	}
}

func TestPostgresParentYouthLinkRepository_UpdateStatus(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}
	truncateAll(t)
	linkRepo := NewParentYouthLinkRepository(testDB)
	profileRepo := NewProfileRepository(testDB)
	ctx := context.Background()

	parent := &profile.Profile{FirstName: "P", LastName: "Parent", Email: "p@test.com", MemberType: profile.MemberTypeAdult, Status: profile.StatusActive}
	youth := &profile.Profile{FirstName: "Y", LastName: "Youth", Email: "y@test.com", MemberType: profile.MemberTypeYouth, Status: profile.StatusActive}
	profileRepo.Create(ctx, parent)
	profileRepo.Create(ctx, youth)

	link := &parentyouthlink.ParentYouthConnection{ParentProfileID: parent.ID, YouthProfileID: youth.ID, Status: parentyouthlink.StatusPending}
	linkRepo.Create(ctx, link)

	if err := linkRepo.UpdateStatus(ctx, link.ID, parentyouthlink.StatusApproved, "admin-user-id"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	fetched, err := linkRepo.GetByID(ctx, link.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Status != parentyouthlink.StatusApproved {
		t.Errorf("expected status approved, got %s", fetched.Status)
	}
	if fetched.ApprovedBy == nil || *fetched.ApprovedBy != "admin-user-id" {
		t.Errorf("expected approved_by admin-user-id, got %v", fetched.ApprovedBy)
	}
	if fetched.ApprovedAt == nil {
		t.Error("expected non-nil approved_at")
	}
}
