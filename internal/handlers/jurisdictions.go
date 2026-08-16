package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type JurisdictionsHandler struct {
	Store *store.Store
}

func NewJurisdictionsHandler(s *store.Store) *JurisdictionsHandler {
	return &JurisdictionsHandler{Store: s}
}

func (h *JurisdictionsHandler) ListJurisdictions(w http.ResponseWriter, r *http.Request) {
	list := h.Store.GetAllJurisdictions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *JurisdictionsHandler) CreateJurisdiction(w http.ResponseWriter, r *http.Request) {
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
	if err != nil || user.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: superadmin only", http.StatusForbidden)
		return
	}

	var req models.Jurisdiction
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" || req.Name == "" || req.Country == "" {
		http.Error(w, "code, name, and country are required", http.StatusBadRequest)
		return
	}

	req.ID = "juris_" + uuid.New().String()
	req.Active = true

	h.Store.CreateJurisdiction(&req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func (h *JurisdictionsHandler) UpdateJurisdiction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil || user.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: superadmin only", http.StatusForbidden)
		return
	}

	var req models.Jurisdiction
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Invalid request or missing ID", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	existing, ok := h.Store.Jurisdictions[req.ID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Jurisdiction not found", http.StatusNotFound)
		return
	}

	existing.Code = req.Code
	existing.Name = req.Name
	existing.Country = req.Country
	existing.VATRate = req.VATRate
	existing.WHTRate = req.WHTRate
	existing.Active = req.Active
	existing.BoundaryPoints = req.BoundaryPoints
	h.Store.Unlock()

	h.Store.UpdateJurisdiction(existing)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (h *JurisdictionsHandler) ToggleJurisdiction(w http.ResponseWriter, r *http.Request) {
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
	if err != nil || user.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: superadmin only", http.StatusForbidden)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	h.Store.Lock()
	existing, ok := h.Store.Jurisdictions[req.ID]
	if !ok {
		h.Store.Unlock()
		http.Error(w, "Jurisdiction not found", http.StatusNotFound)
		return
	}

	existing.Active = !existing.Active
	h.Store.Unlock()

	h.Store.UpdateJurisdiction(existing)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (h *JurisdictionsHandler) DeleteJurisdiction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil || user.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: superadmin only", http.StatusForbidden)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	h.Store.DeleteJurisdiction(req.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// Platform Commission settings Handlers
func (h *JurisdictionsHandler) GetCommissionSettings(w http.ResponseWriter, r *http.Request) {
	pc := h.Store.GetPlatformCommissionSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pc)
}

func (h *JurisdictionsHandler) SaveCommissionSettings(w http.ResponseWriter, r *http.Request) {
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
	if err != nil || user.Role != models.RoleSuperAdmin {
		http.Error(w, "Forbidden: superadmin only", http.StatusForbidden)
		return
	}

	var req models.PlatformCommissionSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = "default-comm" // Singleton settings record
	h.Store.SavePlatformCommissionSettings(&req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}
