package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/user"
)

func TestRosterEndpoint_AdminGetsRosterPage(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	resp, err := env.client.Get(env.server.URL + "/events/" + env.event.ID + "/roster")
	if err != nil {
		t.Fatalf("GET roster: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := new(strings.Builder)
	if _, err := io.Copy(body, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(body.String(), "Attendance (Roster)") {
		t.Errorf("expected Attendance section in roster page:\n%s", body.String())
	}
	if !strings.Contains(body.String(), "Drivers") {
		t.Errorf("expected Drivers section in roster page:\n%s", body.String())
	}
	if !strings.Contains(body.String(), "Back to event") {
		t.Errorf("expected back link in roster page:\n%s", body.String())
	}
}

func TestRosterEndpoint_NonAdminForbidden(t *testing.T) {
	env := setupDriverTest(t)
	defer env.cleanup()

	ctx := context.Background()
	hasher := &auth.BCryptHasher{}
	hash, err := hasher.Hash("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &user.User{Email: "parent@scout.local", PasswordHash: hash}
	if err := env.store.User.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	p := &profile.Profile{
		FirstName: "Parent", LastName: "User", Email: "parent@scout.local",
		MemberType: profile.MemberTypeAdult, Status: profile.StatusActive,
		UserID: &u.ID,
	}
	if err := env.store.Profile.Create(ctx, p); err != nil {
		t.Fatalf("Create profile: %v", err)
	}
	role, err := env.store.RBAC.GetRoleByName(ctx, "parent")
	if err != nil {
		t.Fatalf("GetRoleByName parent: %v", err)
	}
	if err := env.store.RBAC.AssignRoleToUser(ctx, u.ID, role.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	loginResp, err := env.client.PostForm(env.server.URL+"/login", url.Values{
		"email": {"parent@scout.local"}, "password": {"password"},
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if loginResp.StatusCode != http.StatusFound && loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", loginResp.StatusCode)
	}
	loginResp.Body.Close()

	resp, err := env.client.Get(env.server.URL + "/events/" + env.event.ID + "/roster")
	if err != nil {
		t.Fatalf("GET roster: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
