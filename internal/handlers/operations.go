package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type OperationsHandler struct {
	Store *store.Store
}

func calculateFinalPrice(originalPrice float64, pc *models.PlatformCommissionSettings, vatRate, whtRate float64) float64 {
	if originalPrice <= 0 {
		return 0
	}
	base := originalPrice
	// 1. Cost of production markup
	base = base * (1.0 + pc.ProductionMarkupPct/100.0)
	// 2. Base platform commission + taxes (VAT + Withholding tax)
	taxMultiplier := 1.0 + (pc.BaseFeePercentage / 100.0)
	if pc.VATEnabled {
		if vatRate > 0 {
			taxMultiplier += vatRate / 100.0
		} else {
			taxMultiplier += 0.16 // standard Kenyan VAT
		}
	}
	if pc.WHTEnabled {
		if whtRate > 0 {
			taxMultiplier += whtRate / 100.0
		} else {
			taxMultiplier += 0.10 // standard Kenyan WHT
		}
	}
	return base * taxMultiplier
}


type CreateListingReq struct {
	PropertyID         string                     `json:"property_id"`
	UnitID             string                     `json:"unit_id,omitempty"`
	ListingType        string                     `json:"listing_type"`
	RentAmount         float64                    `json:"rent_amount,omitempty"`
	DepositAmount      float64                    `json:"deposit_amount,omitempty"`
	SaleDetails        *models.SaleDetails        `json:"sale_details,omitempty"`
	ShortStayDetails   *models.ShortStayDetails   `json:"short_stay_details,omitempty"`
	EventRentalDetails *models.EventRentalDetails `json:"event_rental_details,omitempty"`
}

func (h *OperationsHandler) CreateListing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.Store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	var listing models.Listing
	if err := json.NewDecoder(r.Body).Decode(&listing); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if listing.PropertyID == "" || listing.ListingType == "" {
		http.Error(w, "PropertyID and ListingType are required", http.StatusBadRequest)
		return
	}

	// Gating: check landlord verification & tier capacity
	if u.Role == models.RoleLandlord {
		verification, err := h.Store.GetLandlordVerificationByUserID(userID)
		if err == nil {
			// Check unlocked types
			typeAllowed := false
			for _, t := range verification.GrantedSnapshot.UnlockedListingTypes {
				if t == listing.ListingType {
					typeAllowed = true
					break
				}
			}
			if !typeAllowed {
				http.Error(w, "Forbidden: your current verification tier does not unlock this listing type", http.StatusForbidden)
				return
			}
		}
	}

	// Retrieve commission and jurisdiction rules to inject pricing commission
	prop, err := h.Store.GetPropertyByID(listing.PropertyID)
	if err == nil {
		pc := h.Store.GetPlatformCommissionSettings()
		var vatRate, whtRate float64
		for _, j := range h.Store.GetAllJurisdictions() {
			if j.Code == prop.Jurisdiction || j.ID == prop.RegionID {
				vatRate = j.VATRate
				whtRate = j.WHTRate
				break
			}
		}

		if listing.RentAmount > 0 {
			listing.RentAmount = calculateFinalPrice(listing.RentAmount, pc, vatRate, whtRate)
		}
		if listing.ShortStayDetails != nil && listing.ShortStayDetails.NightlyRate > 0 {
			listing.ShortStayDetails.NightlyRate = calculateFinalPrice(listing.ShortStayDetails.NightlyRate, pc, vatRate, whtRate)
		}
		if listing.EventRentalDetails != nil && listing.EventRentalDetails.HourlyRate > 0 {
			listing.EventRentalDetails.HourlyRate = calculateFinalPrice(listing.EventRentalDetails.HourlyRate, pc, vatRate, whtRate)
		}
		if listing.SaleDetails != nil && listing.SaleDetails.AskingPrice > 0 {
			listing.SaleDetails.AskingPrice = calculateFinalPrice(listing.SaleDetails.AskingPrice, pc, vatRate, whtRate)
		}
	}

	listing.ID = uuid.New().String()
	listing.CreatedBy = userID
	listing.ApprovalStatus = models.ApprovalPending
	listing.SubmitForReviewAt = time.Now()
	listing.CreatedAt = time.Now()
	listing.UpdatedAt = time.Now()

	// Check if escrow required (legal constraint)
	if listing.ListingType == "rental" || listing.ListingType == "short_stay" || listing.ListingType == "event_hourly" || listing.ListingType == "storage" || listing.ListingType == "coworking" {
		listing.EscrowRequired = true
	}

	h.Store.CreateListing(&listing)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(listing)
}

func (h *OperationsHandler) ListListings(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIdFromAuthHeader(r, h.Store)
	var list []*models.Listing
	all := h.Store.GetAllListings()

	if err != nil {
		// Guest: only approved and published
		for _, l := range all {
			if l.ApprovalStatus == models.ApprovalApproved && l.Status == "published" {
				list = append(list, l)
			}
		}
	} else {
		u, err := h.Store.GetUserByID(userID)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		if u.Role == models.RoleSuperAdmin || u.Role == models.RoleTechAdmin || u.Role == models.RoleSupportAdmin {
			list = all
		} else {
			// Landlords/Agents see their own listings, others see approved & published
			for _, l := range all {
				if l.CreatedBy == userID {
					list = append(list, l)
				} else if l.ApprovalStatus == models.ApprovalApproved && l.Status == "published" {
					list = append(list, l)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

type CreateBookingReq struct {
	ListingID     string    `json:"listing_id"`
	StartDatetime time.Time `json:"start_datetime"`
	EndDatetime   time.Time `json:"end_datetime"`
	TotalPrice    float64   `json:"total_price"`
	DepositHeld   float64   `json:"deposit_held"`
}

func (h *OperationsHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateBookingReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ListingID == "" || req.TotalPrice <= 0 {
		http.Error(w, "ListingID and TotalPrice must be specified", http.StatusBadRequest)
		return
	}

	listing, err := h.Store.GetListing(req.ListingID)
	if err != nil {
		http.Error(w, "Listing not found", http.StatusNotFound)
		return
	}

	booking := &models.Booking{
		ID:            uuid.New().String(),
		ListingID:     req.ListingID,
		ClientID:      clientID,
		StartDatetime: req.StartDatetime,
		EndDatetime:   req.EndDatetime,
		TotalPrice:    req.TotalPrice,
		DepositHeld:   req.DepositHeld,
		Status:        "confirmed",
		CreatedAt:     time.Now(),
	}

	h.Store.CreateBooking(booking)

	// Auto generate commission for staff if applicable
	h.Store.Lock()
	var principalID string
	var principalType string
	for _, p := range h.Store.Properties {
		if p.ID == listing.PropertyID {
			principalID = p.OwnerID
			principalType = "landlord"
			break
		}
	}
	h.Store.Unlock()

	commission := &models.Commission{
		ID:             uuid.New().String(),
		PrincipalID:    principalID,
		PrincipalType:  principalType,
		LeadID:         "lead_auto_" + uuid.New().String()[:8],
		ListingID:      listing.ID,
		TransactionRef: "tx_" + uuid.New().String()[:8],
		Amount:         req.TotalPrice * 0.1, // 10% auto commission
		Status:         "pending",
	}
	h.Store.CreateCommission(commission)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(booking)
}

func (h *OperationsHandler) ListBookings(w http.ResponseWriter, r *http.Request) {
	bookings := h.Store.GetAllBookings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bookings)
}

type InviteStaffReq struct {
	Email          string   `json:"email"`
	Phone          string   `json:"phone"`
	PropertiesList []string `json:"properties"`
	RegionsList    []string `json:"regions"`
}

func (h *OperationsHandler) InviteStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principalID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.Store.GetUserByID(principalID)
	if err != nil {
		http.Error(w, "Principal user not found", http.StatusUnauthorized)
		return
	}

	var req InviteStaffReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create user with role staff
	staffUser := &models.User{
		ID:                   "staff_" + uuid.New().String()[:8],
		Role:                 models.RoleStaff,
		Email:                req.Email,
		Phone:                req.Phone,
		Status:               "active",
		IdentityVerification: "approved",
		CreatedAt:            time.Now(),
		Scope: models.Scope{
			Type:   "assigned_properties",
			RefIDs: req.PropertiesList,
		},
	}
	h.Store.CreateUser(staffUser)

	sm := &models.StaffMembership{
		ID:                 uuid.New().String(),
		StaffUserID:        staffUser.ID,
		PrincipalID:        principalID,
		PrincipalType:      u.Role,
		AssignedProperties: req.PropertiesList,
		AssignedRegions:    req.RegionsList,
		CanAutoPublish:     true,
		Status:             "active",
		InvitedAt:          time.Now(),
		AcceptedAt:         time.Now(),
	}

	h.Store.CreateStaffMembership(sm)

	res := StaffMemberResponse{
		StaffMembership: sm,
		Email:           staffUser.Email,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

type StaffMemberResponse struct {
	*models.StaffMembership
	Email           string  `json:"email"`
	LeadsCount      int     `json:"leads_count"`
	ViewingsCount   int     `json:"viewings_count"`
	DealsClosed     int     `json:"deals_closed"`
	PropertiesCount int     `json:"properties_count"`
	TotalEarnings   float64 `json:"total_earnings"`
}

func (h *OperationsHandler) ListStaff(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Self-healing check for accepted invitations without staff memberships
	allInvitations := h.Store.GetAllInvitations()
	for _, inv := range allInvitations {
		if inv.Status == models.InvitationStatusAccepted && (inv.Role == models.RoleStaff || inv.Role == models.RoleCaretaker || inv.Role == models.RoleAgent) {
			var targetUser *models.User
			var userErr error
			if inv.Email != "" {
				targetUser, userErr = h.Store.GetUserByEmail(inv.Email)
			}
			if userErr == nil && targetUser != nil {
				exists := false
				memberships := h.Store.GetAllStaffMemberships()
				for _, m := range memberships {
					if m.StaffUserID == targetUser.ID && m.PrincipalID == inv.SenderID {
						exists = true
						break
					}
				}
				if !exists {
					var assignedProps []string
					if inv.PropertyID != "" {
						assignedProps = []string{inv.PropertyID}
					}
					sm := &models.StaffMembership{
						ID:                 uuid.New().String(),
						StaffUserID:        targetUser.ID,
						PrincipalID:        inv.SenderID,
						PrincipalType:      models.RoleLandlord,
						AssignedProperties: assignedProps,
						AssignedRegions:    []string{"Nairobi"},
						CanAutoPublish:     true,
						Status:             "active",
						InvitedAt:          inv.CreatedAt,
						AcceptedAt:         time.Now(),
					}
					sender, senderExists := h.Store.GetUserByID(inv.SenderID)
					if senderExists == nil && sender != nil {
						sm.PrincipalType = sender.Role
					}
					h.Store.CreateStaffMembership(sm)
				}
			}
		}
	}

	memberships := h.Store.GetAllStaffMemberships()
	var list []StaffMemberResponse
	for _, m := range memberships {
		if m.PrincipalID == userID || m.StaffUserID == userID {
			email := ""
			h.Store.RLock()
			su, err := h.Store.GetUserByID(m.StaffUserID)
			h.Store.RUnlock()
			if err == nil {
				email = su.Email
			}

			// Compute performance metrics
			leadsCount := 0
			dealsClosed := 0
			allLeads := h.Store.GetAllLeads()
			for _, l := range allLeads {
				if l.AssignedStaffID == m.StaffUserID {
					leadsCount++
					if l.Status == "converted" || l.Status == "closed" {
						dealsClosed++
					}
				}
			}

			viewingsCount := 0
			viewings := h.Store.GetViewingsByStaffID(m.StaffUserID)
			viewingsCount = len(viewings)

			propertiesCount := 0
			allProps := h.Store.GetAllProperties()
			for _, p := range allProps {
				if p.CreatedBy == m.StaffUserID {
					propertiesCount++
				}
			}

			totalEarnings := 0.0
			allComms := h.Store.GetAllCommissions()
			for _, c := range allComms {
				if c.StaffID == m.StaffUserID {
					totalEarnings += c.Amount
				}
			}

			list = append(list, StaffMemberResponse{
				StaffMembership: m,
				Email:           email,
				LeadsCount:      leadsCount,
				ViewingsCount:   viewingsCount,
				DealsClosed:     dealsClosed,
				PropertiesCount: propertiesCount,
				TotalEarnings:   totalEarnings,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *OperationsHandler) ListTiers(w http.ResponseWriter, r *http.Request) {
	tiers := h.Store.GetAllTierDefinitions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tiers)
}

type UpgradeTierReq struct {
	TargetTierLevel int `json:"target_tier_level"`
}

func (h *OperationsHandler) UpgradeTier(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req UpgradeTierReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	lv, err := h.Store.GetLandlordVerificationByUserID(userID)
	if err != nil {
		http.Error(w, "Verification file not found", http.StatusNotFound)
		return
	}

	var targetDef *models.TierDefinition
	for _, td := range h.Store.TierDefinitions {
		if td.Level == req.TargetTierLevel {
			targetDef = td
			break
		}
	}

	if targetDef == nil {
		http.Error(w, "Invalid tier level specified", http.StatusBadRequest)
		return
	}

	// sequential ladder validation
	if req.TargetTierLevel != lv.CurrentTierLevel+1 {
		http.Error(w, "Upgrade must follow sequential tier ladder (1 -> 2 -> 3)", http.StatusBadRequest)
		return
	}

	// Update verification status directly to active (KYC auto-approved in prototype)
	lv.CurrentTierLevel = req.TargetTierLevel
	lv.GrantedSnapshot = models.GrantedSnapshot{
		TierDefinitionID:     targetDef.ID,
		PropertyCap:          targetDef.PropertyCap,
		UnlockedListingTypes: targetDef.UnlockedListingTypes,
		GrantedAt:            time.Now(),
	}
	lv.TierHistory = append(lv.TierHistory, models.TierHistoryItem{
		TierDefinitionID: targetDef.ID,
		Level:            targetDef.Level,
		AchievedAt:       time.Now(),
		VerifiedBy:       "system_auto",
		PaymentRef:       "pay_ref_" + uuid.New().String()[:8],
	})

	h.Store.CreateLandlordVerification(lv)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lv)
}

func (h *OperationsHandler) ListLeads(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	u, _ := h.Store.GetUserByID(userID)

	leads := h.Store.GetAllLeads()
	var list []*models.Lead
	for _, l := range leads {
		if u.Role == models.RoleSuperAdmin || l.PrincipalID == userID || l.AssignedStaffID == userID {
			list = append(list, l)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *OperationsHandler) ListCommissions(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	commissions := h.Store.GetAllCommissions()
	var list []*models.Commission
	for _, c := range commissions {
		if c.PrincipalID == userID || c.StaffID == userID {
			list = append(list, c)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *OperationsHandler) ApproveListing(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}

	var req struct {
		ID   string `json:"id"`
		Note string `json:"note"`
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

	now := time.Now()
	listing.ApprovalStatus = models.ApprovalApproved
	listing.ApprovalNote = req.Note
	listing.ApprovedBy = userID
	listing.ApprovedAt = &now
	listing.UpdatedAt = now
	h.Store.Unlock()

	h.Store.CreateListing(listing)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listing)
}

func (h *OperationsHandler) RejectListing(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Forbidden: admin only", http.StatusForbidden)
		return
	}

	var req struct {
		ID   string `json:"id"`
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" || req.Note == "" {
		http.Error(w, "Listing ID and rejection reason note are required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	listing, ok := h.Store.Listings[req.ID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Listing not found", http.StatusNotFound)
		return
	}

	now := time.Now()
	listing.ApprovalStatus = models.ApprovalRejected
	listing.ApprovalNote = req.Note
	listing.ApprovedBy = userID
	listing.ApprovedAt = &now
	listing.UpdatedAt = now
	h.Store.Unlock()

	h.Store.CreateListing(listing)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listing)
}

func (h *OperationsHandler) UpdateListing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.Listing
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Invalid input or missing Listing ID", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	listing, ok := h.Store.Listings[req.ID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Listing not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if listing.CreatedBy != userID {
		h.Store.Unlock()
		http.Error(w, "Forbidden: you did not create this listing", http.StatusForbidden)
		return
	}

	// Retrieve commission and jurisdiction rules to inject pricing commission
	prop, err := h.Store.GetPropertyByID(listing.PropertyID)
	if err == nil {
		pc := h.Store.GetPlatformCommissionSettings()
		var vatRate, whtRate float64
		for _, j := range h.Store.GetAllJurisdictions() {
			if j.Code == prop.Jurisdiction || j.ID == prop.RegionID {
				vatRate = j.VATRate
				whtRate = j.WHTRate
				break
			}
		}

		if req.RentAmount > 0 {
			listing.RentAmount = calculateFinalPrice(req.RentAmount, pc, vatRate, whtRate)
		} else {
			listing.RentAmount = req.RentAmount
		}
		if req.ShortStayDetails != nil && req.ShortStayDetails.NightlyRate > 0 {
			req.ShortStayDetails.NightlyRate = calculateFinalPrice(req.ShortStayDetails.NightlyRate, pc, vatRate, whtRate)
		}
		if req.EventRentalDetails != nil && req.EventRentalDetails.HourlyRate > 0 {
			req.EventRentalDetails.HourlyRate = calculateFinalPrice(req.EventRentalDetails.HourlyRate, pc, vatRate, whtRate)
		}
		if req.SaleDetails != nil && req.SaleDetails.AskingPrice > 0 {
			req.SaleDetails.AskingPrice = calculateFinalPrice(req.SaleDetails.AskingPrice, pc, vatRate, whtRate)
		}
	} else {
		listing.RentAmount = req.RentAmount
	}

	listing.Title = req.Title
	listing.Description = req.Description
	listing.ListingType = req.ListingType
	listing.Status = req.Status
	listing.Bedrooms = req.Bedrooms
	listing.Bathrooms = req.Bathrooms
	listing.SizeM2 = req.SizeM2
	listing.Furnished = req.Furnished
	listing.PetFriendly = req.PetFriendly
	listing.ParkingSpaces = req.ParkingSpaces
	listing.Floor = req.Floor
	listing.DepositAmount = req.DepositAmount
	listing.ServiceCharge = req.ServiceCharge
	listing.SaleDetails = req.SaleDetails
	listing.ShortStayDetails = req.ShortStayDetails
	listing.EventRentalDetails = req.EventRentalDetails
	listing.EscrowRequired = req.EscrowRequired
	listing.Amenities = req.Amenities
	listing.Images = req.Images
	listing.VideoURL = req.VideoURL
	listing.RegionID = req.RegionID
	listing.UpdatedAt = time.Now()

	// Reset approval state back to pending review when modified
	listing.ApprovalStatus = models.ApprovalPending
	listing.SubmitForReviewAt = time.Now()

	h.Store.Unlock()

	h.Store.CreateListing(listing)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listing)
}

func (h *OperationsHandler) DebugDB(w http.ResponseWriter, r *http.Request) {
	type DebugResponse struct {
		Users            []*models.User            `json:"users"`
		Invitations      []*models.Invitation      `json:"invitations"`
		StaffMemberships []*models.StaffMembership `json:"staff_memberships"`
	}

	h.Store.RLock()
	defer h.Store.RUnlock()

	var users []*models.User
	for _, u := range h.Store.Users {
		users = append(users, u)
	}

	var invites []*models.Invitation
	for _, i := range h.Store.Invitations {
		invites = append(invites, i)
	}

	var memberships []*models.StaffMembership
	for _, m := range h.Store.StaffMemberships {
		memberships = append(memberships, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DebugResponse{
		Users:            users,
		Invitations:      invites,
		StaffMemberships: memberships,
	})
}
