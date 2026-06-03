package api

import (
	"log"
	"net/http"
	"net/url"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/rbac"
)

// RequireAuth returns a handler that checks for a valid session.
// If no session, redirects to /login?redirect=<original-path>.
func RequireAuth(authSvc *auth.AuthService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := authSvc.GetAuthenticatedUser(r)
		if err != nil || user == nil {
			redirectURL := "/login?redirect=" + url.QueryEscape(r.URL.Path)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
		next(w, r)
	}
}

// RequirePermission returns a handler that checks for a valid session
// AND the given permission. If missing permission, returns 403 Forbidden.
func RequirePermission(authSvc *auth.AuthService, rbacRepo rbac.Repository, permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := authSvc.GetAuthenticatedUser(r)
		if err != nil || user == nil {
			redirectURL := "/login?redirect=" + url.QueryEscape(r.URL.Path)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		perms, err := rbacRepo.GetUserPermissions(r.Context(), user.ID)
		if err != nil {
			log.Printf("GetUserPermissions error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		hasPermission := false
		for _, p := range perms {
			if p.Name == permission {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
