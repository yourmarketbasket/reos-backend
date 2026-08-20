package models

import "time"

// Invitation statuses
const (
	InvitationStatusPending  = "pending"
	InvitationStatusAccepted = "accepted"
	InvitationStatusExpired  = "expired"
	InvitationStatusRevoked  = "revoked"
)

type Invitation struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"`
	Email      string    `json:"email"`
	SenderID   string    `json:"sender_id"`
	PropertyID string    `json:"property_id"`
	UnitID     string    `json:"unit_id,omitempty"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}
