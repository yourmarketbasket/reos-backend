package models

import "time"

// Approval statuses
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
)

type WebAuthnCredential struct {
	ID        string    `json:"id"`
	PublicKey string    `json:"public_key"`
	AAGUID    string    `json:"aaguid"`
	SignCount uint32    `json:"sign_count"`
	CreatedAt time.Time `json:"created_at"`
}

type Scope struct {
	Type   string   `json:"type"`    // all | assigned_properties | assigned_region | own_records
	RefIDs []string `json:"ref_ids"` // e.g. specific property_ids
}

type Location struct {
	Type        string    `json:"type"`        // "Point"
	Coordinates []float64 `json:"coordinates"` // [longitude, latitude]
}

type RatingsSummary struct {
	AverageRating   float64        `json:"average_rating"`
	TotalReviews    int            `json:"total_reviews"`
	VerifiedReviews int            `json:"verified_reviews"`
	Distribution    map[string]int `json:"distribution"` // "5":12, "4":3 …
}

type StatusHistoryItem struct {
	Status    string    `json:"status"`
	ChangedBy string    `json:"changed_by"`
	ChangedAt time.Time `json:"changed_at"`
	SourceIP  string    `json:"source_ip"`
	Reason    string    `json:"reason"`
}
