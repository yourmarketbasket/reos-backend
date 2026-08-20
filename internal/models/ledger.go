package models

import "time"

// Ledger entry types & statuses
const (
	LedgerTypeRent      = "rent"
	LedgerTypeDeposit   = "deposit"
	LedgerTypeRefund    = "refund"
	LedgerTypeDeduction = "deduction"
	LedgerTypeFee       = "fee"

	LedgerStatusPending   = "pending"
	LedgerStatusConfirmed = "confirmed"
	LedgerStatusFailed    = "failed"
	LedgerStatusReversed  = "reversed"
)

type LedgerEntry struct {
	ID                       string              `json:"id"`
	LeaseID                  string              `json:"lease_id"`
	TenantID                 string              `json:"tenant_id"`
	LandlordID               string              `json:"landlord_id"`
	Type                     string              `json:"type"`
	Amount                   float64             `json:"amount"`
	Currency                 string              `json:"currency"`
	GatewayUsed              string              `json:"gateway_used"`
	GatewayTransactionID     string              `json:"gateway_transaction_id"`
	IdempotencyKey           string              `json:"idempotency_key"`
	RequestSource            string              `json:"request_source"`
	Status                   string              `json:"status"`
	Description              string              `json:"description,omitempty"`
	StatusHistory            []StatusHistoryItem `json:"status_history"`
	WebhookSignatureVerified bool                `json:"webhook_signature_verified"`
	CreatedAt                time.Time           `json:"created_at"`
}
