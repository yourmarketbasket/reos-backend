package handlers

import (
	"errors"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

func ResolveUltimateOwnerID(s *store.Store, userID string) string {
	currentID := userID
	visited := make(map[string]bool)
	for {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		s.RLock()
		u, err := s.GetUserByID(currentID)
		s.RUnlock()
		if err != nil {
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

// CheckUnitAccess validates if the landlord/staff owns the property of this unit,
// or if the tenant has an active lease in this unit.
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

// CheckLeaseAccess validates if the user is either the tenant or the landlord/staff associated with this lease.
func CheckLeaseAccess(s *store.Store, leaseID string, userID string) error {
	lease, err := s.GetLease(leaseID)
	if err != nil {
		return err
	}
	if lease.TenantID != userID && lease.LandlordID != userID {
		// Check if the user is staff or caretaker of the landlord
		isStaff := false
		memberships := s.GetAllStaffMemberships()
		for _, m := range memberships {
			if m.StaffUserID == userID && m.PrincipalID == lease.LandlordID && m.Status == "active" {
				if len(m.AssignedProperties) == 0 {
					isStaff = true
					break
				}
				// Find property associated with this lease's unit
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

// CheckMaintenanceAccess validates if the user is authorized to read/write this maintenance ticket.
// Tenants can access tickets they filed; Caretakers can access tickets assigned to them; Landlords/Staff can access tickets in properties they own/manage.
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
		// First check if they are the designated caretaker
		if m.CaretakerID == userID {
			return nil
		}
		// Otherwise check if they are staff managing the property
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

// CheckDisputeAccess validates that only the dispute complainant, respondent, or a superadmin can view or message a dispute thread.
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
