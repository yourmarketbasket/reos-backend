package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

func appBaseURL() string {
	if u := os.Getenv("APP_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:5173"
}

type InvitationsHandler struct {
	Store *store.Store
}

type InviteRequest struct {
	Email      string `json:"email"`
	Role       string `json:"role"` // tenant, caretaker, agent
	PropertyID string `json:"property_id"`
	UnitID     string `json:"unit_id,omitempty"`
}

type AcceptInviteRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email"`
	GoogleID string `json:"google_id,omitempty"`
}

func (h *InvitationsHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simple authorization: get sender user ID from token
	landlordID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Resolve the sender's role for permission checks
	h.Store.RLock()
	sender, senderExists := h.Store.Users[landlordID]
	h.Store.RUnlock()
	if !senderExists {
		http.Error(w, "Sender user not found", http.StatusUnauthorized)
		return
	}
	senderRole := sender.Role

	var req InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Role == "" {
		http.Error(w, "Email and Role are required", http.StatusBadRequest)
		return
	}

	role := strings.ToLower(req.Role)
	isAdminRole := role == models.RoleSuperAdmin || role == models.RoleTechAdmin || role == models.RoleSupportAdmin || role == models.RoleBillingAdmin
	isSenderAdmin := senderRole == models.RoleSuperAdmin || senderRole == models.RoleTechAdmin || senderRole == models.RoleSupportAdmin || senderRole == models.RoleBillingAdmin

	// Permission guard: only platform admins can invite other admin roles
	if isAdminRole && !isSenderAdmin {
		http.Error(w, "Forbidden: only platform administrators can invite admin roles", http.StatusForbidden)
		return
	}
	// Landlords and agents can only invite: tenant, caretaker, staff
	if !isSenderAdmin {
		operationalRoles := map[string]bool{
			models.RoleTenant:    true,
			models.RoleCaretaker: true,
			models.RoleStaff:     true,
		}
		if !operationalRoles[role] {
			http.Error(w, "Forbidden: you can only invite tenants, caretakers, or staff members", http.StatusForbidden)
			return
		}
	}

	if !isAdminRole && req.PropertyID == "" {
		http.Error(w, "PropertyID is required", http.StatusBadRequest)
		return
	}

	var propName = "System Platform"
	if !isAdminRole {
		// Verify property exists
		h.Store.RLock()
		prop, ok := h.Store.Properties[req.PropertyID]
		h.Store.RUnlock()
		if !ok {
			http.Error(w, "Property not found", http.StatusNotFound)
			return
		}
		propName = prop.Name
	}

	// Check 1: reject if email already belongs to a registered user
	h.Store.RLock()
	for _, u := range h.Store.Users {
		if strings.EqualFold(u.Email, req.Email) {
			h.Store.RUnlock()
			http.Error(w, "This email address is already registered on the platform", http.StatusConflict)
			return
		}
	}
	// Check 2: reject if a pending invitation already exists for this email
	for _, existing := range h.Store.Invitations {
		if strings.EqualFold(existing.Email, req.Email) && existing.Status == models.InvitationStatusPending {
			h.Store.RUnlock()
			http.Error(w, "A pending invitation already exists for this email address", http.StatusConflict)
			return
		}
	}
	h.Store.RUnlock()

	token := uuid.New().String()
	inv := &models.Invitation{
		ID:         uuid.New().String(),
		Token:      token,
		Email:      req.Email,
		SenderID:   landlordID,
		PropertyID: req.PropertyID,
		UnitID:     req.UnitID,
		Role:       role,
		Status:     models.InvitationStatusPending,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(7 * 24 * time.Hour), // 7 days expiration
	}

	h.Store.CreateInvitation(inv)

	// Build the acceptance link and send the invitation email
	inviteLink := fmt.Sprintf("%s/invite/accept?token=%s", appBaseURL(), token)

	// Resolve sender display name
	h.Store.RLock()
	sender, senderOk := h.Store.Users[landlordID]
	h.Store.RUnlock()
	senderName := req.Email // fallback
	if senderOk && sender.Email != "" {
		senderName = sender.Email
	}

	emailHTML := EmailInvitation(req.Email, senderName, propName, req.Role, inviteLink)
	if err := sendEmail(req.Email, "You're invited to join REOS", emailHTML); err != nil {
		log.Printf("[REOS] Failed to send invitation email to %s: %v", req.Email, err)
	} else {
		log.Printf("[REOS] Invitation email sent to %s (token: %s)", req.Email, token)
	}
	// Always print link to terminal for easy dev testing
	fmt.Printf("\n[REOS INVITE] %s → %s\n\n", req.Email, inviteLink)

	// Log SMS notification stub
	smsLog := &models.SMSNotification{
		ID:              uuid.New().String(),
		UserID:          "",
		Phone:           "+254700000000",
		TemplateType:    "invitation",
		LinkedEntityRef: inv.ID,
		Status:          "sent",
		SentAt:          time.Now(),
	}
	h.Store.AddSMSLog(smsLog)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(inv)
}

func (h *InvitationsHandler) GetInvitationDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	inv, err := h.Store.GetInvitationByToken(token)
	if err != nil {
		http.Error(w, "Invitation not found or invalid token", http.StatusNotFound)
		return
	}

	if inv.Status != models.InvitationStatusPending {
		http.Error(w, "Invitation already used or expired", http.StatusBadRequest)
		return
	}

	if time.Now().After(inv.ExpiresAt) {
		h.Store.UpdateInvitationStatus(token, models.InvitationStatusExpired)
		http.Error(w, "Invitation has expired", http.StatusGone)
		return
	}

	// Fetch property name and landlord info for UI display
	h.Store.RLock()
	prop, _ := h.Store.Properties[inv.PropertyID]
	landlord, _ := h.Store.Users[inv.SenderID]
	var unitLabel string
	if inv.UnitID != "" {
		if u, exists := h.Store.Units[inv.UnitID]; exists {
			unitLabel = u.Label
		}
	}
	h.Store.RUnlock()

	// Safely resolve property name — may be nil for platform-level (admin) invitations
	var propName string
	if prop != nil {
		propName = prop.Name
	} else {
		propName = "System Platform"
	}

	// Safely resolve sender display — fall back to email
	var senderDisplay string
	if landlord != nil {
		senderDisplay = landlord.Email
	} else {
		senderDisplay = inv.Email
	}

	response := map[string]interface{}{
		"id":            inv.ID,
		"token":         inv.Token,
		"email":         inv.Email,
		"role":          inv.Role,
		"property_id":   inv.PropertyID,
		"property_name": propName,
		"unit_id":       inv.UnitID,
		"unit_label":    unitLabel,
		"landlord_name": senderDisplay,
		"created_at":    inv.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *InvitationsHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AcceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		http.Error(w, "Invitation token is required", http.StatusBadRequest)
		return
	}

	inv, err := h.Store.GetInvitationByToken(req.Token)
	if err != nil {
		http.Error(w, "Invitation not found", http.StatusNotFound)
		return
	}

	if inv.Status != models.InvitationStatusPending {
		http.Error(w, "Invitation is not active", http.StatusBadRequest)
		return
	}

	if time.Now().After(inv.ExpiresAt) {
		h.Store.UpdateInvitationStatus(req.Token, models.InvitationStatusExpired)
		http.Error(w, "Invitation has expired", http.StatusGone)
		return
	}

	// Sign up or login the user
	var user *models.User
	var userErr error

	// If Google auth is used
	if req.GoogleID != "" {
		user, userErr = h.Store.GetUserByGoogleID(req.GoogleID)
		if userErr != nil {
			// Check by email
			user, userErr = h.Store.GetUserByEmail(req.Email)
			if userErr == nil {
				// Link Google ID
				h.Store.Lock()
				user.GoogleID = req.GoogleID
				h.Store.Unlock()
			} else {
				// Create new user
				user = &models.User{
					ID:                   uuid.New().String(),
					Role:                 inv.Role,
					Email:                req.Email,
					GoogleID:             req.GoogleID,
					Status:               "active",
					IdentityVerification: "verified",
					CreatedAt:            time.Now(),
				}
				if err := h.Store.CreateUser(user); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
	} else {
		// Normal Email/Phone + password
		if req.Email == "" {
			http.Error(w, "Email is required to accept invitation", http.StatusBadRequest)
			return
		}
		user, userErr = h.Store.GetUserByEmail(req.Email)
		if userErr != nil {
			// Create user
			user = &models.User{
				ID:                   uuid.New().String(),
				Role:                 inv.Role,
				Email:                req.Email,
				Phone:                req.Phone,
				PasswordHash:         store.HashPassword(req.Password),
				Status:               "active",
				IdentityVerification: "pending",
				CreatedAt:            time.Now(),
			}
			if err := h.Store.CreateUser(user); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Update invitation status to accepted
	h.Store.UpdateInvitationStatus(req.Token, models.InvitationStatusAccepted)

	// Send welcome email to the newly onboarded user
	go func() {
		welcomeHTML := EmailWelcome(user.Email, inv.Role, appBaseURL()+"/dashboard")
		if err := sendEmail(user.Email, "Welcome to REOS — You're all set!", welcomeHTML); err != nil {
			log.Printf("[REOS] Failed to send welcome email to %s: %v", user.Email, err)
		} else {
			log.Printf("[REOS] Welcome email sent to %s", user.Email)
		}
	}()

	// If the invitation was for a tenant and a unit was specified, automatically create a Lease
	if inv.Role == models.RoleTenant && inv.UnitID != "" {
		h.Store.Lock()
		unit, unitExists := h.Store.Units[inv.UnitID]
		h.Store.Unlock()

		if unitExists {
			leaseID := uuid.New().String()
			lease := &models.Lease{
				ID:            leaseID,
				UnitID:        inv.UnitID,
				TenantID:      user.ID,
				LandlordID:    inv.SenderID,
				StartDate:     time.Now(),
				EndDate:       time.Now().Add(365 * 24 * time.Hour),
				RentAmount:    unit.RentAmount,
				DepositAmount: unit.DepositAmount,
				Status:        models.LeaseStatusActive,
				SignedAt:      time.Now(),
			}
			h.Store.CreateLease(lease)

			// Update unit to occupied
			h.Store.UpdateUnitStatus(inv.UnitID, models.UnitStatusOccupied, leaseID)

			// Append initial rent/deposit logs to the Ledger (append-only)
			ledgerDeposit := &models.LedgerEntry{
				ID:                   uuid.New().String(),
				LeaseID:              leaseID,
				TenantID:             user.ID,
				LandlordID:           inv.SenderID,
				Type:                 models.LedgerTypeDeposit,
				Amount:               unit.DepositAmount,
				Currency:             "KES",
				GatewayUsed:          "daraja",
				GatewayTransactionID: "MPESA_DEP_" + uuid.New().String()[:8],
				IdempotencyKey:       uuid.New().String(),
				RequestSource:        "webhook",
				Status:               models.LedgerStatusConfirmed,
				WebhookSignatureVerified: true,
				CreatedAt:            time.Now(),
				StatusHistory: []models.StatusHistoryItem{
					{Status: models.LedgerStatusPending, ChangedBy: user.ID, ChangedAt: time.Now(), SourceIP: "127.0.0.1", Reason: "Auto-initiated deposit from invitation"},
					{Status: models.LedgerStatusConfirmed, ChangedBy: "daraja_webhook", ChangedAt: time.Now(), SourceIP: "127.0.0.1", Reason: "Payment webhook received"},
				},
			}
			h.Store.AddLedgerEntry(ledgerDeposit)
		}
	}

	token := "session_" + user.ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

// ListInvitations returns invitations scoped to the requesting user's role.
// Platform admins see all; landlords/agents see only ones they sent.
func (h *InvitationsHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.Store.RLock()
	user, userOk := h.Store.Users[userID]
	h.Store.RUnlock()
	if !userOk {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	isAdmin := user.Role == models.RoleSuperAdmin ||
		user.Role == models.RoleTechAdmin ||
		user.Role == models.RoleSupportAdmin ||
		user.Role == models.RoleBillingAdmin

	h.Store.RLock()
	var list []*models.Invitation
	for _, inv := range h.Store.Invitations {
		if isAdmin || inv.SenderID == userID {
			list = append(list, inv)
		}
	}
	h.Store.RUnlock()

	if list == nil {
		list = []*models.Invitation{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// Dev helper to fetch all invitations
func (h *InvitationsHandler) DebugListInvitations(w http.ResponseWriter, r *http.Request) {
	h.Store.RLock()
	defer h.Store.RUnlock()

	var list []*models.Invitation
	for _, inv := range h.Store.Invitations {
		list = append(list, inv)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

type InvitationTokenRequest struct {
	Token string `json:"token"`
}

// RevokeInvitation marks a pending invitation as revoked so the link stops working.
func (h *InvitationsHandler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	callerID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req InvitationTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	inv, ok := h.Store.Invitations[req.Token]
	caller, callerOk := h.Store.Users[callerID]
	h.Store.RUnlock()

	if !ok {
		http.Error(w, "Invitation not found", http.StatusNotFound)
		return
	}
	if !callerOk {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Only sender or platform admins can revoke
	isAdmin := caller.Role == models.RoleSuperAdmin || caller.Role == models.RoleTechAdmin ||
		caller.Role == models.RoleSupportAdmin || caller.Role == models.RoleBillingAdmin
	if inv.SenderID != callerID && !isAdmin {
		http.Error(w, "Forbidden: only the sender or a platform admin can revoke this invitation", http.StatusForbidden)
		return
	}

	if inv.Status == models.InvitationStatusAccepted {
		http.Error(w, "Cannot revoke an invitation that has already been accepted", http.StatusConflict)
		return
	}
	if inv.Status == models.InvitationStatusRevoked {
		http.Error(w, "Invitation is already revoked", http.StatusConflict)
		return
	}

	h.Store.UpdateInvitationStatus(req.Token, models.InvitationStatusRevoked)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
}

// ResendInvitation re-sends the invitation email and resets the expiry window to 7 days.
func (h *InvitationsHandler) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	callerID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req InvitationTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	inv, ok := h.Store.Invitations[req.Token]
	caller, callerOk := h.Store.Users[callerID]
	h.Store.RUnlock()

	if !ok {
		http.Error(w, "Invitation not found", http.StatusNotFound)
		return
	}
	if !callerOk {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	isAdmin := caller.Role == models.RoleSuperAdmin || caller.Role == models.RoleTechAdmin ||
		caller.Role == models.RoleSupportAdmin || caller.Role == models.RoleBillingAdmin
	if inv.SenderID != callerID && !isAdmin {
		http.Error(w, "Forbidden: only the sender or a platform admin can resend this invitation", http.StatusForbidden)
		return
	}

	if inv.Status == models.InvitationStatusAccepted {
		http.Error(w, "Cannot resend an invitation that has already been accepted", http.StatusConflict)
		return
	}

	// Reset expiry, restore to pending, and re-send email
	h.Store.Lock()
	inv.Status = models.InvitationStatusPending
	inv.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	h.Store.Unlock()
	h.Store.UpdateInvitationStatus(req.Token, models.InvitationStatusPending)

	inviteLink := fmt.Sprintf("%s/invite/accept?token=%s", appBaseURL(), req.Token)
	var propName = "System Platform"
	h.Store.RLock()
	if inv.PropertyID != "" {
		if p, ok := h.Store.Properties[inv.PropertyID]; ok {
			propName = p.Name
		}
	}
	senderName := caller.Email
	h.Store.RUnlock()

	emailHTML := EmailInvitation(inv.Email, senderName, propName, inv.Role, inviteLink)
	if err := sendEmail(inv.Email, "Your REOS invitation (resent)", emailHTML); err != nil {
		log.Printf("[REOS] Failed to resend invitation email to %s: %v", inv.Email, err)
	} else {
		log.Printf("[REOS] Invitation resent to %s (token: %s)", inv.Email, req.Token)
	}
	fmt.Printf("\n[REOS RESEND] %s → %s\n\n", inv.Email, inviteLink)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resent", "email": inv.Email})
}

func getUserIdFromAuthHeader(r *http.Request, s *store.Store) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("missing or invalid auth header")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !strings.HasPrefix(token, "session_") {
		return "", fmt.Errorf("invalid token format")
	}

	s.Lock()
	defer s.Unlock()

	var matchedUser *models.User
	for _, u := range s.Users {
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

	// Fallback for old format
	if matchedUser == nil {
		userID := strings.TrimPrefix(token, "session_")
		if u, err := s.GetUserByID(userID); err == nil && len(u.Sessions) == 0 {
			matchedUser = u
		}
	}

	if matchedUser == nil {
		return "", fmt.Errorf("invalid or expired session token")
	}

	if matchedUser.Status == "suspended" {
		return "", fmt.Errorf("this account has been suspended by system support")
	}

	return matchedUser.ID, nil
}
