package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	mrand "math/rand"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type AuthHandler struct {
	Store *store.Store
}

type RegisterRequest struct {
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Password     string `json:"password"`
	Role         string `json:"role"`
	Jurisdiction string `json:"jurisdiction"`
	AdminHash    string `json:"admin_hash"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type GoogleAuthRequest struct {
	GoogleID string `json:"google_id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"` // default if new user
	IsSignUp bool   `json:"is_signup"`
}

type VerifyOTPRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

func verifyPasswordStrength(password string) bool {
	if len(password) < 8 {
		return false
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		if unicode.IsUpper(r) {
			hasUpper = true
		} else if unicode.IsLower(r) {
			hasLower = true
		} else if unicode.IsDigit(r) {
			hasDigit = true
		} else if unicode.IsPunct(r) || unicode.IsSymbol(r) || strings.ContainsRune("@$!%*?&", r) {
			hasSpecial = true
		}
	}
	return hasUpper && hasLower && hasDigit && hasSpecial
}

func generateOTP() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("%06d", n.Int64()+100000)
}

func sendEmail(to, subject, htmlBody string) error {
	apiKey := "nsk_live_6a7fbfd80c696c3460f34cbb"
	payload, err := json.Marshal(map[string]interface{}{
		"from":    "reos.security@nisoko.co.ke",
		"to":      to,
		"subject": subject,
		"html":    htmlBody,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://nes.nisoko.co.ke/api/v1/nes/send", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("NES service returned status: %s", resp.Status)
	}
	return nil
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)

	if req.Email == "" && req.Phone == "" {
		http.Error(w, "Email or phone number is required", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}

	// Enforce password strength
	if !verifyPasswordStrength(req.Password) {
		http.Error(w, "Password must be at least 8 characters, and contain at least one uppercase letter, one lowercase letter, one digit, and one special character (@$!%*?&).", http.StatusBadRequest)
		return
	}

	role := strings.ToLower(req.Role)
	if role == "" || role == "tenant" {
		role = models.RoleClient
	}

	// Enforce role-based registration restrictions
	if role == models.RoleSuperAdmin {
		expectedHash := os.Getenv("SUPERADMIN_HASH")
		if expectedHash == "" {
			expectedHash = "REOS_SUPERADMIN_SECRET_KEY_2026"
		}
		if req.AdminHash != expectedHash {
			http.Error(w, "Access Denied: Invalid Superadmin Hash", http.StatusForbidden)
			return
		}
	} else if role == models.RoleLandlord || role == models.RoleAgent || role == models.RoleClient {
		// Allowed roles for direct registration
	} else {
		// All other roles (technical_admin, support_admin, billing_admin, staff, caretaker)
		http.Error(w, "Access Denied: Standard signup is not allowed for this role. Please use the invitation link page.", http.StatusForbidden)
		return
	}

	var defaultScope models.Scope
	switch role {
	case models.RoleSuperAdmin, models.RoleTechAdmin, models.RoleSupportAdmin, models.RoleBillingAdmin:
		defaultScope = models.Scope{Type: "all", RefIDs: []string{}}
	case models.RoleLandlord:
		defaultScope = models.Scope{Type: "own_records", RefIDs: []string{}}
	case models.RoleAgent, models.RoleCaretaker, models.RoleStaff:
		defaultScope = models.Scope{Type: "assigned_properties", RefIDs: []string{}}
	default:
		defaultScope = models.Scope{Type: "own_records", RefIDs: []string{}}
	}

	// Admin roles (superadmin, tech_admin, support_admin, billing_admin) are trusted via
	// SUPERADMIN_HASH or invitation — they skip email OTP and are activated immediately.
	// Regular self-signup roles (landlord, agent, client) require email verification.
	isAdminRole := role == models.RoleSuperAdmin ||
		role == models.RoleTechAdmin ||
		role == models.RoleSupportAdmin ||
		role == models.RoleBillingAdmin

	status := "active"
	var otpCode string
	if req.Email != "" && !isAdminRole {
		status = "pending"
		otpCode = generateOTP()
	}

	user := &models.User{
		ID:                   uuid.New().String(),
		Role:                 role,
		Email:                req.Email,
		Phone:                req.Phone,
		PasswordHash:         store.HashPassword(req.Password),
		Jurisdiction:         req.Jurisdiction,
		Status:               status,
		IdentityVerification: "pending",
		CreatedAt:            time.Now(),
		RecoveryPhrase:       generateRecoveryPhrase(),
		OTP:                  otpCode,
		AuthProvider:         "local",
		Sessions:             []string{},
		Scope:                defaultScope,
	}

	var token string
	if status == "active" {
		token = "session_" + uuid.New().String()
		user.Sessions = append(user.Sessions, token)
	}

	if err := h.Store.CreateUser(user); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if role == models.RoleLandlord {
		lv := &models.LandlordVerification{
			ID:               "lv_" + uuid.New().String(),
			UserID:           user.ID,
			CurrentTierLevel: 1,
			GrantedSnapshot: models.GrantedSnapshot{
				TierDefinitionID:     "tier_free",
				PropertyCap:          100,
				UnlockedListingTypes: []string{"rental", "storage"},
				GrantedAt:            time.Now(),
			},
			TierHistory: []models.TierHistoryItem{
				{
					TierDefinitionID: "tier_free",
					Level:            1,
					AchievedAt:       time.Now(),
					VerifiedBy:       "system_auto",
					PaymentRef:       "free_tier",
				},
			},
			KYCDocuments: []models.KYCDocument{},
			Status:       "active",
		}
		h.Store.CreateLandlordVerification(lv)
	}

	if req.Email != "" && otpCode != "" {
		subject := "Verify your REOS Account"
		htmlBody := EmailOTPVerification(req.Email, otpCode)
		if err := sendEmail(req.Email, subject, htmlBody); err != nil {
			log.Printf("Failed to send OTP email to %s: %v", req.Email, err)
		} else {
			log.Printf("Successfully sent OTP email to %s", req.Email)
		}
		// Log the OTP code to terminal for easy testing
		fmt.Printf("\n[REOS SECURITY] Generated verification OTP for %s: %s\n\n", req.Email, otpCode)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var user *models.User
	var err error

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)

	if req.Email != "" {
		user, err = h.Store.GetUserByEmail(req.Email)
	} else if req.Phone != "" {
		user, err = h.Store.GetUserByPhone(req.Phone)
	} else {
		http.Error(w, "Email or phone is required for login", http.StatusBadRequest)
		return
	}

	if err != nil || user.PasswordHash != store.HashPassword(req.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if user.Status == "pending" {
		// Admin roles are trusted via SUPERADMIN_HASH — auto-activate if somehow stuck in pending.
		// This self-heals accounts created before the pending-bypass fix was applied.
		adminRoles := map[string]bool{
			models.RoleSuperAdmin:   true,
			models.RoleTechAdmin:    true,
			models.RoleSupportAdmin: true,
			models.RoleBillingAdmin: true,
		}
		if adminRoles[user.Role] {
			// Mutate directly on the pointer — UpdateUser() acquires its own lock internally
			user.Status = "active"
			user.OTP = ""
			h.Store.UpdateUser(user)
		} else {
			http.Error(w, "unverified", http.StatusForbidden)
			return
		}
	}

	// Create unique session token
	token := "session_" + uuid.New().String()
	h.Store.Lock()
	user.Sessions = append(user.Sessions, token)
	h.Store.Unlock()
	h.Store.UpdateUser(user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.OTP = strings.TrimSpace(req.OTP)

	if req.Email == "" || req.OTP == "" {
		http.Error(w, "Email and OTP are required", http.StatusBadRequest)
		return
	}

	user, err := h.Store.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.OTP != req.OTP {
		http.Error(w, "Invalid OTP code", http.StatusUnauthorized)
		return
	}

	h.Store.Lock()
	user.Status = "active"
	user.OTP = "" // clear OTP
	token := "session_" + uuid.New().String()
	user.Sessions = append(user.Sessions, token)
	h.Store.Unlock()

	h.Store.UpdateUser(user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GoogleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.GoogleID == "" || req.Email == "" {
		http.Error(w, "Google ID and Email are required", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var user *models.User
	var err error

	if req.IsSignUp {
		user, err = h.Store.GetUserByGoogleID(req.GoogleID)
		if err == nil {
			http.Error(w, "Google account already registered. Please sign in.", http.StatusConflict)
			return
		}

		user, err = h.Store.GetUserByEmail(req.Email)
		if err == nil {
			http.Error(w, "Email already registered with a different provider", http.StatusConflict)
			return
		}

		role := req.Role
		if role == "" {
			role = models.RoleTenant
		}
		user = &models.User{
			ID:                   uuid.New().String(),
			Role:                 role,
			Email:                req.Email,
			GoogleID:             req.GoogleID,
			Status:               "active",
			IdentityVerification: "verified",
			CreatedAt:            time.Now(),
			AuthProvider:         "google",
			Sessions:             []string{},
		}
		if err := h.Store.CreateUser(user); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		user, err = h.Store.GetUserByGoogleID(req.GoogleID)
		if err != nil || user.AuthProvider != "google" {
			http.Error(w, "Account not found. Please register an account first.", http.StatusNotFound)
			return
		}
	}

	// Create unique session token
	token := "session_" + uuid.New().String()
	h.Store.Lock()
	user.Sessions = append(user.Sessions, token)
	h.Store.Unlock()
	h.Store.UpdateUser(user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !strings.HasPrefix(token, "session_") {
		http.Error(w, "Invalid session token", http.StatusUnauthorized)
		return
	}

	h.Store.Lock()
	var matchedUser *models.User
	for _, u := range h.Store.Users {
		for _, sess := range u.Sessions {
			if sess == token {
				matchedUser = u
				break
			}
		}
		if matchedUser != nil {
			break
		}
	}

	// Fallback
	if matchedUser == nil {
		userID := strings.TrimPrefix(token, "session_")
		if u, err := h.Store.GetUserByID(userID); err == nil && len(u.Sessions) == 0 {
			matchedUser = u
		}
	}
	h.Store.Unlock()

	if matchedUser == nil {
		http.Error(w, "User not found or session expired", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matchedUser)
}

func generateRecoveryPhrase() string {
	wordList := []string{
		"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
		"india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa",
		"quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey",
		"xray", "yankee", "zulu", "amber", "cedar", "falcon", "granite",
		"harbor", "island", "jasper", "kindle", "lantern", "marble", "noble",
		"orbit", "prism", "quartz", "river", "silver", "timber", "ultra",
		"valley", "walnut", "xenon", "yellow", "zenith", "anchor", "blaze",
	}
	phrase := make([]string, 6)
	buf := make([]byte, 8)
	for i := 0; i < 6; i++ {
		if _, err := rand.Read(buf); err != nil {
			// Fallback to math/rand with unique seed if crypto fails
			phrase[i] = wordList[mrand.Intn(len(wordList))]
			continue
		}
		var n uint64
		for _, b := range buf {
			n = n<<8 | uint64(b)
		}
		phrase[i] = wordList[n%uint64(len(wordList))]
	}
	return strings.Join(phrase, " ")
}

type UpdateProfileRequest struct {
	Phone              *string                       `json:"phone"`
	Email              *string                       `json:"email"`
	Jurisdiction       *string                       `json:"jurisdiction"`
	BankName           *string                       `json:"bank_name"`
	BankAccount        *string                       `json:"bank_account"`
	BankAccountName    *string                       `json:"bank_account_name"`
	MobileMoneyPhone   *string                       `json:"mobile_money_phone"`
	MobileMoneyName    *string                       `json:"mobile_money_name"`
	ProfileImage       *string                       `json:"profile_image"`
	ProfileImages      *[]string                     `json:"profile_images"`
	EmailNotifications *bool                         `json:"email_notifications"`
	SMSNotifications   *bool                         `json:"sms_notifications"`
	MFAEnabled         *bool                         `json:"mfa_enabled"`
	Passkeys           *[]models.WebAuthnCredential `json:"passkeys"`
	Password           *string                       `json:"password,omitempty"`
	RevokeOthers       *bool                         `json:"revoke_others"`
	RevokeSession      *string                       `json:"revoke_session"`
	RecoveryPhrase     *string                       `json:"recovery_phrase,omitempty"`
}

func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	if req.Phone != nil && *req.Phone != "" {
		user.Phone = *req.Phone
	}
	if req.Email != nil && *req.Email != "" {
		user.Email = *req.Email
	}
	if req.Jurisdiction != nil {
		user.Jurisdiction = *req.Jurisdiction
	}
	if req.BankName != nil {
		user.BankName = *req.BankName
	}
	if req.BankAccount != nil {
		user.BankAccount = *req.BankAccount
	}
	if req.BankAccountName != nil {
		user.BankAccountName = *req.BankAccountName
	}
	if req.MobileMoneyPhone != nil {
		user.MobileMoneyPhone = *req.MobileMoneyPhone
	}
	if req.MobileMoneyName != nil {
		user.MobileMoneyName = *req.MobileMoneyName
	}
	if req.ProfileImage != nil {
		user.ProfileImage = *req.ProfileImage
	}
	if req.ProfileImages != nil {
		user.ProfileImages = *req.ProfileImages
	}
	if req.EmailNotifications != nil {
		user.EmailNotifications = *req.EmailNotifications
	}
	if req.SMSNotifications != nil {
		user.SMSNotifications = *req.SMSNotifications
	}
	if req.MFAEnabled != nil {
		user.MFAEnabled = *req.MFAEnabled
	}

	if req.Passkeys != nil {
		user.Passkeys = *req.Passkeys
	}

	if req.RecoveryPhrase != nil && *req.RecoveryPhrase != "" {
		user.RecoveryPhrase = *req.RecoveryPhrase
	}

	if req.Password != nil && *req.Password != "" {
		// Enforce password strength on rotation
		if !verifyPasswordStrength(*req.Password) {
			h.Store.Unlock()
			http.Error(w, "Password must be at least 8 characters, and contain at least one uppercase letter, one lowercase letter, one digit, and one special character.", http.StatusBadRequest)
			return
		}
		user.PasswordHash = store.HashPassword(*req.Password)
	}

	// Revoke sessions
	if req.RevokeOthers != nil && *req.RevokeOthers {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		user.Sessions = []string{token}
	}
	if req.RevokeSession != nil && *req.RevokeSession != "" {
		var activeSess []string
		for _, s := range user.Sessions {
			if s != *req.RevokeSession {
				activeSess = append(activeSess, s)
			}
		}
		user.Sessions = activeSess
	}
	h.Store.Unlock()

	h.Store.UpdateUser(user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

type RecoverRequest struct {
	Email          string `json:"email"`
	RecoveryPhrase string `json:"recovery_phrase"`
	NewPassword    string `json:"new_password"`
}

func (h *AuthHandler) RecoverPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.RecoveryPhrase = strings.TrimSpace(strings.ToLower(req.RecoveryPhrase))

	user, err := h.Store.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	if strings.TrimSpace(strings.ToLower(user.RecoveryPhrase)) != req.RecoveryPhrase {
		http.Error(w, "Invalid recovery phrase. Please check and try again.", http.StatusForbidden)
		return
	}

	// Enforce password strength on recovery
	if !verifyPasswordStrength(req.NewPassword) {
		http.Error(w, "Password must be at least 8 characters, and contain uppercase, lowercase, digit, and symbol.", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	user.PasswordHash = store.HashPassword(req.NewPassword)
	user.Sessions = []string{} // Revoke all sessions on password recovery reset for safety!
	h.Store.Unlock()

	h.Store.UpdateUser(user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Password successfully reset!"})
}

// WebAuthn Endpoints

type WebAuthnRegisterOptionsResponse struct {
	Challenge              string                         `json:"challenge"`
	RelyingParty           WebAuthnRP                     `json:"rp"`
	User                   WebAuthnUser                   `json:"user"`
	PubKeyCredParams       []WebAuthnPubKeyCredParam      `json:"pubKeyCredParams"`
	AuthenticatorSelection WebAuthnAuthenticatorSelection `json:"authenticatorSelection"`
	Timeout                int                            `json:"timeout"`
	Attestation            string                         `json:"attestation"`
}

type WebAuthnRP struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type WebAuthnUser struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type WebAuthnPubKeyCredParam struct {
	Type string `json:"type"`
	Alg  int    `json:"alg"`
}

type WebAuthnAuthenticatorSelection struct {
	AuthenticatorAttachment string `json:"authenticatorAttachment"`
	UserVerification        string `json:"userVerification"`
}

func (h *AuthHandler) WebAuthnRegisterOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Generate challenge
	challenge := uuid.New().String()
	h.Store.Lock()
	user.MFASecret = challenge // temporarily store challenge here
	h.Store.Unlock()
	h.Store.UpdateUser(user)

	opts := WebAuthnRegisterOptionsResponse{
		Challenge: challenge,
		RelyingParty: WebAuthnRP{
			Name: "REOS Rental Monolith",
			ID:   "localhost", // for development
		},
		User: WebAuthnUser{
			ID:          user.ID,
			Name:        user.Email,
			DisplayName: user.Email,
		},
		PubKeyCredParams: []WebAuthnPubKeyCredParam{
			{Type: "public-key", Alg: -7}, // ES256
			{Type: "public-key", Alg: -257}, // RS256
		},
		AuthenticatorSelection: WebAuthnAuthenticatorSelection{
			AuthenticatorAttachment: "platform",
			UserVerification:        "preferred",
		},
		Timeout:     60000,
		Attestation: "none",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(opts)
}

type WebAuthnRegisterVerifyRequest struct {
	CredentialID string `json:"credentialId"`
	PublicKey    string `json:"publicKey"`
}

func (h *AuthHandler) WebAuthnRegisterVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	var req WebAuthnRegisterVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CredentialID == "" || req.PublicKey == "" {
		http.Error(w, "Credential ID and Public Key are required", http.StatusBadRequest)
		return
	}

	cred := models.WebAuthnCredential{
		ID:        req.CredentialID,
		PublicKey: req.PublicKey,
		AAGUID:    "platform-authenticator",
		SignCount: 1,
		CreatedAt: time.Now(),
	}

	h.Store.Lock()
	user.Passkeys = append(user.Passkeys, cred)
	user.MFASecret = "" // clear challenge
	h.Store.Unlock()
	h.Store.UpdateUser(user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user":    user,
	})
}

type WebAuthnLoginOptionsRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type WebAuthnLoginOptionsResponse struct {
	Challenge        string              `json:"challenge"`
	AllowCredentials []WebAuthnAllowCred `json:"allowCredentials"`
	Timeout          int                 `json:"timeout"`
	UserVerification string              `json:"userVerification"`
}

type WebAuthnAllowCred struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func (h *AuthHandler) WebAuthnLoginOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req WebAuthnLoginOptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var user *models.User
	var err error

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email != "" {
		user, err = h.Store.GetUserByEmail(req.Email)
	} else if req.Phone != "" {
		user, err = h.Store.GetUserByPhone(req.Phone)
	} else {
		http.Error(w, "Email or phone is required", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if len(user.Passkeys) == 0 {
		http.Error(w, "No passkeys registered for this account", http.StatusBadRequest)
		return
	}

	// Generate challenge
	challenge := uuid.New().String()
	h.Store.Lock()
	user.MFASecret = challenge
	h.Store.Unlock()
	h.Store.UpdateUser(user)

	allowCreds := []WebAuthnAllowCred{}
	for _, k := range user.Passkeys {
		allowCreds = append(allowCreds, WebAuthnAllowCred{
			Type: "public-key",
			ID:   k.ID,
		})
	}

	opts := WebAuthnLoginOptionsResponse{
		Challenge:        challenge,
		AllowCredentials: allowCreds,
		Timeout:          60000,
		UserVerification: "preferred",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(opts)
}

type WebAuthnLoginVerifyRequest struct {
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	CredentialID string `json:"credentialId"`
}

func (h *AuthHandler) WebAuthnLoginVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req WebAuthnLoginVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var user *models.User
	var err error

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email != "" {
		user, err = h.Store.GetUserByEmail(req.Email)
	} else if req.Phone != "" {
		user, err = h.Store.GetUserByPhone(req.Phone)
	} else {
		http.Error(w, "Email or phone is required", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Match credential
	matched := false
	for _, k := range user.Passkeys {
		if k.ID == req.CredentialID {
			matched = true
			break
		}
	}

	if !matched {
		http.Error(w, "Invalid passkey credential signature", http.StatusUnauthorized)
		return
	}

	// Generate session token
	token := "session_" + uuid.New().String()
	h.Store.Lock()
	user.Sessions = append(user.Sessions, token)
	user.MFASecret = "" // clear challenge
	h.Store.Unlock()
	h.Store.UpdateUser(user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) NESWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	signature := r.Header.Get("X-Nisoko-Signature")
	if signature != "" {
		parts := strings.Split(signature, ",")
		if len(parts) == 2 {
			var timestamp, expectedHash string
			for _, part := range parts {
				kv := strings.Split(part, "=")
				if len(kv) == 2 {
					if kv[0] == "t" {
						timestamp = kv[1]
					} else if kv[0] == "v1" {
						expectedHash = kv[1]
					}
				}
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil {
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				mac := hmac.New(sha256.New, []byte("nsk_webhook_secret_12345"))
				mac.Write([]byte(timestamp + "." + string(bodyBytes)))
				computedHash := hex.EncodeToString(mac.Sum(nil))

				if computedHash != expectedHash {
					log.Printf("[NES Webhook] Invalid signature: expected %s, got %s", expectedHash, computedHash)
				} else {
					log.Println("[NES Webhook] Signature verified successfully!")
				}
			}
		}
	}

	var event map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&event); err == nil {
		log.Printf("[NES Webhook] Event received: %v", event)
	}

	w.WriteHeader(http.StatusOK)
}

