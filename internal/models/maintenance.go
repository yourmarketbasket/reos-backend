package models

import "time"

// Maintenance statuses
const (
	MaintenanceStatusReported   = "reported"
	MaintenanceStatusReviewed   = "reviewed"
	MaintenanceStatusInProgress = "in_progress"
	MaintenanceStatusCompleted  = "completed"
)

type Maintenance struct {
	ID                 string    `json:"id"`
	UnitID             string    `json:"unit_id"`
	ReportedBy         string    `json:"reported_by"`
	IssueType          string    `json:"issue_type"`
	Description        string    `json:"description"`
	ImagesBefore       []string  `json:"images_before"`
	ImagesAfter        []string  `json:"images_after"`
	CaretakerID        string    `json:"caretaker_id,omitempty"`
	Priority           string    `json:"priority"`
	CostEstimate       float64   `json:"cost_estimate"`
	FinalCost          float64   `json:"final_cost"`
	Status             string    `json:"status"`
	AssignedCrewID     string    `json:"assigned_crew_id,omitempty"`
	InvoiceURL         string    `json:"invoice_url,omitempty"`
	RequisitionAmount  float64   `json:"requisition_amount,omitempty"`
	SpentAmount        float64   `json:"spent_amount,omitempty"`
	CaretakerComments  string    `json:"caretaker_comments,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Inspection struct {
	ID            string    `json:"id"`
	LeaseID       string    `json:"lease_id"`
	UnitID        string    `json:"unit_id"`
	CaretakerID   string    `json:"caretaker_id"`
	Type          string    `json:"type"`
	ChecklistJson string    `json:"checklist_json"`
	MeterReadings float64   `json:"meter_readings"`
	Photos        []string  `json:"photos"`
	LoggedAt      time.Time `json:"logged_at"`
}

type Viewing struct {
	ID        string    `json:"id"`
	LeadID    string    `json:"lead_id"`
	StaffID   string    `json:"staff_id"`
	LoggedAt  time.Time `json:"logged_at"`
	Notes     string    `json:"notes"`
	Scheduled time.Time `json:"scheduled"`
}
