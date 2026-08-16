package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type DashboardsHandler struct {
	Store *store.Store
}

// Global Gateway Configuration mockup
type GatewayConfig struct {
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	Priority int    `json:"priority"`
}

var ActiveGateways = []GatewayConfig{
	{Name: "daraja", IsActive: true, Priority: 1},
	{Name: "paystack", IsActive: false, Priority: 2},
	{Name: "intasend", IsActive: true, Priority: 3},
	{Name: "pesapal", IsActive: false, Priority: 4},
}

type MaintenanceRequest struct {
	UnitID      string `json:"unit_id"`
	IssueType   string `json:"issue_type"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

type UpdateMaintenanceRequest struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"` // reported, reviewed, in_progress, completed
	Priority     string  `json:"priority,omitempty"`
	CaretakerID  string  `json:"caretaker_id,omitempty"`
	CostEstimate float64 `json:"cost_estimate,omitempty"`
	FinalCost    float64 `json:"final_cost,omitempty"`
}

type CreateDisputeRequest struct {
	Type            string   `json:"type"`
	PropertyID      string   `json:"property_id"`
	LeaseID         string   `json:"lease_id"`
	RespondentID    string   `json:"respondent_id"`
	SourceRefEntity string   `json:"source_ref_entity"`
	SourceRefID     string   `json:"source_ref_id"`
	Content         string   `json:"content"`
	Evidence        []string `json:"evidence"`
}

type MessageDisputeRequest struct {
	DisputeID string `json:"dispute_id"`
	Content   string `json:"content"`
}

type ResolveDisputeRequest struct {
	DisputeID       string `json:"dispute_id"`
	ResolutionNotes string `json:"resolution_notes"`
}

type GatewayConfigRequest struct {
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// General Dashboard stats response
func (h *DashboardsHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
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

	h.Store.RLock()
	defer h.Store.RUnlock()

	stats := make(map[string]interface{})
	stats["user_role"] = u.Role

	switch u.Role {
	case models.RoleSuperAdmin:
		// Invitation stats
		invTotal, invPending, invAccepted := 0, 0, 0
		for _, inv := range h.Store.Invitations {
			invTotal++
			switch inv.Status {
			case models.InvitationStatusPending:
				invPending++
			case models.InvitationStatusAccepted:
				invAccepted++
			}
		}
		stats["total_users"] = len(h.Store.Users)
		stats["total_properties"] = len(h.Store.Properties)
		stats["gateways"] = ActiveGateways
		stats["total_disputes"] = len(h.Store.Disputes)
		stats["total_listings"] = len(h.Store.Listings)
		stats["total_bookings"] = len(h.Store.Bookings)
		stats["invitations_sent"] = invTotal
		stats["invitations_pending"] = invPending
		stats["invitations_accepted"] = invAccepted

	case models.RoleTechAdmin:
		stats["mongo_status"] = "connected"
		stats["redis_status"] = "connected"
		stats["active_websocket_connections"] = 73
		stats["api_latency_ms"] = 8
		stats["system_memory_usage_mb"] = 214
		stats["error_logs_count"] = 0

	case models.RoleSupportAdmin:
		pendingKYC := 0
		for _, lv := range h.Store.LandlordVerifications {
			for _, doc := range lv.KYCDocuments {
				if doc.Status == "pending" {
					pendingKYC++
				}
			}
		}
		openDisputes := 0
		for _, d := range h.Store.Disputes {
			if d.Status == models.DisputeStatusOpen || d.Status == models.DisputeStatusUnderReview {
				openDisputes++
			}
		}
		stats["pending_kyc_count"] = pendingKYC
		stats["open_disputes_count"] = openDisputes
		stats["ticket_queue_count"] = 4
		stats["actions_logged"] = 12

	case models.RoleBillingAdmin:
		var totalPayments float64 = 0
		for _, entry := range h.Store.Ledger {
			if entry.Status == models.LedgerStatusConfirmed {
				totalPayments += entry.Amount
			}
		}
		stats["total_payments_amount"] = totalPayments
		stats["reconciliation_warnings"] = 0
		stats["pending_refunds_count"] = 1
		stats["saas_revenue_amount"] = 35000.0

	case models.RoleLandlord:
		var totalRevenue float64 = 0
		var activeTenants int = 0
		var maintenanceCount int = 0

		properties := make([]*models.Property, 0)
		for _, p := range h.Store.Properties {
			if p.OwnerID == userID {
				properties = append(properties, p)
			}
		}

		for _, lease := range h.Store.Leases {
			if lease.LandlordID == userID && lease.Status == models.LeaseStatusActive {
				activeTenants++
			}
		}

		for _, entry := range h.Store.Ledger {
			if entry.LandlordID == userID && entry.Status == models.LedgerStatusConfirmed {
				if entry.Type == models.LedgerTypeRent || entry.Type == models.LedgerTypeDeposit {
					totalRevenue += entry.Amount
				}
			}
		}

		for _, maint := range h.Store.Maintenance {
			if unit, exists := h.Store.Units[maint.UnitID]; exists {
				if prop, propExists := h.Store.Properties[unit.PropertyID]; propExists && prop.OwnerID == userID {
					if maint.Status != models.MaintenanceStatusCompleted {
						maintenanceCount++
					}
				}
			}
		}

		// Landlord Verification Details
		var verification *models.LandlordVerification
		for _, lv := range h.Store.LandlordVerifications {
			if lv.UserID == userID {
				verification = lv
				break
			}
		}
		if verification == nil {
			// fallback if missing
			verification = &models.LandlordVerification{
				UserID:           userID,
				CurrentTierLevel: 1,
				GrantedSnapshot: models.GrantedSnapshot{
					TierDefinitionID:     "tier_free",
					PropertyCap:          100,
					UnlockedListingTypes: []string{"rental", "storage"},
				},
				Status: "active",
			}
		}

		// Staff count
		staffCount := 0
		for _, sm := range h.Store.StaffMemberships {
			if sm.PrincipalID == userID {
				staffCount++
			}
		}

		stats["total_revenue"] = totalRevenue
		stats["active_tenants"] = activeTenants
		stats["properties_count"] = len(properties)
		stats["pending_maintenance"] = maintenanceCount
		stats["verification"] = verification
		stats["staff_count"] = staffCount

		// Invitation stats for landlord
		landlordInvTotal, landlordInvPending, landlordInvAccepted := 0, 0, 0
		for _, inv := range h.Store.Invitations {
			if inv.SenderID == userID {
				landlordInvTotal++
				switch inv.Status {
				case models.InvitationStatusPending:
					landlordInvPending++
				case models.InvitationStatusAccepted:
					landlordInvAccepted++
				}
			}
		}
		stats["invitations_sent"] = landlordInvTotal
		stats["invitations_pending"] = landlordInvPending
		stats["invitations_accepted"] = landlordInvAccepted

	case models.RoleAgent:
		assignedProperties := 0
		for _, sm := range h.Store.StaffMemberships {
			if sm.StaffUserID == userID && sm.PrincipalType == "agent" && sm.Status == "active" {
				assignedProperties += len(sm.AssignedProperties)
			}
		}
		// Calculate commissions
		var commissions float64 = 0
		for _, c := range h.Store.Commissions {
			if c.PrincipalID == userID && c.PrincipalType == "agent" {
				commissions += c.Amount
			}
		}
		stats["assigned_properties"] = assignedProperties
		stats["inquiries_count"] = 3
		stats["commissions_earned"] = commissions

		// Invitation stats for agent
		agentInvTotal, agentInvPending, agentInvAccepted := 0, 0, 0
		for _, inv := range h.Store.Invitations {
			if inv.SenderID == userID {
				agentInvTotal++
				switch inv.Status {
				case models.InvitationStatusPending:
					agentInvPending++
				case models.InvitationStatusAccepted:
					agentInvAccepted++
				}
			}
		}
		stats["invitations_sent"] = agentInvTotal
		stats["invitations_pending"] = agentInvPending
		stats["invitations_accepted"] = agentInvAccepted

	case models.RoleCaretaker:
		assignedTickets := 0
		completedTickets := 0
		for _, m := range h.Store.Maintenance {
			if m.CaretakerID == userID {
				if m.Status == models.MaintenanceStatusCompleted {
					completedTickets++
				} else {
					assignedTickets++
				}
			}
		}
		stats["assigned_tickets"] = assignedTickets
		stats["completed_tickets"] = completedTickets

	case models.RoleClient, models.RoleTenant:
		var currentLeases []*models.Lease
		var activeProperties []*models.Property
		var activeUnits []*models.Unit
		var balance float64 = 0

		for _, l := range h.Store.Leases {
			if l.TenantID == userID && l.Status == models.LeaseStatusActive {
				currentLeases = append(currentLeases, l)
				if unit, exists := h.Store.Units[l.UnitID]; exists {
					activeUnits = append(activeUnits, unit)
					if prop, propExists := h.Store.Properties[unit.PropertyID]; propExists {
						activeProperties = append(activeProperties, prop)
					}
				}
			}
		}

		// Calculate client outstanding balance (mockup calculation)
		if len(currentLeases) > 0 {
			for _, cl := range currentLeases {
				balance += cl.RentAmount
				for _, entry := range h.Store.Ledger {
					if entry.LeaseID == cl.ID && entry.Type == models.LedgerTypeRent && entry.Status == models.LedgerStatusConfirmed {
						balance -= entry.Amount
					}
				}
			}
			if balance < 0 {
				balance = 0
			}
		}

		// Client Bookings
		var bookings []*models.Booking
		for _, b := range h.Store.Bookings {
			if b.ClientID == userID {
				bookings = append(bookings, b)
			}
		}

		stats["active_leases"] = currentLeases
		stats["properties"] = activeProperties
		stats["units"] = activeUnits
		stats["outstanding_balance"] = balance
		stats["bookings"] = bookings

	case models.RoleStaff:
		assignedLeads := 0
		for _, l := range h.Store.Leads {
			if l.AssignedStaffID == userID {
				assignedLeads++
			}
		}
		var commissions float64 = 0
		for _, c := range h.Store.Commissions {
			if c.StaffID == userID {
				commissions += c.Amount
			}
		}
		stats["assigned_leads_count"] = assignedLeads
		stats["commissions_earned"] = commissions
		stats["properties_submitted"] = 2
		stats["rating"] = 4.8

		principalType := "landlord"
		principalName := "REOS Partner"
		for _, m := range h.Store.StaffMemberships {
			if m.StaffUserID == userID && m.Status == "active" {
				principalType = m.PrincipalType
				pu, err := h.Store.GetUserByID(m.PrincipalID)
				if err == nil {
					principalName = pu.Email
					if pu.Phone != "" {
						principalName = pu.Phone
					}
				}
				break
			}
		}
		stats["principal_type"] = principalType
		stats["principal_name"] = principalName
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Maintenance Handler Endpoints
func (h *DashboardsHandler) ReportMaintenance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req MaintenanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UnitID == "" || req.IssueType == "" || req.Description == "" {
		http.Error(w, "UnitID, IssueType, and Description are required", http.StatusBadRequest)
		return
	}

	maint := &models.Maintenance{
		ID:           uuid.New().String(),
		UnitID:       req.UnitID,
		ReportedBy:   tenantID,
		IssueType:    req.IssueType,
		Description:  req.Description,
		Priority:     req.Priority,
		ImagesBefore: []string{"/assets/maintenance-placeholder.jpg"},
		Status:       models.MaintenanceStatusReported,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Auto-assign first caretaker available for simplicity in MVP
	h.Store.Lock()
	for _, user := range h.Store.Users {
		if user.Role == models.RoleCaretaker {
			maint.CaretakerID = user.ID
			maint.Status = models.MaintenanceStatusReviewed
			break
		}
	}
	h.Store.Unlock()

	h.Store.CreateMaintenance(maint)

	// Create SMS log
	smsLog := &models.SMSNotification{
		ID:              uuid.New().String(),
		UserID:          tenantID,
		Phone:           "+254700000000",
		TemplateType:    "maintenance_update",
		LinkedEntityRef: maint.ID,
		Status:          "sent",
		SentAt:          time.Now(),
	}
	h.Store.AddSMSLog(smsLog)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(maint)
}

func (h *DashboardsHandler) ListMaintenance(w http.ResponseWriter, r *http.Request) {
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

	h.Store.RLock()
	defer h.Store.RUnlock()

	var list []*models.Maintenance
	if u.Role == models.RoleTenant {
		for _, m := range h.Store.Maintenance {
			if m.ReportedBy == userID {
				list = append(list, m)
			}
		}
	} else if u.Role == models.RoleCaretaker {
		for _, m := range h.Store.Maintenance {
			if m.CaretakerID == userID {
				list = append(list, m)
			}
		}
	} else if u.Role == models.RoleLandlord {
		// All tickets for units on landlord's properties
		for _, m := range h.Store.Maintenance {
			if unit, exists := h.Store.Units[m.UnitID]; exists {
				if prop, propExists := h.Store.Properties[unit.PropertyID]; propExists && prop.OwnerID == userID {
					list = append(list, m)
				}
			}
		}
	} else {
		for _, m := range h.Store.Maintenance {
			list = append(list, m)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *DashboardsHandler) UpdateMaintenance(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "User details not found", http.StatusUnauthorized)
		return
	}

	var req UpdateMaintenanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Protect against IDOR
	if err := CheckMaintenanceAccess(h.Store, req.ID, userID, user.Role); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	m, err := h.Store.GetMaintenance(req.ID)
	if err != nil {
		http.Error(w, "Maintenance record not found", http.StatusNotFound)
		return
	}

	h.Store.Lock()
	m.Status = req.Status
	m.UpdatedAt = time.Now()
	if req.Priority != "" {
		m.Priority = req.Priority
	}
	if req.CaretakerID != "" {
		m.CaretakerID = req.CaretakerID
	}
	if req.CostEstimate > 0 {
		m.CostEstimate = req.CostEstimate
	}
	if req.FinalCost > 0 {
		m.FinalCost = req.FinalCost
	}
	if req.Status == models.MaintenanceStatusCompleted {
		m.ImagesAfter = []string{"/assets/maintenance-completed.jpg"}
	}
	h.Store.Unlock()

	h.Store.UpdateMaintenance(m)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

// Dispute Handlers
func (h *DashboardsHandler) CreateDispute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	complainantID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateDisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.PropertyID == "" || req.LeaseID == "" || req.RespondentID == "" || req.Content == "" {
		http.Error(w, "PropertyID, LeaseID, RespondentID, and Content are required", http.StatusBadRequest)
		return
	}

	disp := &models.Dispute{
		ID:              uuid.New().String(),
		Type:            req.Type,
		PropertyID:      req.PropertyID,
		LeaseID:         req.LeaseID,
		ComplainantID:   complainantID,
		RespondentID:    req.RespondentID,
		SourceRefEntity: req.SourceRefEntity,
		SourceRefID:     req.SourceRefID,
		Evidence:        req.Evidence,
		Status:          models.DisputeStatusOpen,
		Messages: []models.Message{
			{SenderID: complainantID, Content: req.Content, SentAt: time.Now()},
		},
		CreatedAt: time.Now(),
	}

	h.Store.CreateDispute(disp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(disp)
}

func (h *DashboardsHandler) ListDisputes(w http.ResponseWriter, r *http.Request) {
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

	h.Store.RLock()
	defer h.Store.RUnlock()

	var list []*models.Dispute
	if u.Role == models.RoleSuperAdmin {
		list = h.Store.GetAllDisputes()
	} else if u.Role == models.RoleTechAdmin {
		for _, d := range h.Store.Disputes {
			if d.Escalated && d.Department == "technical" {
				list = append(list, d)
			}
		}
	} else if u.Role == models.RoleBillingAdmin {
		for _, d := range h.Store.Disputes {
			if d.Escalated && d.Department == "billing" {
				list = append(list, d)
			}
		}
	} else if u.Role == models.RoleSupportAdmin {
		for _, d := range h.Store.Disputes {
			if d.Escalated && (d.Department == "support" || d.Department == "") {
				list = append(list, d)
			}
		}
	} else {
		for _, d := range h.Store.Disputes {
			if d.ComplainantID == userID || d.RespondentID == userID {
				list = append(list, d)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *DashboardsHandler) AddDisputeMessage(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "User details not found", http.StatusUnauthorized)
		return
	}

	var req MessageDisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Protect against IDOR
	if err := CheckDisputeAccess(h.Store, req.DisputeID, userID, user.Role); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	msg := models.Message{
		SenderID: userID,
		Content:  req.Content,
		SentAt:   time.Now(),
	}

	if err := h.Store.AddDisputeMessage(req.DisputeID, msg); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Determine recipient for routing
	recipientID := ""
	h.Store.Lock()
	if d, ok := h.Store.Disputes[req.DisputeID]; ok {
		if d.ComplainantID == userID {
			recipientID = d.RespondentID
		} else {
			recipientID = d.ComplainantID
		}
	}
	h.Store.Unlock()

	// Broadcast WS message
	wsMsg := WSMessage{
		Type: "dispute_chat",
		UserID: userID,
		Payload: map[string]interface{}{
			"dispute_id":   req.DisputeID,
			"message":      msg,
			"recipient_id": recipientID,
			"sender_id":    userID,
		},
		Timestamp: time.Now(),
	}
	bytes, _ := json.Marshal(wsMsg)
	if store.Redis != nil {
		store.Redis.Publish("reos_events", string(bytes))
	} else {
		dispatchLocal(wsMsg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func (h *DashboardsHandler) ResolveDispute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	adminID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.Store.GetUserByID(adminID)
	isAdmin := err == nil && (u.Role == models.RoleSuperAdmin || u.Role == models.RoleSupportAdmin || u.Role == models.RoleBillingAdmin || u.Role == models.RoleTechAdmin)
	if !isAdmin {
		http.Error(w, "Forbidden: Admins only", http.StatusForbidden)
		return
	}

	var req ResolveDisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.Store.ResolveDispute(req.DisputeID, adminID, req.ResolutionNotes); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	disp, _ := h.Store.GetDispute(req.DisputeID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(disp)
}

func (h *DashboardsHandler) EscalateDispute(w http.ResponseWriter, r *http.Request) {
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
		DisputeID string `json:"dispute_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DisputeID == "" {
		http.Error(w, "Dispute ID is required", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	d, ok := h.Store.Disputes[req.DisputeID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Dispute not found", http.StatusNotFound)
		return
	}

	// Route based on dispute type
	dept := "support"
	if d.Type == "deposit" || d.Type == "payment" {
		dept = "billing"
	} else if d.Type == "maintenance" {
		dept = "technical"
	}

	d.Escalated = true
	d.Department = dept
	d.Status = "escalated"
	d.EscalationHistory = append(d.EscalationHistory, "Escalated by user "+userID+" to "+dept+" at "+time.Now().Format(time.RFC3339))
	h.Store.Unlock()

	h.Store.CreateDispute(d)

	// Broadcast updates via websocket
	BroadcastNotification(d.ComplainantID, "Dispute Escalated", "Your dispute has been escalated to platform "+dept+" administration.")
	BroadcastNotification(d.RespondentID, "Dispute Escalated", "A dispute from your unit has been escalated to platform "+dept+" administration.")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d)
}

// Superadmin Config Controls
func (h *DashboardsHandler) UpdateGatewayConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	adminID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.Store.GetUserByID(adminID)
	if err != nil || u.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: Admins only", http.StatusForbidden)
		return
	}

	var req GatewayConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	found := false
	for i, g := range ActiveGateways {
		if g.Name == req.Name {
			ActiveGateways[i].IsActive = req.IsActive
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "Gateway config not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ActiveGateways)
}

func (h *DashboardsHandler) ListSystemUsers(w http.ResponseWriter, r *http.Request) {
	adminID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.Store.GetUserByID(adminID)
	if err != nil || u.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: Admins only", http.StatusForbidden)
		return
	}

	h.Store.RLock()
	var list []*models.User
	for _, user := range h.Store.Users {
		list = append(list, user)
	}
	h.Store.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *DashboardsHandler) GetSMSLogs(w http.ResponseWriter, r *http.Request) {
	adminID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.Store.GetUserByID(adminID)
	if err != nil || u.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: Admins only", http.StatusForbidden)
		return
	}

	logs := h.Store.GetSMSLogs()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
