package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

// RequireRole checks if the authenticated user has one of the allowed roles
func RequireRole(s *store.Store, allowedRoles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, err := GetUserIdFromAuthHeader(r, s)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := s.GetUserByID(userID)
			if err != nil {
				http.Error(w, "User identity not found", http.StatusUnauthorized)
				return
			}

			roleAllowed := false
			for _, role := range allowedRoles {
				if user.Role == role {
					roleAllowed = true
					break
				}
			}

			if !roleAllowed {
				http.Error(w, fmt.Sprintf("Forbidden: this endpoint requires role permissions of %s", strings.Join(allowedRoles, ", ")), http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}

// RequireSubscriptionTier checks if the authenticated landlord has at least the required plan tier
func RequireSubscriptionTier(s *store.Store, minTier string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, err := GetUserIdFromAuthHeader(r, s)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := s.GetUserByID(userID)
			if err != nil {
				http.Error(w, "User identity not found", http.StatusUnauthorized)
				return
			}

			if user.Role == "landlord" {
				tierWeights := map[string]int{
					"":         1,
					"free":     1,
					"standard": 2,
					"premium":  3,
				}

				currentWeight := tierWeights[user.SubscriptionTier]
				requiredWeight := tierWeights[minTier]

				if currentWeight < requiredWeight {
					http.Error(w, fmt.Sprintf("Forbidden: access to this premium feature requires subscription tier '%s' or higher", minTier), http.StatusPaymentRequired)
					return
				}

				if user.SubscriptionStatus != "active" && user.SubscriptionStatus != "trialing" && user.SubscriptionTier != "free" {
					http.Error(w, "Payment Required: your subscription status is past due or inactive", http.StatusPaymentRequired)
					return
				}
			}

			next(w, r)
		}
	}
}

// ResolveUltimateOwnerID resolves the ultimate principal/landlord for staff/agents/caretakers
func ResolveUltimateOwnerID(s *store.Store, userID string) string {
	currentID := userID
	visited := make(map[string]bool)
	for {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		s.RLock()
		u, ok := s.Users[currentID]
		s.RUnlock()
		if !ok {
			break
		}

		if u.Role != models.RoleStaff && u.Role != models.RoleCaretaker && u.Role != models.RoleAgent {
			break
		}

		memberships := s.GetAllStaffMemberships()
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

// CheckPropertyOwnership validates if user owns or is staff authorized for the property
func CheckPropertyOwnership(s *store.Store, propertyID string, userID string) error {
	s.RLock()
	prop, ok := s.Properties[propertyID]
	s.RUnlock()
	if !ok {
		return errors.New("property not found")
	}

	ownerLandlordID := ResolveUltimateOwnerID(s, prop.OwnerID)
	userLandlordID := ResolveUltimateOwnerID(s, userID)

	if ownerLandlordID == userLandlordID {
		if userID == ownerLandlordID {
			return nil
		}

		memberships := s.GetAllStaffMemberships()
		for _, m := range memberships {
			if m.StaffUserID == userID && m.Status == "active" {
				if len(m.AssignedProperties) == 0 {
					return nil
				}
				for _, pid := range m.AssignedProperties {
					if pid == propertyID {
						return nil
					}
				}
			}
		}
		return nil
	}

	return errors.New("forbidden: you do not own or manage this property resource (IDOR protection)")
}

// CheckUnitAccess validates if user has property access or active tenancy lease access
func CheckUnitAccess(s *store.Store, unitID string, userID string, role string) error {
	s.RLock()
	unit, ok := s.Units[unitID]
	s.RUnlock()
	if !ok {
		return errors.New("unit not found")
	}

	if role == models.RoleLandlord || role == models.RoleStaff || role == models.RoleCaretaker {
		return CheckPropertyOwnership(s, unit.PropertyID, userID)
	}

	if role == models.RoleTenant {
		hasLease := false
		s.RLock()
		for _, lease := range s.Leases {
			if lease.UnitID == unitID && lease.TenantID == userID && lease.Status == models.LeaseStatusActive {
				hasLease = true
				break
			}
		}
		s.RUnlock()
		if !hasLease {
			return errors.New("forbidden: you do not have an active lease for this unit (IDOR protection)")
		}
	}

	return nil
}

// CheckLeaseAccess validates lease access permissions
func CheckLeaseAccess(s *store.Store, leaseID string, userID string) error {
	lease, err := s.GetLease(leaseID)
	if err != nil {
		return err
	}
	if lease.TenantID != userID && lease.LandlordID != userID {
		isStaff := false
		memberships := s.GetAllStaffMemberships()
		for _, m := range memberships {
			if m.StaffUserID == userID && m.PrincipalID == lease.LandlordID && m.Status == "active" {
				if len(m.AssignedProperties) == 0 {
					isStaff = true
					break
				}
				s.RLock()
				unit, ok := s.Units[lease.UnitID]
				s.RUnlock()
				if ok {
					for _, pid := range m.AssignedProperties {
						if pid == unit.PropertyID {
							isStaff = true
							break
						}
					}
				}
				if isStaff {
					break
				}
			}
		}
		if !isStaff {
			return errors.New("forbidden: you are not authorized to inspect this lease agreement (IDOR protection)")
		}
	}
	return nil
}

// CheckMaintenanceAccess validates maintenance ticket access
func CheckMaintenanceAccess(s *store.Store, maintID string, userID string, role string) error {
	m, err := s.GetMaintenance(maintID)
	if err != nil {
		return err
	}

	if role == models.RoleTenant {
		if m.ReportedBy != userID {
			return errors.New("forbidden: you did not file this maintenance ticket (IDOR protection)")
		}
	} else if role == models.RoleCaretaker || role == models.RoleStaff {
		if m.CaretakerID == userID {
			return nil
		}
		s.RLock()
		unit, unitExists := s.Units[m.UnitID]
		s.RUnlock()
		if !unitExists {
			return errors.New("unit associated with ticket not found")
		}
		return CheckPropertyOwnership(s, unit.PropertyID, userID)
	} else if role == models.RoleLandlord {
		s.RLock()
		unit, unitExists := s.Units[m.UnitID]
		s.RUnlock()
		if !unitExists {
			return errors.New("unit associated with ticket not found")
		}
		return CheckPropertyOwnership(s, unit.PropertyID, userID)
	}

	return nil
}

// CheckDisputeAccess validates dispute context access
func CheckDisputeAccess(s *store.Store, disputeID string, userID string, role string) error {
	if role == models.RoleSuperAdmin {
		return nil
	}

	d, err := s.GetDispute(disputeID)
	if err != nil {
		return err
	}

	if d.ComplainantID != userID && d.RespondentID != userID {
		return errors.New("forbidden: you are not an authorized party in this dispute context (IDOR protection)")
	}
	return nil
}
