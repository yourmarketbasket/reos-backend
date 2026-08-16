package handlers

import (
	"errors"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

// CheckPropertyOwnership validates if the authenticated user owns the specified property ID.
func CheckPropertyOwnership(s *store.Store, propertyID string, userID string) error {
	s.RLock()
	prop, ok := s.Properties[propertyID]
	s.RUnlock()
	if !ok {
		return errors.New("property not found")
	}
	if prop.OwnerID != userID {
		return errors.New("forbidden: you do not own this property resource (IDOR protection)")
	}
	return nil
}

// CheckUnitAccess validates if the landlord owns the property of this unit,
// or if the tenant has an active lease in this unit.
func CheckUnitAccess(s *store.Store, unitID string, userID string, role string) error {
	s.RLock()
	unit, ok := s.Units[unitID]
	s.RUnlock()
	if !ok {
		return errors.New("unit not found")
	}

	if role == models.RoleLandlord {
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

// CheckLeaseAccess validates if the user is either the tenant or the landlord associated with this lease.
func CheckLeaseAccess(s *store.Store, leaseID string, userID string) error {
	lease, err := s.GetLease(leaseID)
	if err != nil {
		return err
	}
	if lease.TenantID != userID && lease.LandlordID != userID {
		return errors.New("forbidden: you are not authorized to inspect this lease agreement (IDOR protection)")
	}
	return nil
}

// CheckMaintenanceAccess validates if the user is authorized to read/write this maintenance ticket.
// Tenants can access tickets they filed; Caretakers can access tickets assigned to them; Landlords can access tickets in properties they own.
func CheckMaintenanceAccess(s *store.Store, maintID string, userID string, role string) error {
	m, err := s.GetMaintenance(maintID)
	if err != nil {
		return err
	}

	if role == models.RoleTenant {
		if m.ReportedBy != userID {
			return errors.New("forbidden: you did not file this maintenance ticket (IDOR protection)")
		}
	} else if role == models.RoleCaretaker {
		if m.CaretakerID != userID {
			return errors.New("forbidden: you are not designated to resolve this work order (IDOR protection)")
		}
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
