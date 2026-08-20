package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type LedgerService struct {
	Store *store.Store
}

func NewLedgerService(s *store.Store) *LedgerService {
	return &LedgerService{Store: s}
}

func (s *LedgerService) CreateLedgerEntry(leaseID, tenantID, landlordID, entryType string, amount float64, currency, description string) (*models.LedgerEntry, error) {
	entry := &models.LedgerEntry{
		ID:             uuid.New().String(),
		LeaseID:        leaseID,
		TenantID:       tenantID,
		LandlordID:     landlordID,
		Type:           entryType,
		Amount:         amount,
		Currency:       currency,
		Status:         models.LedgerStatusPending,
		Description:    description,
		IdempotencyKey: uuid.New().String(),
		CreatedAt:      time.Now(),
		StatusHistory: []models.StatusHistoryItem{
			{
				Status:    models.LedgerStatusPending,
				ChangedBy: tenantID,
				ChangedAt: time.Now(),
				SourceIP:  "127.0.0.1",
				Reason:    "Manual ledger transaction registration",
			},
		},
	}

	s.Store.AddLedgerEntry(entry)
	return entry, nil
}
