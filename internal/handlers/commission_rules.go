package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type CommissionRulesHandler struct {
	Store *store.Store
}

func NewCommissionRulesHandler(s *store.Store) *CommissionRulesHandler {
	return &CommissionRulesHandler{Store: s}
}

func (h *CommissionRulesHandler) ListCommissionRules(w http.ResponseWriter, r *http.Request) {
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

	rules := h.Store.GetAllCommissionRules()
	var filtered []*models.CommissionRule

	for _, rule := range rules {
		// Superadmin sees all
		if user.Role == models.RoleSuperAdmin {
			filtered = append(filtered, rule)
			continue
		}

		// Landlords see rules they set or that target their agents
		if user.Role == models.RoleLandlord {
			if rule.SetByID == userID || rule.TargetID == userID {
				filtered = append(filtered, rule)
			}
			continue
		}

		// Agents see rules targeting them or rules they set targeting staff
		if user.Role == models.RoleAgent {
			if rule.TargetID == userID || rule.SetByID == userID {
				filtered = append(filtered, rule)
			}
			continue
		}

		// Staff see rules targeting them
		if user.Role == models.RoleStaff {
			if rule.TargetID == userID {
				filtered = append(filtered, rule)
			}
			continue
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

func (h *CommissionRulesHandler) CreateCommissionRule(w http.ResponseWriter, r *http.Request) {
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

	// Only landlord, agent, or superadmin can configure rules
	if user.Role != models.RoleLandlord && user.Role != models.RoleAgent && user.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: only landlords, agents, or admin can set commission rules", http.StatusForbidden)
		return
	}

	var req models.CommissionRule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if req.TargetID == "" || req.RateType == "" || req.TriggerEvent == "" {
		http.Error(w, "target_id, rate_type, and trigger_event are required", http.StatusBadRequest)
		return
	}

	// Verify target user exists and get their role
	targetUser, err := h.Store.GetUserByID(req.TargetID)
	if err != nil {
		http.Error(w, "Target user not found", http.StatusBadRequest)
		return
	}

	// Rules validation
	if user.Role == models.RoleLandlord {
		// Landlord sets commissions for agents/caretakers/staff
		if targetUser.Role != models.RoleAgent && targetUser.Role != models.RoleCaretaker && targetUser.Role != models.RoleStaff {
			http.Error(w, "Landlord can only configure commissions for Agent, Caretaker, or Staff", http.StatusForbidden)
			return
		}
	} else if user.Role == models.RoleAgent {
		// Agent sets commissions for staff/caretakers
		if targetUser.Role != models.RoleStaff && targetUser.Role != models.RoleCaretaker {
			http.Error(w, "Agent can only configure commissions for Caretaker or Staff", http.StatusForbidden)
			return
		}
	}

	req.ID = "rule_" + uuidGen()
	req.SetByID = userID
	req.SetByRole = user.Role
	req.TargetRole = targetUser.Role
	req.IsActive = true
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	h.Store.CreateCommissionRule(&req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func (h *CommissionRulesHandler) UpdateCommissionRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.CommissionRule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Invalid input or missing ID", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	existing, ok := h.Store.CommissionRules[req.ID]
	h.Store.RUnlock()

	if !ok {
		http.Error(w, "Commission rule not found", http.StatusNotFound)
		return
	}

	// Verify caller configured this rule or is superadmin
	caller, err := h.Store.GetUserByID(userID)
	if err != nil || (existing.SetByID != userID && caller.Role != models.RoleSuperAdmin) {
		http.Error(w, "Forbidden: you cannot edit this commission rule", http.StatusForbidden)
		return
	}

	existing.RateType = req.RateType
	existing.Rate = req.Rate
	existing.Currency = req.Currency
	existing.TriggerEvent = req.TriggerEvent
	existing.IsActive = req.IsActive
	existing.Notes = req.Notes
	existing.PropertyID = req.PropertyID
	existing.UpdatedAt = time.Now()

	h.Store.UpdateCommissionRule(existing)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (h *CommissionRulesHandler) DeleteCommissionRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Invalid input or missing ID", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	existing, ok := h.Store.CommissionRules[req.ID]
	h.Store.RUnlock()

	if !ok {
		http.Error(w, "Commission rule not found", http.StatusNotFound)
		return
	}

	caller, err := h.Store.GetUserByID(userID)
	if err != nil || (existing.SetByID != userID && caller.Role != models.RoleSuperAdmin) {
		http.Error(w, "Forbidden: you cannot delete this commission rule", http.StatusForbidden)
		return
	}

	h.Store.DeleteCommissionRule(req.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
