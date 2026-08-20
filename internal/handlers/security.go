package handlers

import (
	"github.com/reos/api/internal/middleware"
	"github.com/reos/api/internal/store"
)

func ResolveUltimateOwnerID(s *store.Store, userID string) string {
	return middleware.ResolveUltimateOwnerID(s, userID)
}

func CheckPropertyOwnership(s *store.Store, propertyID string, userID string) error {
	return middleware.CheckPropertyOwnership(s, propertyID, userID)
}

func CheckUnitAccess(s *store.Store, unitID string, userID string, role string) error {
	return middleware.CheckUnitAccess(s, unitID, userID, role)
}

func CheckLeaseAccess(s *store.Store, leaseID string, userID string) error {
	return middleware.CheckLeaseAccess(s, leaseID, userID)
}

func CheckMaintenanceAccess(s *store.Store, maintID string, userID string, role string) error {
	return middleware.CheckMaintenanceAccess(s, maintID, userID, role)
}

func CheckDisputeAccess(s *store.Store, disputeID string, userID string, role string) error {
	return middleware.CheckDisputeAccess(s, disputeID, userID, role)
}
