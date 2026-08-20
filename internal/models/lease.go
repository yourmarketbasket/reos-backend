package models

import "time"

// Lease statuses
const (
	LeaseStatusActive      = "active"
	LeaseStatusNoticeGiven = "notice_given"
	LeaseStatusTerminated  = "terminated"
	LeaseStatusCompleted   = "completed"
)

type Lease struct {
	ID            string    `json:"id"`
	UnitID        string    `json:"unit_id"`
	TenantID      string    `json:"tenant_id"`
	LandlordID    string    `json:"landlord_id"`
	AgentID       string    `json:"agent_id,omitempty"`
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	RentAmount    float64   `json:"rent_amount"`
	DepositAmount float64   `json:"deposit_amount"`
	Status        string    `json:"status"`
	Documents     []string  `json:"documents"`
	SignedAt      time.Time `json:"signed_at"`
}

type VacationNotice struct {
	ID          string    `json:"id"`
	LeaseID     string    `json:"lease_id"`
	TenantID    string    `json:"tenant_id"`
	NoticeDate  time.Time `json:"notice_date"`
	MoveOutDate time.Time `json:"move_out_date"`
	Status      string    `json:"status"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}
