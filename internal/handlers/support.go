package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type SupportHandler struct {
	Store *store.Store
}

func NewSupportHandler(s *store.Store) *SupportHandler {
	return &SupportHandler{Store: s}
}

// UnpublishListing allows a support agent to unpublish a listing
func (h *SupportHandler) UnpublishListing(w http.ResponseWriter, r *http.Request) {
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
	if err != nil || (user.Role != models.RoleSuperAdmin && user.Role != models.RoleSupportAdmin) {
		http.Error(w, "Forbidden: support only", http.StatusForbidden)
		return
	}

	var req struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Listing ID is required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	listing, ok := h.Store.Listings[req.ID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Listing not found", http.StatusNotFound)
		return
	}

	listing.Status = "delisted"
	listing.ApprovalStatus = models.ApprovalRejected
	listing.RejectionReason = req.Reason
	listing.UpdatedAt = time.Now()
	h.Store.Unlock()

	h.Store.CreateListing(listing)

	// Send real-time notification to the landlord
	BroadcastNotification(listing.CreatedBy, "Listing Delisted", "Your listing '"+listing.Title+"' has been unpublished by system support: "+req.Reason)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listing)
}

// SuspendUser allows support agent to suspend or activate a landlord or agent
func (h *SupportHandler) SuspendUser(w http.ResponseWriter, r *http.Request) {
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
	if err != nil || (user.Role != models.RoleSuperAdmin && user.Role != models.RoleSupportAdmin) {
		http.Error(w, "Forbidden: support only", http.StatusForbidden)
		return
	}

	var req struct {
		TargetUserID string `json:"user_id"`
		Status       string `json:"status"` // active, suspended
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TargetUserID == "" || req.Status == "" {
		http.Error(w, "user_id and status are required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	target, err := h.Store.GetUserByID(req.TargetUserID)
	if err != nil {
		h.Store.Unlock()
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	target.Status = req.Status
	h.Store.Unlock()

	h.Store.UpdateUser(target)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(target)
}

// VerifyKYC verifies landlord or agent verification documents
func (h *SupportHandler) VerifyKYC(w http.ResponseWriter, r *http.Request) {
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
	if err != nil || (user.Role != models.RoleSuperAdmin && user.Role != models.RoleSupportAdmin) {
		http.Error(w, "Forbidden: support only", http.StatusForbidden)
		return
	}

	var req struct {
		VerificationID string `json:"verification_id"`
		Status         string `json:"status"` // approved, rejected
		Reason         string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VerificationID == "" || req.Status == "" {
		http.Error(w, "verification_id and status are required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	lv, ok := h.Store.LandlordVerifications[req.VerificationID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Verification file not found", http.StatusNotFound)
		return
	}

	lv.Status = req.Status
	for i := range lv.KYCDocuments {
		if lv.KYCDocuments[i].Status == "pending" {
			lv.KYCDocuments[i].Status = req.Status
			lv.KYCDocuments[i].RejectionReason = req.Reason
			lv.KYCDocuments[i].ReviewedBy = userID
		}
	}

	if req.Status == "approved" {
		// Automatically activate the upgraded tier and permissions
		lv.CurrentTierLevel = 2 // Verified Host
		// Fetch tier definition level 2 details to build snapshot
		for _, td := range h.Store.TierDefinitions {
			if td.Level == 2 {
				lv.GrantedSnapshot = models.GrantedSnapshot{
					TierDefinitionID:     td.ID,
					PropertyCap:          td.PropertyCap,
					UnlockedListingTypes: td.UnlockedListingTypes,
					GrantedAt:            time.Now(),
				}
				break
			}
		}
	}
	h.Store.Unlock()

	h.Store.CreateLandlordVerification(lv)

	// Update landlord account tier
	h.Store.Lock()
	landlordUser, err := h.Store.GetUserByID(lv.UserID)
	if err == nil {
		if req.Status == "approved" {
			landlordUser.SubscriptionTier = "verified_host"
			landlordUser.SubscriptionStatus = "active"
		}
	}
	h.Store.Unlock()
	if err == nil {
		h.Store.UpdateUser(landlordUser)
		BroadcastNotification(landlordUser.ID, "KYC Status Update", "Your verification documents review status: "+req.Status)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lv)
}

func (h *SupportHandler) ListKYCQueues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil || (user.Role != models.RoleSuperAdmin && user.Role != models.RoleSupportAdmin) {
		http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
		return
	}

	h.Store.Lock()
	defer h.Store.Unlock()
	var list []*models.LandlordVerification
	for _, lv := range h.Store.LandlordVerifications {
		list = append(list, lv)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// VacateLease submits a vacation notice
func (h *SupportHandler) VacateLease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.VacationNotice
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LeaseID == "" {
		http.Error(w, "Lease ID is required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	lease, ok := sLease(h.Store, req.LeaseID)
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Lease not found", http.StatusNotFound)
		return
	}

	req.ID = "vacate_" + uuid.New().String()
	req.TenantID = userID
	req.NoticeDate = time.Now()
	req.Status = "pending"
	req.CreatedAt = time.Now()
	h.Store.Unlock()

	h.Store.CreateVacationNotice(&req)

	// Update Lease state
	h.Store.Lock()
	lease.Status = "notice_given"
	h.Store.Unlock()
	h.Store.CreateLease(lease)

	// Update Unit status
	h.Store.Lock()
	unit, ok := h.Store.Units[lease.UnitID]
	if ok {
		unit.Status = "notice_given"
	}
	h.Store.Unlock()
	if ok {
		h.Store.CreateUnit(unit)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

// TransferTenant moves a tenant from one unit to another
func (h *SupportHandler) TransferTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentLeaseID string `json:"current_lease_id"`
		TargetUnitID   string `json:"target_unit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CurrentLeaseID == "" || req.TargetUnitID == "" {
		http.Error(w, "current_lease_id and target_unit_id are required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	currentLease, ok := sLease(h.Store, req.CurrentLeaseID)
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Current lease not found", http.StatusNotFound)
		return
	}

	targetUnit, ok := h.Store.Units[req.TargetUnitID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Target unit not found", http.StatusNotFound)
		return
	}

	// Terminate current lease
	now := time.Now()
	currentLease.Status = "terminated"
	currentLease.EndDate = now
	h.Store.Unlock()

	h.Store.CreateLease(currentLease)

	// Make current unit available
	h.Store.Lock()
	currUnit, ok := h.Store.Units[currentLease.UnitID]
	if ok {
		currUnit.Status = "available"
	}
	h.Store.Unlock()
	if ok {
		h.Store.CreateUnit(currUnit)
	}

	// Create new lease for the target unit
	h.Store.Lock()
	newLease := &models.Lease{
		ID:            "lease_" + uuid.New().String(),
		UnitID:        targetUnit.ID,
		TenantID:      currentLease.TenantID,
		LandlordID:    currentLease.LandlordID,
		RentAmount:    targetUnit.RentAmount,
		DepositAmount: targetUnit.DepositAmount,
		StartDate:     now,
		Status:        "active",
	}
	h.Store.Unlock()

	h.Store.CreateLease(newLease)

	// Make target unit occupied
	h.Store.Lock()
	targetUnit.Status = "occupied"
	targetUnit.CurrentLeaseID = newLease.ID
	h.Store.Unlock()
	h.Store.CreateUnit(targetUnit)

	// Create a transfer ledger entry
	h.Store.Lock()
	ledgerEntry := &models.LedgerEntry{
		ID:          "ledger_" + uuid.New().String(),
		LeaseID:     newLease.ID,
		TenantID:    newLease.TenantID,
		LandlordID:  newLease.LandlordID,
		Type:        "transfer",
		Amount:      targetUnit.RentAmount,
		Currency:    "KES",
		GatewayUsed: "platform_transfer",
		CreatedAt:   now,
	}
	h.Store.Unlock()
	h.Store.AddLedgerEntry(ledgerEntry)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newLease)
}

func sLease(st *store.Store, leaseID string) (*models.Lease, bool) {
	l, ok := st.Leases[leaseID]
	return l, ok
}

func (h *SupportHandler) CreateInspection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.Inspection
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = "insp_" + uuid.New().String()
	req.CaretakerID = userID
	req.LoggedAt = time.Now()

	h.Store.CreateInspection(&req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func (h *SupportHandler) ListInspections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	leaseID := r.URL.Query().Get("lease_id")
	if leaseID == "" {
		http.Error(w, "lease_id query parameter is required", http.StatusBadRequest)
		return
	}

	list := h.Store.GetInspectionsByLeaseID(leaseID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *SupportHandler) CreateDeductionDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		LeaseID     string  `json:"lease_id"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LeaseID == "" || req.Amount <= 0 {
		http.Error(w, "Invalid request arguments", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	lease, ok := h.Store.Leases[req.LeaseID]
	h.Store.Unlock()
	if !ok {
		http.Error(w, "Lease not found", http.StatusNotFound)
		return
	}

	ledger := &models.LedgerEntry{
		ID:            "ledger_" + uuid.New().String(),
		LeaseID:       req.LeaseID,
		TenantID:      lease.TenantID,
		LandlordID:    lease.LandlordID,
		Type:          "deduction",
		Amount:        req.Amount,
		Currency:      "KES",
		Description:   req.Description,
		RequestSource: "caretaker_manual",
		Status:        "pending",
		CreatedAt:     time.Now(),
	}

	h.Store.AddLedgerEntry(ledger)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ledger)
}

func (h *SupportHandler) ListDeductions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.Store.Lock()
	defer h.Store.Unlock()

	var list []*models.LedgerEntry
	for _, entry := range h.Store.Ledger {
		if entry.Type == "deduction" {
			list = append(list, entry)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *SupportHandler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.Application
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ListingID == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = "app_" + uuid.New().String()
	req.TenantID = userID
	req.AppliedAt = time.Now()
	req.Status = "pending"

	// Match logic:
	// A simple heuristic for credit score & income to calculate match score
	matchScore := 50
	if req.CreditScore >= 700 {
		matchScore += 25
	} else if req.CreditScore >= 620 {
		matchScore += 15
	}
	// Let's assume rent is around 40k. If income ratio is high, match score goes up
	if req.MonthlyIncome >= 150000 {
		matchScore += 25
	} else if req.MonthlyIncome >= 80000 {
		matchScore += 15
	}
	if matchScore > 100 {
		matchScore = 100
	}
	req.MatchScore = matchScore

	// Auto vs Manual application approval logic:
	// If the landlord has enabled auto-approval for listings or properties,
	// and matchScore is high (e.g. >= 80), automatically approve and create a lease!
	autoApprove := false
	h.Store.Lock()
	listing, okListing := h.Store.Listings[req.ListingID]
	if okListing {
		if listing.ApplicationReviewMode == "auto" {
			autoApprove = true
		}
	}
	h.Store.Unlock()

	if autoApprove && matchScore >= 80 {
		req.Status = "approved"
		req.Notes = "Automatically matched and approved by system auto-review rules."

		// Create Lease
		h.Store.Lock()
		newLease := &models.Lease{
			ID:            "lease_" + uuid.New().String(),
			UnitID:        listing.UnitID,
			TenantID:      userID,
			LandlordID:    listing.CreatedBy,
			RentAmount:    listing.RentAmount,
			DepositAmount: listing.RentAmount, // standard 1 month rent deposit
			StartDate:     time.Now(),
			Status:        "active",
		}
		h.Store.Unlock()
		h.Store.CreateLease(newLease)

		// Make Unit occupied
		h.Store.Lock()
		unit, okUnit := h.Store.Units[listing.UnitID]
		if okUnit {
			unit.Status = "occupied"
			unit.CurrentLeaseID = newLease.ID
		}
		h.Store.Unlock()
		if okUnit {
			h.Store.CreateUnit(unit)
		}

		// Notify user
		BroadcastNotification(userID, "Application Auto-Approved", "Congratulations! Your application for '"+req.ListingTitle+"' has been automatically approved and a lease generated.")
	} else {
		// Notify landlord of new application
		if okListing {
			BroadcastNotification(listing.CreatedBy, "New Tenant Application", "You have received a new application from "+req.TenantName+" for '"+req.ListingTitle+"'.")
		}
	}

	h.Store.CreateApplication(&req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func (h *SupportHandler) ListApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.Store.Lock()
	user, errUser := h.Store.GetUserByID(userID)
	h.Store.Unlock()

	var list []*models.Application
	if errUser == nil {
		h.Store.Lock()
		defer h.Store.Unlock()
		for _, app := range h.Store.Applications {
			// Landlords see applications for their listings. Tenants see their own.
			if user.Role == models.RoleSuperAdmin || user.Role == models.RoleSupportAdmin {
				list = append(list, app)
			} else if user.Role == models.RoleLandlord || user.Role == models.RoleAgent {
				if listing, ok := h.Store.Listings[app.ListingID]; ok && listing.CreatedBy == userID {
					list = append(list, app)
				}
			} else {
				if app.TenantID == userID {
					list = append(list, app)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *SupportHandler) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	app, ok := h.Store.Applications[req.ID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}

	app.Status = req.Status
	app.Notes = req.Notes
	h.Store.Unlock()

	h.Store.UpdateApplication(app)

	// If approved manually, generate lease
	if req.Status == "approved" {
		h.Store.Lock()
		listing, okListing := h.Store.Listings[app.ListingID]
		h.Store.Unlock()

		if okListing {
			h.Store.Lock()
			newLease := &models.Lease{
				ID:            "lease_" + uuid.New().String(),
				UnitID:        listing.UnitID,
				TenantID:      app.TenantID,
				LandlordID:    listing.CreatedBy,
				RentAmount:    listing.RentAmount,
				DepositAmount: listing.RentAmount,
				StartDate:     time.Now(),
				Status:        "active",
			}
			h.Store.Unlock()
			h.Store.CreateLease(newLease)

			h.Store.Lock()
			unit, okUnit := h.Store.Units[listing.UnitID]
			if okUnit {
				unit.Status = "occupied"
				unit.CurrentLeaseID = newLease.ID
			}
			h.Store.Unlock()
			if okUnit {
				h.Store.CreateUnit(unit)
			}
		}
	}

	// Notify tenant
	BroadcastNotification(app.TenantID, "Application Update", "Your application for '"+app.ListingTitle+"' has been updated to: "+req.Status)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

func (h *SupportHandler) CreateViewing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.Viewing
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LeadID == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = "view_" + uuid.New().String()
	req.StaffID = userID
	req.LoggedAt = time.Now()

	h.Store.CreateViewing(&req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func (h *SupportHandler) ListViewings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	list := h.Store.GetViewingsByStaffID(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
