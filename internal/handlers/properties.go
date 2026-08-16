package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type PropertiesHandler struct {
	Store *store.Store
}

type CreatePropertyRequest struct {
	Name         string   `json:"name"`
	Latitude     float64  `json:"latitude"`
	Longitude    float64  `json:"longitude"`
	Jurisdiction string   `json:"jurisdiction"`
	Amenities    []string `json:"amenities"`
	Rules        []string `json:"rules"`
}

type CreateUnitRequest struct {
	PropertyID    string                 `json:"property_id"`
	BuildingLabel string                 `json:"building_label"`
	Label         string                 `json:"label"`
	RentAmount    float64                `json:"rent_amount"`
	DepositAmount float64                `json:"deposit_amount"`
	Images        []models.PropertyImage `json:"images"`
}

type PayRentRequest struct {
	LeaseID string  `json:"lease_id"`
	Amount  float64 `json:"amount"`
}

func (h *PropertiesHandler) GetUltimateOwnerID(userID string) string {
	currentID := userID
	visited := make(map[string]bool)
	for {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		u, err := h.Store.GetUserByID(currentID)
		if err != nil {
			break
		}

		if u.Role != models.RoleStaff && u.Role != models.RoleCaretaker && u.Role != models.RoleAgent {
			break
		}

		memberships := h.Store.GetAllStaffMemberships()
		var foundMembership *models.StaffMembership
		for _, m := range memberships {
			if m.StaffUserID == currentID && m.Status == "active" {
				foundMembership = m
				break
			}
		}

		if foundMembership == nil {
			break
		}

		currentID = foundMembership.PrincipalID
	}
	return currentID
}

func (h *PropertiesHandler) CreateProperty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	creatorID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ownerID := h.GetUltimateOwnerID(creatorID)

	var p models.Property
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if p.Name == "" {
		http.Error(w, "Property name is required", http.StatusBadRequest)
		return
	}

	p.ID = uuid.New().String()
	p.OwnerID = ownerID
	p.CreatedBy = creatorID
	p.ApprovalStatus = models.ApprovalPending
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	p.Reviews = []models.PropertyReview{}
	p.Ratings = models.RatingsSummary{
		AverageRating: 0.0,
		Distribution:  make(map[string]int),
	}

	h.Store.CreateProperty(&p)

	// Notify all associated users in real-time
	BroadcastPropertySync(ownerID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (h *PropertiesHandler) ListProperties(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIdFromAuthHeader(r, h.Store)
	var list []*models.Property

	if err != nil {
		// Guest user: return only approved properties
		all := h.Store.GetAllProperties()
		for _, p := range all {
			if p.ApprovalStatus == models.ApprovalApproved {
				list = append(list, p)
			}
		}
	} else {
		u, err := h.Store.GetUserByID(userID)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		if u.Role == models.RoleLandlord {
			list = h.Store.GetPropertiesByOwner(userID)
		} else if u.Role == models.RoleSuperAdmin || u.Role == models.RoleTechAdmin || u.Role == models.RoleSupportAdmin {
			list = h.Store.GetAllProperties()
		} else if u.Role == models.RoleStaff || u.Role == models.RoleCaretaker || u.Role == models.RoleAgent {
			memberships := h.Store.GetAllStaffMemberships()
			var principalIDs []string
			var specificPropIDs []string
			globalAllForPrincipal := false

			for _, m := range memberships {
				if m.StaffUserID == userID && m.Status == "active" {
					principalIDs = append(principalIDs, m.PrincipalID)
					if len(m.AssignedProperties) == 0 {
						globalAllForPrincipal = true
					} else {
						specificPropIDs = append(specificPropIDs, m.AssignedProperties...)
					}
				}
			}

			allProps := h.Store.GetAllProperties()
			for _, p := range allProps {
				if p.CreatedBy == userID {
					list = append(list, p)
					continue
				}

				if globalAllForPrincipal {
					isPrincipalOwner := false
					for _, pid := range principalIDs {
						if p.OwnerID == pid {
							isPrincipalOwner = true
							break
						}
					}
					if isPrincipalOwner {
						list = append(list, p)
						continue
					}
				}

				isSpecific := false
				for _, spid := range specificPropIDs {
					if p.ID == spid {
						isSpecific = true
						break
					}
				}
				if isSpecific {
					list = append(list, p)
				}
			}
		} else {
			// regular clients/tenants see only approved properties
			all := h.Store.GetAllProperties()
			for _, p := range all {
				if p.ApprovalStatus == models.ApprovalApproved {
					list = append(list, p)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *PropertiesHandler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateUnitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.PropertyID == "" || req.Label == "" || req.RentAmount <= 0 {
		http.Error(w, "PropertyID, Label, and RentAmount are required", http.StatusBadRequest)
		return
	}

	// Protect against IDOR
	if err := CheckPropertyOwnership(h.Store, req.PropertyID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	unit := &models.Unit{
		ID:            uuid.New().String(),
		PropertyID:    req.PropertyID,
		BuildingLabel: req.BuildingLabel,
		Label:         req.Label,
		Status:        models.UnitStatusAvailable,
		RentAmount:    req.RentAmount,
		DepositAmount: req.DepositAmount,
		Images:        req.Images,
	}

	h.Store.CreateUnit(unit)

	// Fetch property to find owner landlord ID for sync
	if prop, err := h.Store.GetPropertyByID(req.PropertyID); err == nil {
		BroadcastPropertySync(prop.OwnerID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(unit)
}

func (h *PropertiesHandler) ListUnits(w http.ResponseWriter, r *http.Request) {
	propertyID := r.URL.Query().Get("property_id")
	if propertyID == "" {
		http.Error(w, "property_id query param is required", http.StatusBadRequest)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err == nil {
		u, err := h.Store.GetUserByID(userID)
		if err == nil && u.Role == models.RoleLandlord {
			// Protect against IDOR for Landlords listing properties
			if err := CheckPropertyOwnership(h.Store, propertyID, userID); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
		}
	}

	list := h.Store.GetUnitsByProperty(propertyID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *PropertiesHandler) ListLeases(w http.ResponseWriter, r *http.Request) {
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

	var list []*models.Lease
	if u.Role == models.RoleTenant {
		list = h.Store.GetLeasesByTenant(userID)
	} else if u.Role == models.RoleLandlord {
		list = h.Store.GetLeasesByLandlord(userID)
	} else {
		// caretaker/agent/superadmin can see all leases for simplicity in this MVP mockup
		h.Store.RLock()
		for _, lease := range h.Store.Leases {
			list = append(list, lease)
		}
		h.Store.RUnlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *PropertiesHandler) PayRent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req PayRentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	lease, err := h.Store.GetLease(req.LeaseID)
	if err != nil {
		http.Error(w, "Lease not found", http.StatusNotFound)
		return
	}

	if lease.TenantID != tenantID {
		http.Error(w, "Unauthorized for this lease", http.StatusUnauthorized)
		return
	}

	idempKey := r.Header.Get("X-Idempotency-Key")
	if idempKey == "" {
		idempKey = uuid.New().String()
	}

	// Verify idempotency
	h.Store.RLock()
	var duplicate *models.LedgerEntry
	for _, entry := range h.Store.Ledger {
		if entry.IdempotencyKey == idempKey {
			duplicate = entry
			break
		}
	}
	h.Store.RUnlock()

	if duplicate != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(duplicate)
		return
	}

	// Create append-only ledger entry
	ledgerID := uuid.New().String()
	ledgerEntry := &models.LedgerEntry{
		ID:                   ledgerID,
		LeaseID:              lease.ID,
		TenantID:             tenantID,
		LandlordID:           lease.LandlordID,
		Type:                 models.LedgerTypeRent,
		Amount:               req.Amount,
		Currency:             "KES",
		GatewayUsed:          "daraja", // M-Pesa default
		GatewayTransactionID: "MPESA_RNT_" + uuid.New().String()[:8],
		IdempotencyKey:       idempKey,
		RequestSource:        "client",
		Status:               models.LedgerStatusConfirmed,
		WebhookSignatureVerified: true,
		CreatedAt:            time.Now(),
		StatusHistory: []models.StatusHistoryItem{
			{Status: models.LedgerStatusPending, ChangedBy: tenantID, ChangedAt: time.Now().Add(-2 * time.Second), SourceIP: r.RemoteAddr, Reason: "Initiated rent payment"},
			{Status: models.LedgerStatusConfirmed, ChangedBy: "daraja_gateway", ChangedAt: time.Now(), SourceIP: r.RemoteAddr, Reason: "Payment approved upstream"},
		},
	}

	h.Store.AddLedgerEntry(ledgerEntry)

	// Dispatch simulated SMS notification
	sms := &models.SMSNotification{
		ID:              uuid.New().String(),
		UserID:          tenantID,
		Phone:           "+254700000000",
		TemplateType:    "payment_confirmed",
		LinkedEntityRef: ledgerID,
		Status:          "sent",
		SentAt:          time.Now(),
	}
	h.Store.AddSMSLog(sms)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ledgerEntry)
}

func (h *PropertiesHandler) ListLedger(w http.ResponseWriter, r *http.Request) {
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

	var list []*models.LedgerEntry
	if u.Role == models.RoleTenant {
		list = h.Store.GetLedgerByTenant(userID)
	} else if u.Role == models.RoleLandlord {
		list = h.Store.GetLedgerByLandlord(userID)
	} else {
		list = h.Store.GetAllLedger()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *PropertiesHandler) ApproveProperty(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Forbidden: user not found", http.StatusForbidden)
		return
	}

	var req struct {
		ID   string `json:"id"`
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Property ID is required", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	prop, ok := h.Store.Properties[req.ID]
	h.Store.RUnlock()
	if !ok {
		http.Error(w, "Property not found", http.StatusNotFound)
		return
	}

	// Verify permission to approve
	isAllowed := false
	if user.Role == models.RoleSuperAdmin || user.Role == models.RoleSupportAdmin {
		isAllowed = true
	} else {
		ultimatePropOwner := ResolveUltimateOwnerID(h.Store, prop.OwnerID)
		ultimateUser := ResolveUltimateOwnerID(h.Store, userID)
		if ultimatePropOwner == userID || ultimatePropOwner == ultimateUser {
			isAllowed = true
		}
	}

	if !isAllowed {
		http.Error(w, "Forbidden: you do not have permission to approve this property", http.StatusForbidden)
		return
	}

	h.Store.Lock()
	now := time.Now()
	prop.ApprovalStatus = models.ApprovalApproved
	prop.ApprovalNote = req.Note
	prop.ApprovedBy = userID
	prop.ApprovedAt = &now
	prop.UpdatedAt = now
	h.Store.Unlock()

	h.Store.CreateProperty(prop) // writes update to db

	// Dispatch real-time WebSocket sync update
	BroadcastPropertySync(prop.OwnerID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prop)
}

func (h *PropertiesHandler) RejectProperty(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Forbidden: user not found", http.StatusForbidden)
		return
	}

	var req struct {
		ID   string `json:"id"`
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" || req.Note == "" {
		http.Error(w, "Property ID and rejection reason note are required", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	prop, ok := h.Store.Properties[req.ID]
	h.Store.RUnlock()
	if !ok {
		http.Error(w, "Property not found", http.StatusNotFound)
		return
	}

	// Verify permission to reject
	isAllowed := false
	if user.Role == models.RoleSuperAdmin || user.Role == models.RoleSupportAdmin {
		isAllowed = true
	} else {
		ultimatePropOwner := ResolveUltimateOwnerID(h.Store, prop.OwnerID)
		ultimateUser := ResolveUltimateOwnerID(h.Store, userID)
		if ultimatePropOwner == userID || ultimatePropOwner == ultimateUser {
			isAllowed = true
		}
	}

	if !isAllowed {
		http.Error(w, "Forbidden: you do not have permission to reject this property", http.StatusForbidden)
		return
	}

	h.Store.Lock()
	now := time.Now()
	prop.ApprovalStatus = models.ApprovalRejected
	prop.ApprovalNote = req.Note
	prop.ApprovedBy = userID
	prop.ApprovedAt = &now
	prop.UpdatedAt = now
	h.Store.Unlock()

	h.Store.CreateProperty(prop)

	// Dispatch real-time WebSocket sync update
	BroadcastPropertySync(prop.OwnerID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prop)
}

func (h *PropertiesHandler) UpdateProperty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.Property
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Invalid input or missing Property ID", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	prop, ok := h.Store.Properties[req.ID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Property not found", http.StatusNotFound)
		return
	}

	if prop.OwnerID != userID {
		h.Store.Unlock()
		http.Error(w, "Forbidden: you do not own this property resource", http.StatusForbidden)
		return
	}

	// Update fields
	prop.Name = req.Name
	prop.Slug = req.Slug
	prop.Description = req.Description
	prop.PropertyType = req.PropertyType
	prop.OwnershipType = req.OwnershipType
	prop.YearBuilt = req.YearBuilt
	prop.TotalUnits = req.TotalUnits
	prop.TotalFloors = req.TotalFloors
	prop.Location = req.Location
	prop.Address = req.Address
	prop.Neighbourhood = req.Neighbourhood
	prop.City = req.City
	prop.Country = req.Country
	prop.RegionID = req.RegionID
	prop.Jurisdiction = req.Jurisdiction
	prop.IsGated = req.IsGated
	prop.IsBeachfront = req.IsBeachfront
	prop.BeachDistanceM = req.BeachDistanceM
	prop.IsWaterfront = req.IsWaterfront
	prop.LakeRiverName = req.LakeRiverName
	prop.AltitudeM = req.AltitudeM
	prop.IsGolfEstate = req.IsGolfEstate
	prop.IsEcoReserve = req.IsEcoReserve
	prop.Utilities = req.Utilities
	prop.ParkingSpaces = req.ParkingSpaces
	prop.ParkingType = req.ParkingType
	prop.HasElevator = req.HasElevator
	prop.HasGym = req.HasGym
	prop.HasPool = req.HasPool
	prop.HasRooftop = req.HasRooftop
	prop.HasBackupPower = req.HasBackupPower
	prop.HasChildPlayArea = req.HasChildPlayArea
	prop.HasConference = req.HasConference
	prop.HasServiced = req.HasServiced
	prop.Amenities = req.Amenities
	prop.Rules = req.Rules
	prop.NearbyFacilities = req.NearbyFacilities
	prop.Images = req.Images
	prop.VideoTourURL = req.VideoTourURL
	prop.VirtualTourURL = req.VirtualTourURL
	prop.Documents = req.Documents
	prop.UpdatedAt = time.Now()
	
	// Reset approval state back to pending review when modified
	prop.ApprovalStatus = models.ApprovalPending

	h.Store.Unlock()

	h.Store.CreateProperty(prop)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prop)
}

func (h *PropertiesHandler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		PropertyID string   `json:"property_id"`
		Rating     float64  `json:"rating"`
		Headline   string   `json:"headline"`
		Body       string   `json:"body"`
		Tags       []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PropertyID == "" || req.Rating < 1 || req.Rating > 5 {
		http.Error(w, "Property ID and valid rating (1-5) are required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	defer h.Store.Unlock()

	prop, ok := h.Store.Properties[req.PropertyID]
	if !ok {
		http.Error(w, "Property not found", http.StatusNotFound)
		return
	}

	// Verify tenancy check
	isVerified := false
	leaseID := ""
	units := make(map[string]bool)
	for _, unit := range h.Store.Units {
		if unit.PropertyID == req.PropertyID {
			units[unit.ID] = true
		}
	}
	for _, lease := range h.Store.Leases {
		if lease.TenantID == userID && units[lease.UnitID] {
			isVerified = true
			leaseID = lease.ID
			break
		}
	}

	review := models.PropertyReview{
		ID:         "rev_" + uuidGen(),
		ReviewerID: userID,
		LeaseID:    leaseID,
		Rating:     req.Rating,
		Headline:   req.Headline,
		Body:       req.Body,
		Tags:       req.Tags,
		IsVerified: isVerified,
		CreatedAt:  time.Now(),
	}

	prop.Reviews = append(prop.Reviews, review)

	// Recalculate ratings summary
	sum := 0.0
	verifiedCount := 0
	dist := make(map[string]int)
	for _, rev := range prop.Reviews {
		sum += rev.Rating
		if rev.IsVerified {
			verifiedCount++
		}
		rStr := fmt.Sprintf("%.0f", rev.Rating)
		dist[rStr]++
	}
	prop.Ratings.AverageRating = sum / float64(len(prop.Reviews))
	prop.Ratings.TotalReviews = len(prop.Reviews)
	prop.Ratings.VerifiedReviews = verifiedCount
	prop.Ratings.Distribution = dist

	h.Store.Unlock()

	h.Store.CreateProperty(prop)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

func (h *PropertiesHandler) RespondToReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		PropertyID string `json:"property_id"`
		ReviewID   string `json:"review_id"`
		Response   string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PropertyID == "" || req.ReviewID == "" || req.Response == "" {
		http.Error(w, "Property ID, Review ID, and Response body are required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	defer h.Store.Unlock()

	prop, ok := h.Store.Properties[req.PropertyID]
	if !ok {
		http.Error(w, "Property not found", http.StatusNotFound)
		return
	}

	if prop.OwnerID != userID {
		http.Error(w, "Forbidden: you do not own this property resource", http.StatusForbidden)
		return
	}

	found := false
	for i, rev := range prop.Reviews {
		if rev.ID == req.ReviewID {
			prop.Reviews[i].Response = req.Response
			prop.Reviews[i].ResponseAt = time.Now()
			found = true
			break
		}
	}

	if !found {
		h.Store.Unlock()
		http.Error(w, "Review not found", http.StatusNotFound)
		return
	}

	h.Store.Unlock()

	h.Store.CreateProperty(prop)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "responded"})
}
