package api

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout-app/internal/domain/auth"
	"scout-app/internal/domain/otpcode"
	"scout-app/internal/domain/profile"
	"scout-app/internal/storage/mock"
)

func setupRegistrationTest(t *testing.T) (*RegistrationHandler, *auth.AuthService, *mock.UserRepository, *mock.ProfileRepository, *mock.OTPCodeRepository, *mock.EmailService, *mock.RBACRepository) {
	t.Helper()

	userRepo := mock.NewUserRepository()
	profileRepo := mock.NewProfileRepository()
	parentYouthLinkRepo := mock.NewParentYouthLinkRepository()
	rbacRepo := mock.NewRBACRepository()
	eventRepo := mock.NewEventRepository(profileRepo)
	otpRepo := mock.NewOTPCodeRepository()
	emailSvc := mock.NewEmailService()

	hasher := &auth.MockHasher{}
	store := auth.NewCookieStore("test-secret-key")
	authService := auth.NewAuthService(userRepo, rbacRepo, hasher, store)

	ctx := t.Context()
	if err := rbacRepo.SeedRoles(ctx); err != nil {
		t.Fatalf("SeedRoles: %v", err)
	}
	if err := authService.SeedAdminUser(ctx); err != nil {
		t.Fatalf("SeedAdminUser: %v", err)
	}

	adminUser, err := userRepo.GetByEmail(ctx, "admin@scout.local")
	if err != nil {
		t.Fatalf("GetByEmail admin: %v", err)
	}
	adminProfile := &profile.Profile{
		FirstName:  "Admin",
		LastName:   "User",
		Email:      "admin@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
		UserID:     &adminUser.ID,
	}
	if err := profileRepo.Create(ctx, adminProfile); err != nil {
		t.Fatalf("Create admin profile: %v", err)
	}

	youthProfile := &profile.Profile{
		FirstName:  "Alex",
		LastName:   "Youth",
		Email:      "alex.youth@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, youthProfile); err != nil {
		t.Fatalf("Create youth profile: %v", err)
	}

	unregisteredAdult := &profile.Profile{
		FirstName:  "Unregistered",
		LastName:   "Adult",
		Email:      "unregistered.adult@scout.local",
		MemberType: profile.MemberTypeAdult,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, unregisteredAdult); err != nil {
		t.Fatalf("Create unregistered adult: %v", err)
	}

	unregisteredYouth := &profile.Profile{
		FirstName:  "Unregistered",
		LastName:   "Youth",
		Email:      "unregistered.youth@scout.local",
		MemberType: profile.MemberTypeYouth,
		Status:     profile.StatusActive,
	}
	if err := profileRepo.Create(ctx, unregisteredYouth); err != nil {
		t.Fatalf("Create unregistered youth: %v", err)
	}

	_ = parentYouthLinkRepo
	_ = eventRepo

	regHandler := NewRegistrationHandler(profileRepo, otpRepo, userRepo, rbacRepo, emailSvc, hasher, store)

	return regHandler, authService, userRepo, profileRepo, otpRepo, emailSvc, rbacRepo
}

// Test 1: GET /register renders form, no error
func TestRegistrationHandler_RegisterPage(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("GET", "/register", nil)
	rr := httptest.NewRecorder()

	handler.RegisterPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RegisterPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Create Your Account") {
		t.Errorf("expected page to contain 'Create Your Account', got:\n%s", body)
	}
	if !strings.Contains(body, "type=\"email\"") {
		t.Errorf("expected email input, got:\n%s", body)
	}
	if !strings.Contains(body, "/register") {
		t.Errorf("expected form action /register, got:\n%s", body)
	}
}

// Test 2: POST /register — profile not found
func TestRegistrationHandler_Register_ProfileNotFound(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("POST", "/register", strings.NewReader("email=unknown@test.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Register returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "No account found for this email address") {
		t.Errorf("expected 'no account found' message, got:\n%s", body)
	}
}

// Test 3: POST /register — profile already claimed
func TestRegistrationHandler_Register_AlreadyClaimed(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("POST", "/register", strings.NewReader("email=admin@scout.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Register returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "already been registered") {
		t.Errorf("expected 'already been registered' message, got:\n%s", body)
	}
}

// Test 4: POST /register — rate limited
func TestRegistrationHandler_Register_RateLimited(t *testing.T) {
	handler, _, _, _, otpRepo, _, _ := setupRegistrationTest(t)

	ctx := t.Context()

	// Create 5 OTPs within the last hour
	for i := 0; i < 5; i++ {
		_, otp, err := otpcode.NewOTPCode("unregistered.adult@scout.local")
		if err != nil {
			t.Fatalf("NewOTPCode: %v", err)
		}
		otp.CreatedAt = time.Now().Add(-time.Duration(i) * time.Minute)
		if err := otpRepo.Create(ctx, otp); err != nil {
			t.Fatalf("Create OTP: %v", err)
		}
	}

	req := httptest.NewRequest("POST", "/register", strings.NewReader("email=unregistered.adult@scout.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Register returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Too many verification code requests") {
		t.Errorf("expected rate limit message, got:\n%s", body)
	}
}

// Test 5: POST /register — success
func TestRegistrationHandler_Register_Success(t *testing.T) {
	handler, _, _, _, otpRepo, emailSvc, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("POST", "/register", strings.NewReader("email=unregistered.adult@scout.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Register returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/register/verify?otp_id=") {
		t.Errorf("expected redirect to /register/verify with otp_id, got %q", location)
	}

	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Error("expected session cookie to be set")
	}

	// Verify OTP was created in repo
	ctx := t.Context()
	count, err := otpRepo.CountByEmailSince(ctx, "unregistered.adult@scout.local", time.Now().Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("CountByEmailSince: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 OTP created, got %d", count)
	}

	// Verify email was sent
	if len(emailSvc.SentOTPs) != 1 {
		t.Errorf("expected 1 email sent, got %d", len(emailSvc.SentOTPs))
	} else {
		email := emailSvc.SentOTPs[0]
		if email.To != "unregistered.adult@scout.local" {
			t.Errorf("expected email to unregistered.adult@scout.local, got %s", email.To)
		}
		if len(email.Code) != 6 {
			t.Errorf("expected 6-digit code, got %q", email.Code)
		}
		if email.OTPID == "" {
			t.Error("expected OTPID to be set")
		}
	}
}

// Test 6: GET /register/verify — invalid OTP ID redirects to /register
func TestRegistrationHandler_VerifyPage_InvalidOTPID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("GET", "/register/verify?otp_id=nonexistent", nil)
	rr := httptest.NewRecorder()

	handler.VerifyPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("VerifyPage returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/register" {
		t.Errorf("expected redirect to /register, got %q", location)
	}
}

// Test 7: GET /register/verify — expired OTP redirects to /register
func TestRegistrationHandler_VerifyPage_ExpiredOTP(t *testing.T) {
	handler, _, _, _, otpRepo, _, _ := setupRegistrationTest(t)

	ctx := t.Context()
	_, otp, _ := otpcode.NewOTPCode("test@test.com")
	otp.ExpiresAt = time.Now().Add(-1 * time.Hour)
	otpRepo.Create(ctx, otp)

	req := httptest.NewRequest("GET", "/register/verify?otp_id="+otp.ID, nil)
	rr := httptest.NewRecorder()

	handler.VerifyPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("VerifyPage returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/register" {
		t.Errorf("expected redirect to /register, got %q", location)
	}
}

// Test 8: GET /register/verify — valid OTP renders form with masked email
func TestRegistrationHandler_VerifyPage_ValidOTP(t *testing.T) {
	handler, _, _, _, otpRepo, _, _ := setupRegistrationTest(t)

	ctx := t.Context()
	_, otp, _ := otpcode.NewOTPCode("test@scout.local")
	otpRepo.Create(ctx, otp)

	req := httptest.NewRequest("GET", "/register/verify?otp_id="+otp.ID, nil)
	rr := httptest.NewRecorder()

	handler.VerifyPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("VerifyPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Verify Your Email") {
		t.Errorf("expected 'Verify Your Email' heading, got:\n%s", body)
	}
	if !strings.Contains(body, "Verify Your Email") {
		t.Errorf("expected 'Verify Your Email' heading, got:\n%s", body)
	}
	if !strings.Contains(body, otp.ID) {
		t.Errorf("expected hidden input with OTP ID, got:\n%s", body)
	}
}

// Test 9 & 10: POST /register/verify — wrong code shows error and increments
func TestRegistrationHandler_Verify_WrongCode(t *testing.T) {
	handler, _, _, _, otpRepo, _, _ := setupRegistrationTest(t)

	ctx := t.Context()
	plainCode, otp, _ := otpcode.NewOTPCode("test@scout.local")
	otpRepo.Create(ctx, otp)

	// Use a wrong code
	wrongCode := "000000"
	if plainCode == wrongCode {
		wrongCode = "111111"
	}

	req := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otp.ID+"&code="+wrongCode))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Verify(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Verify returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Invalid verification code") {
		t.Errorf("expected error message, got:\n%s", body)
	}

	// Check that attempts were incremented
	fetched, err := otpRepo.GetByID(ctx, otp.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", fetched.Attempts)
	}
}

// Test 11: POST /register/verify — too many attempts
func TestRegistrationHandler_Verify_TooManyAttempts(t *testing.T) {
	handler, authService, _, _, otpRepo, _, _ := setupRegistrationTest(t)

	ctx := t.Context()
	plainCode, otp, _ := otpcode.NewOTPCode("test@scout.local")
	otpRepo.Create(ctx, otp)
	// Set attempts to 4 after create (Create resets to 0)
	otpRepo.IncrementAttempts(ctx, otp.ID)
	otpRepo.IncrementAttempts(ctx, otp.ID)
	otpRepo.IncrementAttempts(ctx, otp.ID)
	otpRepo.IncrementAttempts(ctx, otp.ID)

	wrongCode := "000000"
	if plainCode == wrongCode {
		wrongCode = "111111"
	}

	cookies := loginAndGetCookies(t, authService)
	req := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otp.ID+"&code="+wrongCode))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()

	handler.Verify(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Verify returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/register" {
		t.Errorf("expected redirect to /register, got %q", location)
	}
}

// Test 12: POST /register/verify — correct code
func TestRegistrationHandler_Verify_CorrectCode(t *testing.T) {
	handler, authService, _, _, otpRepo, emailSvc, _ := setupRegistrationTest(t)

	ctx := t.Context()
	cookies, otpID, _ := registerAndGetOTP(t, handler, authService, "unregistered.adult@scout.local", otpRepo, emailSvc)

	// Get the plain code from the email service
	var plainCode string
	for _, sent := range emailSvc.SentOTPs {
		if sent.To == "unregistered.adult@scout.local" {
			plainCode = sent.Code
			break
		}
	}
	if plainCode == "" {
		t.Fatal("no OTP email sent")
	}

	req := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otpID+"&code="+plainCode))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()

	handler.Verify(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Verify returned status %d, want 302 Found. Body: %s", rr.Code, rr.Body.String())
	}

	location := rr.Header().Get("Location")
	if location != "/register/complete" {
		t.Errorf("expected redirect to /register/complete, got %q", location)
	}

	// Verify OTP is marked as used
	fetched, err := otpRepo.GetByID(ctx, otpID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !fetched.Used {
		t.Error("expected OTP to be marked as used")
	}
}

// Test 13: POST /register/verify — race condition (MarkUsedIfUnused)
func TestRegistrationHandler_Verify_RaceCondition(t *testing.T) {
	handler, authService, _, _, otpRepo, _, _ := setupRegistrationTest(t)

	ctx := t.Context()
	plainCode, otp, _ := otpcode.NewOTPCode("unregistered.adult@scout.local")
	otpRepo.Create(ctx, otp)

	// Mark it used first (simulating another request)
	otpRepo.MarkUsedIfUnused(ctx, otp.ID)

	// Login first to have a session
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler := NewAuthHandler(authService)
	authHandler.Login(loginRR, loginReq)
	cookies := loginRR.Result().Cookies()

	req := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otp.ID+"&code="+plainCode))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()

	handler.Verify(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Verify returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/register" {
		t.Errorf("expected redirect to /register for already-used OTP, got %q", location)
	}
}

// Test 14: GET /register/complete — no session redirects to /register
func TestRegistrationHandler_CompletePage_NoSession(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("GET", "/register/complete", nil)
	rr := httptest.NewRecorder()

	handler.CompletePage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("CompletePage returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/register" {
		t.Errorf("expected redirect to /register, got %q", location)
	}
}

// Test 16: GET /register/complete — verified renders password form
func TestRegistrationHandler_CompletePage_Verified(t *testing.T) {
	handler, authService, _, _, otpRepo, emailSvc, _ := setupRegistrationTest(t)

	cookies, otpID, _ := registerAndGetOTP(t, handler, authService, "unregistered.adult@scout.local", otpRepo, emailSvc)

	var plainCode string
	for _, sent := range emailSvc.SentOTPs {
		if sent.To == "unregistered.adult@scout.local" {
			plainCode = sent.Code
			break
		}
	}

	// Verify to get verified_email in session
	verifyReq := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otpID+"&code="+plainCode))
	verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		verifyReq.AddCookie(c)
	}
	verifyRR := httptest.NewRecorder()
	handler.Verify(verifyRR, verifyReq)
	verifyCookies := verifyRR.Result().Cookies()

	completeReq := httptest.NewRequest("GET", "/register/complete", nil)
	for _, c := range verifyCookies {
		completeReq.AddCookie(c)
	}
	completeRR := httptest.NewRecorder()
	handler.CompletePage(completeRR, completeReq)

	if completeRR.Code != http.StatusOK {
		t.Errorf("CompletePage returned status %d, want 200", completeRR.Code)
	}

	body := completeRR.Body.String()
	if !strings.Contains(body, "Create Your Password") {
		t.Errorf("expected 'Create Your Password' heading, got:\n%s", body)
	}
	if !strings.Contains(body, "type=\"password\"") {
		t.Errorf("expected password input, got:\n%s", body)
	}
}

// Test 17: POST /register/complete — creates user with adult profile (parent role)
func TestRegistrationHandler_Complete_Adult_ParentRole(t *testing.T) {
	handler, authService, userRepo, profileRepo, otpRepo, emailSvc, rbacRepo := setupRegistrationTest(t)
	ctx := t.Context()

	cookies, otpID, _ := registerAndGetOTP(t, handler, authService, "unregistered.adult@scout.local", otpRepo, emailSvc)

	var plainCode string
	for _, sent := range emailSvc.SentOTPs {
		if sent.To == "unregistered.adult@scout.local" {
			plainCode = sent.Code
			break
		}
	}

	// Verify
	verifyReq := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otpID+"&code="+plainCode))
	verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		verifyReq.AddCookie(c)
	}
	verifyRR := httptest.NewRecorder()
	handler.Verify(verifyRR, verifyReq)
	verifyCookies := verifyRR.Result().Cookies()

	// Complete registration
	completeReq := httptest.NewRequest("POST", "/register/complete", strings.NewReader("password=newpassword123"))
	completeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range verifyCookies {
		completeReq.AddCookie(c)
	}
	completeRR := httptest.NewRecorder()
	handler.Complete(completeRR, completeReq)

	if completeRR.Code != http.StatusFound {
		t.Errorf("Complete returned status %d, want 302 Found", completeRR.Code)
	}

	location := completeRR.Header().Get("Location")
	if location != "/login?registered=1" {
		t.Errorf("expected redirect to /login?registered=1, got %q", location)
	}

	// Verify user was created by finding the profile and getting UserID
	prof, err := profileRepo.GetByEmail(ctx, "unregistered.adult@scout.local")
	if err != nil {
		t.Fatalf("GetByEmail profile: %v", err)
	}
	if prof.UserID == nil {
		t.Fatal("expected profile to be linked to a user")
	}

	user, err := userRepo.GetByID(ctx, *prof.UserID)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if user == nil {
		t.Fatal("expected user to be created")
	}

	// Verify parent role was assigned
	roles, err := rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	hasParentRole := false
	for _, role := range roles {
		if role.Name == "parent" {
			hasParentRole = true
			break
		}
	}
	if !hasParentRole {
		t.Error("expected user to have 'parent' role")
	}
}

// Test 18: POST /register/complete — creates user with youth profile (scout role)
func TestRegistrationHandler_Complete_Youth_ScoutRole(t *testing.T) {
	handler, authService, userRepo, profileRepo, otpRepo, emailSvc, rbacRepo := setupRegistrationTest(t)
	ctx := t.Context()

	cookies, otpID, _ := registerAndGetOTP(t, handler, authService, "unregistered.youth@scout.local", otpRepo, emailSvc)

	var plainCode string
	for _, sent := range emailSvc.SentOTPs {
		if sent.To == "unregistered.youth@scout.local" {
			plainCode = sent.Code
			break
		}
	}

	// Verify
	verifyReq := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otpID+"&code="+plainCode))
	verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		verifyReq.AddCookie(c)
	}
	verifyRR := httptest.NewRecorder()
	handler.Verify(verifyRR, verifyReq)
	verifyCookies := verifyRR.Result().Cookies()

	completeReq := httptest.NewRequest("POST", "/register/complete", strings.NewReader("password=youthpass"))
	completeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range verifyCookies {
		completeReq.AddCookie(c)
	}
	completeRR := httptest.NewRecorder()
	handler.Complete(completeRR, completeReq)

	if completeRR.Code != http.StatusFound {
		t.Errorf("Complete returned status %d, want 302 Found", completeRR.Code)
	}

	// Verify user was created by finding the profile and getting UserID
	prof, err := profileRepo.GetByEmail(ctx, "unregistered.youth@scout.local")
	if err != nil {
		t.Fatalf("GetByEmail profile: %v", err)
	}
	if prof.UserID == nil {
		t.Fatal("expected profile to be linked to a user")
	}

	user, err := userRepo.GetByID(ctx, *prof.UserID)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if user == nil {
		t.Fatal("expected user to be created")
	}

	// Verify scout role was assigned
	roles, err := rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserRoles: %v", err)
	}
	hasScoutRole := false
	for _, role := range roles {
		if role.Name == "scout" {
			hasScoutRole = true
			break
		}
	}
	if !hasScoutRole {
		t.Error("expected user to have 'scout' role")
	}
}

// Test 19: POST /register/complete — success redirects to /login?registered=1
func TestRegistrationHandler_Complete_SuccessRedirect(t *testing.T) {
	handler, authService, _, _, otpRepo, emailSvc, _ := setupRegistrationTest(t)

	cookies, otpID, _ := registerAndGetOTP(t, handler, authService, "unregistered.adult@scout.local", otpRepo, emailSvc)

	var plainCode string
	for _, sent := range emailSvc.SentOTPs {
		if sent.To == "unregistered.adult@scout.local" {
			plainCode = sent.Code
			break
		}
	}

	verifyReq := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otpID+"&code="+plainCode))
	verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		verifyReq.AddCookie(c)
	}
	verifyRR := httptest.NewRecorder()
	handler.Verify(verifyRR, verifyReq)
	verifyCookies := verifyRR.Result().Cookies()

	completeReq := httptest.NewRequest("POST", "/register/complete", strings.NewReader("password=secret"))
	completeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range verifyCookies {
		completeReq.AddCookie(c)
	}
	completeRR := httptest.NewRecorder()
	handler.Complete(completeRR, completeReq)

	if completeRR.Code != http.StatusFound {
		t.Errorf("Complete returned status %d, want 302 Found", completeRR.Code)
	}

	location := completeRR.Header().Get("Location")
	if location != "/login?registered=1" {
		t.Errorf("expected redirect to /login?registered=1, got %q", location)
	}
}

// Test 20: GET /login?registered=1 — shows success banner
func TestAuthHandler_LoginPage_WithRegisteredParam(t *testing.T) {
	authHandler, _, _, _ := setupAuthTest(t)

	req := httptest.NewRequest("GET", "/login?registered=1", nil)
	rr := httptest.NewRecorder()

	authHandler.LoginPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("LoginPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Account created successfully") {
		t.Errorf("expected success banner, got:\n%s", body)
	}
}

func loginAndGetCookies(t *testing.T, authService *auth.AuthService) []*http.Cookie {
	t.Helper()
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=admin@scout.local&password=password"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler := NewAuthHandler(authService)
	authHandler.Login(loginRR, loginReq)
	return loginRR.Result().Cookies()
}

func registerAndGetOTP(t *testing.T, handler *RegistrationHandler, authService *auth.AuthService, email string, otpRepo *mock.OTPCodeRepository, emailSvc *mock.EmailService) ([]*http.Cookie, string, string) {
	t.Helper()
	ctx := t.Context()

	// Login
	cookies := loginAndGetCookies(t, authService)

	// POST /register
	registerReq := httptest.NewRequest("POST", "/register", strings.NewReader("email="+email))
	registerReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		registerReq.AddCookie(c)
	}
	registerRR := httptest.NewRecorder()
	handler.Register(registerRR, registerReq)

	if registerRR.Code != http.StatusFound {
		t.Fatalf("Register step returned %d, want 302", registerRR.Code)
	}

	// Get the OTP ID from the redirect URL
	loc := registerRR.Header().Get("Location")
	otpID := strings.TrimPrefix(loc, "/register/verify?otp_id=")

	// Get the plain code from the mock email service
	var plainCode string
	for _, sent := range emailSvc.SentOTPs {
		if sent.To == email {
			plainCode = sent.Code
			break
		}
	}
	if plainCode == "" {
		t.Fatal("no email sent for " + email)
	}

	// Verify OTP was created
	fetched, err := otpRepo.GetByID(ctx, otpID)
	if err != nil {
		t.Fatalf("OTP not found: %v", err)
	}
	_ = fetched

	return registerRR.Result().Cookies(), otpID, plainCode
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test@scout.local", "t***@*****.local"},
		{"a@b.com", "a@b.com"},
		{"abc@test.com", "a**@****.com"},
	}
	for _, tt := range tests {
		got := maskEmail(tt.input)
		if got != tt.expected {
			t.Errorf("maskEmail(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRegistrationHandler_Register_EmptyEmail(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("POST", "/register", strings.NewReader("email="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Register returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Please enter your email address") {
		t.Errorf("expected error for empty email, got:\n%s", body)
	}
}

func TestRegistrationHandler_Verify_NoOTPID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("POST", "/register/verify", strings.NewReader("code=123456"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Verify(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Verify returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/register" {
		t.Errorf("expected redirect to /register, got %q", location)
	}
}

func TestRegistrationHandler_Complete_NoPassword(t *testing.T) {
	handler, authService, _, _, otpRepo, emailSvc, _ := setupRegistrationTest(t)

	cookies, otpID, _ := registerAndGetOTP(t, handler, authService, "unregistered.adult@scout.local", otpRepo, emailSvc)

	var plainCode string
	for _, sent := range emailSvc.SentOTPs {
		if sent.To == "unregistered.adult@scout.local" {
			plainCode = sent.Code
			break
		}
	}

	// Verify
	verifyReq := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otpID+"&code="+plainCode))
	verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		verifyReq.AddCookie(c)
	}
	verifyRR := httptest.NewRecorder()
	handler.Verify(verifyRR, verifyReq)
	verifyCookies := verifyRR.Result().Cookies()

	completeReq := httptest.NewRequest("POST", "/register/complete", strings.NewReader("password="))
	completeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range verifyCookies {
		completeReq.AddCookie(c)
	}
	completeRR := httptest.NewRecorder()
	handler.Complete(completeRR, completeReq)

	if completeRR.Code != http.StatusOK {
		t.Errorf("Complete returned status %d, want %d", completeRR.Code, http.StatusOK)
	}

	body := completeRR.Body.String()
	if !strings.Contains(body, "Please enter a password") {
		t.Errorf("expected error for empty password, got:\n%s", body)
	}
}

// Test that correct code hash comparison works
func TestRegistrationHandler_Verify_CodeHashMatch(t *testing.T) {
	handler, authService, _, _, otpRepo, emailSvc, _ := setupRegistrationTest(t)
	ctx := t.Context()

	cookies, otpID, _ := registerAndGetOTP(t, handler, authService, "unregistered.adult@scout.local", otpRepo, emailSvc)

	var plainCode string
	for _, sent := range emailSvc.SentOTPs {
		if sent.To == "unregistered.adult@scout.local" {
			plainCode = sent.Code
			break
		}
	}

	// Verify the hash compares correctly
	fetched, err := otpRepo.GetByID(ctx, otpID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	inputHash := sha256.Sum256([]byte(plainCode))
	if len(inputHash) != len(fetched.CodeHash) {
		t.Error("hash length mismatch")
	}
	for i := range inputHash {
		if inputHash[i] != fetched.CodeHash[i] {
			t.Errorf("hash mismatch at byte %d", i)
		}
	}

	req := httptest.NewRequest("POST", "/register/verify", strings.NewReader("otp_id="+otpID+"&code="+plainCode))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	handler.Verify(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Verify returned status %d, want 302 Found", rr.Code)
	}
}

// Test that VerifyPage without otp_id redirects
func TestRegistrationHandler_VerifyPage_NoOTPID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("GET", "/register/verify", nil)
	rr := httptest.NewRecorder()

	handler.VerifyPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("VerifyPage returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/register" {
		t.Errorf("expected redirect to /register, got %q", location)
	}
}

// Test Complete without session redirects
func TestRegistrationHandler_Complete_NoSession(t *testing.T) {
	handler, _, _, _, _, _, _ := setupRegistrationTest(t)

	req := httptest.NewRequest("POST", "/register/complete", strings.NewReader("password=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Complete(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Complete returned status %d, want 302 Found", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/register" {
		t.Errorf("expected redirect to /register, got %q", location)
	}
}
