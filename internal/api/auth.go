package api

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"scout-app/internal/domain"
)

// loginPageData is passed to the login template.
type loginPageData struct {
	Error string
}

// AuthHandler serves login and logout endpoints.
type AuthHandler struct {
	auth *domain.AuthService
	tmpl *template.Template
}

// NewAuthHandler creates an AuthHandler with compiled templates.
func NewAuthHandler(auth *domain.AuthService) *AuthHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/login.html"),
	)
	return &AuthHandler{
		auth: auth,
		tmpl: tmpl,
	}
}

// LoginPage renders the login form.
// GET /login
func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := loginPageData{}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		data.Error = errMsg
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
		log.Printf("login template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Login authenticates the user and creates a session.
// POST /login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/events"
	}

	_, err := h.auth.Login(w, r, email, password)
	if err != nil {
		http.Redirect(w, r, "/login?error=Invalid+credentials", http.StatusFound)
		return
	}

	http.Redirect(w, r, redirect, http.StatusFound)
}

// Logout clears the session and redirects to login.
// POST /logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.Logout(w, r); err != nil {
		log.Printf("Logout error: %v", err)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}
