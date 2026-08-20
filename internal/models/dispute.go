package models

import "time"

// Dispute statuses & types
const (
	DisputeStatusOpen        = "open"
	DisputeStatusUnderReview = "under_review"
	DisputeStatusResolved    = "resolved"
	DisputeStatusEscalated   = "escalated"

	DisputeTypeDeposit     = "deposit"
	DisputeTypeMaintenance = "maintenance"
	DisputeTypePayment     = "payment"
	DisputeTypeLeaseBreach = "lease_breach"
	DisputeTypeOther       = "other"
)

type Message struct {
	SenderID string    `json:"sender_id"`
	Content  string    `json:"content"`
	SentAt   time.Time `json:"sent_at"`
}

type Dispute struct {
	ID                string    `json:"id"`
	Type              string    `json:"type"`
	PropertyID        string    `json:"property_id"`
	LeaseID           string    `json:"lease_id"`
	ComplainantID     string    `json:"complainant_id"`
	RespondentID      string    `json:"respondent_id"`
	SourceRefEntity   string    `json:"source_ref_entity"`
	SourceRefID       string    `json:"source_ref_id"`
	Evidence          []string  `json:"evidence"`
	Messages          []Message `json:"messages"`
	Status            string    `json:"status"`
	AssignedAdminID   string    `json:"assigned_admin_id,omitempty"`
	ResolutionNotes   string    `json:"resolution_notes,omitempty"`
	Escalated         bool      `json:"escalated"`
	Department        string    `json:"department"`
	EscalationHistory []string  `json:"escalation_history"`
	ResolvedAt        time.Time `json:"resolved_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
