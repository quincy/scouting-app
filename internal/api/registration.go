package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/email"
	"scout-app/internal/domain/otpcode"
	"scout-app/internal/domain/profile"
	"scout-app/internal/domain/rbac"
	"scout-app/internal/domain/user"

	"github.com/gorilla/sessions"
)

type RegistrationHandler struct {
	profileRepo profile.Repository
	otpRepo     otpcode.Repository
	userRepo    user.Repository
	rbacRepo    rbac.Repository
	emailSvc    email.Service
	hasher      auth.Hasher
	session     *sessions.CookieStore
	tmpl        *template.Template
}

func NewRegistrationHandler(
	profileRepo profile.Repository,
	otpRepo otpcode.Repository,
	userRepo user.Repository,
	rbacRepo rbac.Repository,
	emailSvc email.Service,
	hasher auth.Hasher,
	session *sessions.CookieStore,
) *RegistrationHandler {
	tmpl := template.Must(
		template.New("").ParseFS(viewsFS, "views/register.html", "views/register_verify.html", "views/register_complete.html"),
	)
	return &RegistrationHandler{
		profileRepo: profileRepo,
		otpRepo:     otpRepo,
		userRepo:    userRepo,
		rbacRepo:    rbacRepo,
		emailSvc:    emailSvc,
		hasher:      hasher,
		session:     session,
		tmpl:        tmpl,
	}
}

type registerPageData struct {
	Error string
}

type verifyPageData struct {
	OTPID       string
	MaskedEmail string
	Error       string
}

type completePageData struct {
	Error string
}

// GET /register
func (h *RegistrationHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	data := registerPageData{}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "register.html", data); err != nil {
		log.Printf("register template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// POST /register
func (h *RegistrationHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)

	if email == "" {
		h.renderRegister(w, "Please enter your email address.")
		return
	}

	ctx := r.Context()

	prof, err := h.profileRepo.GetByEmail(ctx, email)
	if err != nil {
		h.renderRegister(w, "No account found for this email address. Double-check that the email you entered matches the email registered with your Scoutbook account. If you continue having trouble, contact your Troop Webmaster for help.")
		return
	}

	if prof.UserID != nil {
		h.renderRegister(w, "This account has already been registered. <a href=\"/login\">Sign in</a> instead.")
		return
	}

	count, err := h.otpRepo.CountByEmailSince(ctx, email, time.Now().Add(-1*time.Hour))
	if err != nil {
		log.Printf("rate limit check: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if count >= 5 {
		h.renderRegister(w, "Too many verification code requests. Please try again later.")
		return
	}

	if err := h.otpRepo.InvalidateByEmail(ctx, email); err != nil {
		log.Printf("invalidate existing OTPs: %v", err)
	}

	plainCode, otp, err := otpcode.NewOTPCode(email)
	if err != nil {
		log.Printf("generate OTP: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	sess, err := h.session.Get(r, auth.SessionName)
	if err != nil {
		log.Printf("session get: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sess.Values["register_email"] = email
	if err := sess.Save(r, w); err != nil {
		log.Printf("session save: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.otpRepo.Create(ctx, otp); err != nil {
		log.Printf("save OTP: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.emailSvc.SendOTP(ctx, email, plainCode, otp.ID); err != nil {
		log.Printf("send OTP email: %v", err)
	}

	http.Redirect(w, r, "/register/verify?otp_id="+otp.ID, http.StatusFound)
}

func (h *RegistrationHandler) renderRegister(w http.ResponseWriter, errMsg string) {
	data := registerPageData{Error: errMsg}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "register.html", data); err != nil {
		log.Printf("register template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GET /register/verify
func (h *RegistrationHandler) VerifyPage(w http.ResponseWriter, r *http.Request) {
	otpID := r.URL.Query().Get("otp_id")
	if otpID == "" {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	ctx := r.Context()
	otp, err := h.otpRepo.GetByID(ctx, otpID)
	if err != nil || otp.IsExpired() || otp.Used {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	data := verifyPageData{
		OTPID:       otpID,
		MaskedEmail: maskEmail(otp.Email),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "register_verify.html", data); err != nil {
		log.Printf("verify template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// POST /register/verify
func (h *RegistrationHandler) Verify(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	otpID := r.FormValue("otp_id")
	code := r.FormValue("code")

	if otpID == "" || code == "" {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	ctx := r.Context()

	otp, err := h.otpRepo.GetByID(ctx, otpID)
	if err != nil {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	if otp.IsExpired() || otp.Used {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	if otp.Attempts >= otpcode.MaxAttempts {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	inputHash := sha256.Sum256([]byte(code))
	if subtle.ConstantTimeCompare(inputHash[:], otp.CodeHash) != 1 {
		newAttempts, err := h.otpRepo.IncrementAttempts(ctx, otpID)
		if err != nil {
			log.Printf("increment attempts: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if newAttempts >= otpcode.MaxAttempts {
			http.Redirect(w, r, "/register", http.StatusFound)
			return
		}
		data := verifyPageData{
			OTPID:       otpID,
			MaskedEmail: maskEmail(otp.Email),
			Error:       "Invalid verification code. Please try again.",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.tmpl.ExecuteTemplate(w, "register_verify.html", data); err != nil {
			log.Printf("verify template execution: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	ok, err := h.otpRepo.MarkUsedIfUnused(ctx, otpID)
	if err != nil {
		log.Printf("mark used: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	sess, err := h.session.Get(r, auth.SessionName)
	if err != nil {
		log.Printf("session get: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sess.Values["verified_email"] = otp.Email
	sess.Values["verified_at"] = time.Now().Format(time.RFC3339)
	delete(sess.Values, "register_email")
	if err := sess.Save(r, w); err != nil {
		log.Printf("session save error: %v", err)
		http.Error(w, fmt.Sprintf("session save error: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/register/complete", http.StatusFound)
}

// GET /register/complete
func (h *RegistrationHandler) CompletePage(w http.ResponseWriter, r *http.Request) {
	sess, err := h.session.Get(r, auth.SessionName)
	if err != nil {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	verifiedEmail, ok := sess.Values["verified_email"].(string)
	if !ok || verifiedEmail == "" {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	if isVerifiedExpired(sess) {
		delete(sess.Values, "verified_email")
		delete(sess.Values, "verified_at")
		_ = sess.Save(r, w)
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	data := completePageData{}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "register_complete.html", data); err != nil {
		log.Printf("complete template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// POST /register/complete
func (h *RegistrationHandler) Complete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	sess, err := h.session.Get(r, auth.SessionName)
	if err != nil {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	verifiedEmail, ok := sess.Values["verified_email"].(string)
	if !ok || verifiedEmail == "" {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	if isVerifiedExpired(sess) {
		delete(sess.Values, "verified_email")
		delete(sess.Values, "verified_at")
		_ = sess.Save(r, w)
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	password := r.FormValue("password")
	if password == "" {
		h.renderComplete(w, "Please enter a password.")
		return
	}

	ctx := r.Context()

	prof, err := h.profileRepo.GetByEmail(ctx, verifiedEmail)
	if err != nil {
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	hash, err := h.hasher.Hash(password)
	if err != nil {
		log.Printf("hash password: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	u := &user.User{PasswordHash: hash}
	if err := h.userRepo.Create(ctx, u); err != nil {
		log.Printf("create user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var roleName string
	if prof.MemberType == profile.MemberTypeAdult {
		roleName = "parent"
	} else {
		roleName = "Scouts BSA"
	}

	role, err := h.rbacRepo.GetRoleByName(ctx, roleName)
	if err != nil {
		log.Printf("get role %q: %v", roleName, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.rbacRepo.AssignRoleToUser(ctx, u.ID, role.ID); err != nil {
		log.Printf("assign role: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if prof.Positions != "" {
		if _, _, err := rbac.ReconcileRoles(ctx, h.rbacRepo, prof.ID, u.ID, prof.Positions); err != nil {
			log.Printf("reconcile roles: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	prof.UserID = &u.ID
	if err := h.profileRepo.Update(ctx, prof); err != nil {
		log.Printf("update profile: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	delete(sess.Values, "verified_email")
	delete(sess.Values, "verified_at")
	if err := sess.Save(r, w); err != nil {
		log.Printf("session save: %v", err)
	}

	http.Redirect(w, r, "/login?registered=1", http.StatusFound)
}

func (h *RegistrationHandler) renderComplete(w http.ResponseWriter, errMsg string) {
	data := completePageData{Error: errMsg}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "register_complete.html", data); err != nil {
		log.Printf("complete template execution: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func isVerifiedExpired(sess *sessions.Session) bool {
	verifiedAtStr, ok := sess.Values["verified_at"].(string)
	if !ok {
		return false
	}
	verifiedAt, err := time.Parse(time.RFC3339, verifiedAtStr)
	if err != nil {
		return false
	}
	return time.Since(verifiedAt) > 30*time.Minute
}

func maskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 1 {
		return email
	}
	prefix := email[:at]
	suffix := email[at:]
	masked := string(prefix[0]) + strings.Repeat("*", len(prefix)-1)
	dot := strings.Index(suffix, ".")
	if dot > 1 {
		masked += string(suffix[0]) + strings.Repeat("*", dot-1) + suffix[dot:]
	} else {
		masked += suffix
	}
	return masked
}
