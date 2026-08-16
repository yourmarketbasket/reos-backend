package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type RegionsHandler struct {
	Store *store.Store
}

func NewRegionsHandler(s *store.Store) *RegionsHandler {
	return &RegionsHandler{Store: s}
}

func uuidGen() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *RegionsHandler) ListRegions(w http.ResponseWriter, r *http.Request) {
	regions := h.Store.GetAllRegions()
	
	// Optional query param: active_only=true
	activeOnly := r.URL.Query().Get("active_only") == "true"
	
	var list []*models.Region
	for _, reg := range regions {
		if activeOnly && !reg.IsActive {
			continue
		}
		list = append(list, reg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *RegionsHandler) CreateRegion(w http.ResponseWriter, r *http.Request) {
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

	var req models.Region
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Country == "" || req.Jurisdiction == "" {
		http.Error(w, "name, country, and jurisdiction are required fields", http.StatusBadRequest)
		return
	}

	req.ID = "region_" + uuidGen()
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()
	req.CreatedBy = userID
	req.IsActive = true

	h.Store.CreateRegion(&req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func (h *RegionsHandler) UpdateRegion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
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

	var req models.Region
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Invalid input or missing region ID", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	existing, ok := h.Store.Regions[req.ID]
	h.Store.RUnlock()

	if !ok {
		http.Error(w, "Region not found", http.StatusNotFound)
		return
	}

	existing.Name = req.Name
	existing.Country = req.Country
	existing.Jurisdiction = req.Jurisdiction
	existing.IsActive = req.IsActive
	existing.BoundaryPoints = req.BoundaryPoints
	existing.UpdatedAt = time.Now()

	h.Store.UpdateRegion(existing)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (h *RegionsHandler) ToggleRegion(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Invalid input or missing region ID", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	existing, ok := h.Store.Regions[req.ID]
	h.Store.RUnlock()

	if !ok {
		http.Error(w, "Region not found", http.StatusNotFound)
		return
	}

	existing.IsActive = !existing.IsActive
	existing.UpdatedAt = time.Now()

	h.Store.UpdateRegion(existing)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (h *RegionsHandler) DeleteRegion(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Invalid input or missing region ID", http.StatusBadRequest)
		return
	}

	h.Store.RLock()
	_, ok := h.Store.Regions[req.ID]
	h.Store.RUnlock()

	if !ok {
		http.Error(w, "Region not found", http.StatusNotFound)
		return
	}

	h.Store.DeleteRegion(req.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
