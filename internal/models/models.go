package models

import "time"

// User roles
const (
	RoleLandlord     = "landlord"
	RoleAgent        = "agent"
	RoleCaretaker    = "caretaker"
	RoleTenant       = "tenant" // client alias
	RoleClient       = "client"
	RoleSuperAdmin   = "superadmin"
	RoleTechAdmin    = "technical_admin"
	RoleSupportAdmin = "support_admin"
	RoleBillingAdmin = "billing_admin"
	RoleStaff        = "staff"
)

// Property types
const (
	PropTypeApartment   = "apartment" // flat
	PropTypeBungalow    = "bungalow"
	PropTypeVilla       = "villa"
	PropTypeTownhouse   = "townhouse"
	PropTypeMaisonette  = "maisonette"
	PropTypeDuplex      = "duplex"
	PropTypePenthouse   = "penthouse"
	PropTypeStudio      = "studio"
	PropTypeBedsitter   = "bedsitter"
	PropTypeSingleRoom  = "single_room"
	PropTypeCommercial  = "commercial"
	PropTypeOffice      = "office"
	PropTypeWarehouse   = "warehouse"
	PropTypeShopfront   = "shopfront"
	PropTypeLand        = "land"
	PropTypeAcres       = "acres"
	PropTypeHolidayHome = "holiday_home"
	PropTypeServiced    = "serviced_apartment"
	PropTypeHostel      = "hostel"
)

// Ownership types
const (
	OwnershipFreehold       = "freehold"
	OwnershipLeasehold      = "leasehold"
	OwnershipSectionalTitle = "sectional_title"
)

// Approval statuses
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
)

// Property publish statuses (landlord-controlled, independent of admin approval)
const (
	PublishStatusDraft     = "draft"
	PublishStatusPublished = "published"
)

// Unit statuses
const (
	UnitStatusAvailable        = "available"
	UnitStatusAvailableSoon    = "available_soon"
	UnitStatusReserved         = "reserved"
	UnitStatusOccupied         = "occupied"
	UnitStatusNoticeGiven      = "notice_given"
	UnitStatusUnderMaintenance = "under_maintenance"
)

// Lease statuses
const (
	LeaseStatusActive      = "active"
	LeaseStatusNoticeGiven = "notice_given"
	LeaseStatusTerminated  = "terminated"
	LeaseStatusCompleted   = "completed"
)

// Maintenance statuses
const (
	MaintenanceStatusReported   = "reported"
	MaintenanceStatusReviewed   = "reviewed"
	MaintenanceStatusInProgress = "in_progress"
	MaintenanceStatusCompleted  = "completed"
)

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

// Invitation statuses
const (
	InvitationStatusPending  = "pending"
	InvitationStatusAccepted = "accepted"
	InvitationStatusExpired  = "expired"
	InvitationStatusRevoked  = "revoked"
)

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

// Commission rule rate types & trigger events
const (
	CommissionRatePercentage = "percentage"
	CommissionRateFixed      = "fixed"

	CommissionTriggerLeaseSigned   = "lease_signed"
	CommissionTriggerRentCollected = "rent_collected"
	CommissionTriggerSaleClosed    = "sale_closed"
	CommissionTriggerBooking       = "booking_confirmed"
)

// ─────────────────────────────────────────
// Shared / embedded types
// ─────────────────────────────────────────

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

// ─────────────────────────────────────────
// Property sub-types
// ─────────────────────────────────────────

type Utilities struct {
	Water            string `json:"water"`            // borehole | municipal | shared_tank | rainwater | none
	WaterStatus      string `json:"water_status"`     // available | intermittent | unavailable
	Electricity      string `json:"electricity"`      // grid | solar | generator | hybrid | none
	ElectricBilling  string `json:"electric_billing"` // meter | shared | included | prepaid_token
	Gas              string `json:"gas"`              // piped | cylinder | none
	Internet         string `json:"internet"`         // fibre | dsl | satellite | none
	InternetProvider string `json:"internet_provider,omitempty"`
	Sewerage         string `json:"sewerage"`        // municipal | septic_tank | biodigester | none
	Garbage          string `json:"garbage"`         // municipal | private | self
	SecuritySystem   string `json:"security_system"` // cctv | manned_gate | alarm | electric_fence | combination | none
}

type NearbyFacility struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"` // school | hospital | mall | market | mosque | church | highway | airport | beach | lake | park | gym | bank | atm | bus_stop | restaurant
	DistanceKm  float64 `json:"distance_km"`
	WalkMinutes int     `json:"walk_minutes,omitempty"`
}

type PropertyImage struct {
	URL       string `json:"url"`
	Category  string `json:"category"` // exterior | interior | floor_plan | amenity | neighbourhood | drone
	Caption   string `json:"caption"`
	IsCover   bool   `json:"is_cover"`
	SortOrder int    `json:"sort_order"`
}

type PropertyReview struct {
	ID         string    `json:"id"`
	ReviewerID string    `json:"reviewer_id"`
	LeaseID    string    `json:"lease_id,omitempty"` // verified tenancy link
	Rating     float64   `json:"rating"`             // 1.0 – 5.0
	Headline   string    `json:"headline"`
	Body       string    `json:"body"`
	Tags       []string  `json:"tags"`        // e.g. "quiet", "good_water", "poor_security"
	IsVerified bool      `json:"is_verified"` // true = reviewer had a confirmed lease
	CreatedAt  time.Time `json:"created_at"`
	// Owner response
	Response   string    `json:"response,omitempty"`
	ResponseAt time.Time `json:"response_at,omitempty"`
}

type RatingsSummary struct {
	AverageRating   float64        `json:"average_rating"`
	TotalReviews    int            `json:"total_reviews"`
	VerifiedReviews int            `json:"verified_reviews"`
	Distribution    map[string]int `json:"distribution"` // "5":12, "4":3 …
}

// ─────────────────────────────────────────
// Property
// ─────────────────────────────────────────

type Property struct {
	ID        string `json:"id"`
	OwnerID   string `json:"owner_id"`
	CreatedBy string `json:"created_by"`
	RegionID  string `json:"region_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"` // URL-friendly identifier

	// Description
	Description string `json:"description"`

	// Classification
	PropertyType  string `json:"property_type"`  // apartment | bungalow | villa | …
	OwnershipType string `json:"ownership_type"` // freehold | leasehold | sectional_title
	YearBuilt     int    `json:"year_built,omitempty"`
	TotalUnits    int    `json:"total_units"`
	TotalFloors   int    `json:"total_floors,omitempty"`

	// Location
	Location      Location `json:"location"` // GeoJSON Point
	Address       string   `json:"address"`
	Neighbourhood string   `json:"neighbourhood"`
	City          string   `json:"city"`
	Country       string   `json:"country"`
	Jurisdiction  string   `json:"jurisdiction"` // county / region code

	// Location features
	IsGated        bool   `json:"is_gated"`
	IsBeachfront   bool   `json:"is_beachfront"`
	BeachDistanceM int    `json:"beach_distance_m,omitempty"`
	IsWaterfront   bool   `json:"is_waterfront"`
	LakeRiverName  string `json:"lake_river_name,omitempty"`
	AltitudeM      int    `json:"altitude_m,omitempty"` // metres above sea level
	IsGolfEstate   bool   `json:"is_golf_estate"`
	IsEcoReserve   bool   `json:"is_eco_reserve"`

	// Infrastructure & Utilities
	Utilities        Utilities `json:"utilities"`
	ParkingSpaces    int       `json:"parking_spaces"`
	ParkingType      string    `json:"parking_type"` // basement | open | street | none
	HasElevator      bool      `json:"has_elevator"`
	HasGym           bool      `json:"has_gym"`
	HasPool          bool      `json:"has_pool"`
	HasRooftop       bool      `json:"has_rooftop"`
	HasBackupPower   bool      `json:"has_backup_power"`
	HasChildPlayArea bool      `json:"has_child_play_area"`
	HasConference    bool      `json:"has_conference_room"`
	HasServiced      bool      `json:"has_serviced_units"`
	Amenities        []string  `json:"amenities"`
	Rules            []string  `json:"rules"`

	// Nearby facilities
	NearbyFacilities []NearbyFacility `json:"nearby_facilities"`

	// Media
	Images         []PropertyImage `json:"images"`
	VideoTourURL   string          `json:"video_tour_url,omitempty"`
	VirtualTourURL string          `json:"virtual_tour_url,omitempty"`
	Documents      []string        `json:"documents"`

	// Ratings & Reviews (embedded for fast reads)
	Ratings RatingsSummary   `json:"ratings"`
	Reviews []PropertyReview `json:"reviews"`

	// Approval workflow
	ApprovalStatus string     `json:"approval_status"` // pending | approved | rejected
	ApprovalNote   string     `json:"approval_note,omitempty"`
	ApprovedBy     string     `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`

	// Landlord-controlled publish lifecycle (independent of admin approval)
	PublishStatus string `json:"publish_status"` // draft | published

	// Hero carousel — landlord pays for featured placement
	IsFeatured    bool       `json:"is_featured"`
	FeaturedUntil *time.Time `json:"featured_until,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─────────────────────────────────────────
// Region
// ─────────────────────────────────────────

type Region struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Country        string       `json:"country"`
	Jurisdiction   string       `json:"jurisdiction"` // e.g. "KE-NBI"
	IsActive       bool         `json:"is_active"`
	CreatedBy      string       `json:"created_by"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	BoundaryPoints [][2]float64 `json:"boundary_points,omitempty"`
}

// ─────────────────────────────────────────
// CommissionRule (configured rates — not transaction records)
// ─────────────────────────────────────────

type CommissionRule struct {
	ID           string    `json:"id"`
	SetByID      string    `json:"set_by_id"`   // landlord or agent ID
	SetByRole    string    `json:"set_by_role"` // landlord | agent
	TargetID     string    `json:"target_id"`   // agent or staff user ID
	TargetRole   string    `json:"target_role"` // agent | staff
	PropertyID   string    `json:"property_id,omitempty"` // empty = applies to all
	RateType     string    `json:"rate_type"`   // percentage | fixed
	Rate         float64   `json:"rate"`        // e.g. 5.0 (%) or 5000.0 (KES)
	Currency     string    `json:"currency"`
	TriggerEvent string    `json:"trigger_event"` // lease_signed | rent_collected | sale_closed | booking_confirmed
	IsActive     bool      `json:"is_active"`
	Notes        string    `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ─────────────────────────────────────────
// Unit
// ─────────────────────────────────────────

type RentChange struct {
	Amount    float64   `json:"amount"`
	ChangedAt time.Time `json:"changed_at"`
}

type RepairHistory struct {
	MaintenanceID string    `json:"maintenance_id"`
	Cost          float64   `json:"cost"`
	CompletedAt   time.Time `json:"completed_at"`
}

type UnitHistory struct {
	Tenants     []string        `json:"tenants"`
	RentChanges []RentChange    `json:"rent_changes"`
	Repairs     []RepairHistory `json:"repairs"`
	Vacancies   []time.Time     `json:"vacancies"`
}

type Unit struct {
	ID             string          `json:"id"`
	PropertyID     string          `json:"property_id"`
	BuildingLabel  string          `json:"building_label"` // e.g. "Block A"
	Label          string          `json:"label"`          // e.g. "Unit 102"
	Status         string          `json:"status"`         // available, reserved, occupied, under_maintenance
	RentAmount     float64         `json:"rent_amount"`
	DepositAmount  float64         `json:"deposit_amount"`
	CurrentLeaseID string          `json:"current_lease_id,omitempty"`
	History        UnitHistory     `json:"history"`
	Images         []PropertyImage `json:"images"`
}

// ─────────────────────────────────────────
// Lease
// ─────────────────────────────────────────

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
	Status        string    `json:"status"` // active, notice_given, terminated, completed
	Documents     []string  `json:"documents"`
	SignedAt      time.Time `json:"signed_at"`
}

// ─────────────────────────────────────────
// Maintenance
// ─────────────────────────────────────────

type Maintenance struct {
	ID                 string    `json:"id"`
	UnitID             string    `json:"unit_id"`
	ReportedBy         string    `json:"reported_by"` // User ID
	IssueType          string    `json:"issue_type"`
	Description        string    `json:"description"`
	ImagesBefore       []string  `json:"images_before"`
	ImagesAfter        []string  `json:"images_after"`
	CaretakerID        string    `json:"caretaker_id,omitempty"`
	Priority           string    `json:"priority"` // low, medium, high
	CostEstimate       float64   `json:"cost_estimate"`
	FinalCost          float64   `json:"final_cost"`
	Status             string    `json:"status"` // reported, reviewed, in_progress, completed
	AssignedCrewID     string    `json:"assigned_crew_id,omitempty"`
	InvoiceURL         string    `json:"invoice_url,omitempty"`
	RequisitionAmount  float64   `json:"requisition_amount,omitempty"`
	SpentAmount        float64   `json:"spent_amount,omitempty"`
	CaretakerComments  string    `json:"caretaker_comments,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ─────────────────────────────────────────
// Ledger
// ─────────────────────────────────────────

type StatusHistoryItem struct {
	Status    string    `json:"status"`
	ChangedBy string    `json:"changed_by"`
	ChangedAt time.Time `json:"changed_at"`
	SourceIP  string    `json:"source_ip"`
	Reason    string    `json:"reason"`
}

type LedgerEntry struct {
	ID                       string              `json:"id"`
	LeaseID                  string              `json:"lease_id"`
	TenantID                 string              `json:"tenant_id"`
	LandlordID               string              `json:"landlord_id"`
	Type                     string              `json:"type"`    // rent, deposit, refund, deduction, fee
	Amount                   float64             `json:"amount"`
	Currency                 string              `json:"currency"`
	GatewayUsed              string              `json:"gateway_used"`
	GatewayTransactionID     string              `json:"gateway_transaction_id"`
	IdempotencyKey           string              `json:"idempotency_key"`
	RequestSource            string              `json:"request_source"` // client, webhook, admin_manual_reconcile
	Status                   string              `json:"status"`         // pending, confirmed, failed, reversed
	Description              string              `json:"description,omitempty"`
	StatusHistory            []StatusHistoryItem `json:"status_history"`
	WebhookSignatureVerified bool                `json:"webhook_signature_verified"`
	CreatedAt                time.Time           `json:"created_at"`
}

// ─────────────────────────────────────────
// Invitation
// ─────────────────────────────────────────

type Invitation struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"`
	Email      string    `json:"email"`
	SenderID   string    `json:"sender_id"`
	PropertyID string    `json:"property_id"`
	UnitID     string    `json:"unit_id,omitempty"`
	Role       string    `json:"role"`
	Status     string    `json:"status"` // pending, accepted, expired, revoked
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// ─────────────────────────────────────────
// Dispute
// ─────────────────────────────────────────

type Message struct {
	SenderID string    `json:"sender_id"`
	Content  string    `json:"content"`
	SentAt   time.Time `json:"sent_at"`
}

type Dispute struct {
	ID                string    `json:"id"`
	Type              string    `json:"type"` // deposit, maintenance, payment, lease_breach, other
	PropertyID        string    `json:"property_id"`
	LeaseID           string    `json:"lease_id"`
	ComplainantID     string    `json:"complainant_id"`
	RespondentID      string    `json:"respondent_id"`
	SourceRefEntity   string    `json:"source_ref_entity"`
	SourceRefID       string    `json:"source_ref_id"`
	Evidence          []string  `json:"evidence"`
	Messages          []Message `json:"messages"`
	Status            string    `json:"status"` // open, under_review, resolved, escalated
	AssignedAdminID   string    `json:"assigned_admin_id,omitempty"`
	ResolutionNotes   string    `json:"resolution_notes,omitempty"`
	Escalated         bool      `json:"escalated"`
	Department        string    `json:"department"` // billing, technical, support
	EscalationHistory []string  `json:"escalation_history"`
	ResolvedAt        time.Time `json:"resolved_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// ─────────────────────────────────────────
// SMS
// ─────────────────────────────────────────

type SMSNotification struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Phone           string    `json:"phone"`
	TemplateType    string    `json:"template_type"`
	LinkedEntityRef string    `json:"linked_entity_ref"`
	Status          string    `json:"status"` // queued, sent, delivered, failed
	SentAt          time.Time `json:"sent_at"`
}

// ─────────────────────────────────────────
// Listing (enhanced)
// ─────────────────────────────────────────

type SaleDetails struct {
	AskingPrice           float64 `json:"asking_price"`
	TitleDeedRef          string  `json:"title_deed_ref"`
	LandReferenceNumber   string  `json:"land_reference_number"`
	BookingFeeAmount      float64 `json:"booking_fee_amount"`
	ConveyancingStatus    string  `json:"conveyancing_status"`
	RequiresLicensedAgent bool    `json:"requires_licensed_agent"`
	EscrowEnabled         bool    `json:"escrow_enabled"`
}

type ShortStayDetails struct {
	NightlyRate        float64           `json:"nightly_rate"`
	MinNights          int               `json:"min_nights"`
	MaxGuests          int               `json:"max_guests"`
	CleaningFee        float64           `json:"cleaning_fee"`
	CancellationPolicy string            `json:"cancellation_policy"` // flexible | moderate | strict
	Calendar           []CalendarBlocked `json:"calendar"`
}

type CalendarBlocked struct {
	Date          string  `json:"date"`
	Available     bool    `json:"available"`
	PriceOverride float64 `json:"price_override,omitempty"`
}

type EventRentalDetails struct {
	HourlyRate        float64  `json:"hourly_rate"`
	MinHours          int      `json:"min_hours"`
	EventType         string   `json:"event_type"` // podcast | film | photoshoot | event | other
	EquipmentIncluded []string `json:"equipment_included"`
	DamageDeposit     float64  `json:"damage_deposit"`
	EarliestStart     string   `json:"earliest_start"`
	LatestEnd         string   `json:"latest_end"`
	PermitRequired    bool     `json:"permit_required"`
}

type ListingImage struct {
	URL       string `json:"url"`
	Category  string `json:"category"` // exterior | interior | floor_plan | amenity | video_thumb
	Caption   string `json:"caption"`
	SortOrder int    `json:"sort_order"`
}

type Listing struct {
	ID                    string `json:"id"`
	PropertyID            string `json:"property_id"`
	UnitID                string `json:"unit_id,omitempty"`
	ApplicationReviewMode string `json:"application_review_mode"` // auto | manual
	CreatedBy  string `json:"created_by"`
	RegionID   string `json:"region_id"`

	// Core
	Title       string `json:"title"`
	Description string `json:"description"`
	ListingType string `json:"listing_type"` // rental | sale | short_stay | event_hourly | land_sale | coworking | storage
	Status      string `json:"status"`       // draft | published | under_offer | reserved | closed | delisted

	// Unit details
	Bedrooms      int     `json:"bedrooms"`
	Bathrooms     int     `json:"bathrooms"`
	SizeM2        float64 `json:"size_m2"`
	Furnished     string  `json:"furnished"` // furnished | semi_furnished | unfurnished
	PetFriendly   bool    `json:"pet_friendly"`
	ParkingSpaces int     `json:"parking_spaces"`
	Floor         int     `json:"floor"`

	// Pricing
	RentAmount    float64 `json:"rent_amount,omitempty"`
	DepositAmount float64 `json:"deposit_amount,omitempty"`
	ServiceCharge float64 `json:"service_charge,omitempty"`

	// Type-specific details
	SaleDetails        *SaleDetails        `json:"sale_details,omitempty"`
	ShortStayDetails   *ShortStayDetails   `json:"short_stay_details,omitempty"`
	EventRentalDetails *EventRentalDetails `json:"event_rental_details,omitempty"`

	EscrowRequired bool `json:"escrow_required"`

	// Amenities
	Amenities []string `json:"amenities"`

	// Media — categorized images
	Images   []ListingImage `json:"images"`
	VideoURL string         `json:"video_url,omitempty"`

	// Approval
	ApprovalStatus string     `json:"approval_status"` // draft | pending_review | approved | rejected
	ApprovalNote   string     `json:"approval_note,omitempty"`
	ApprovedBy     string     `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`

	SubmitForReviewAt time.Time  `json:"submit_for_review_at"`
	RejectionReason   string     `json:"rejection_reason,omitempty"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─────────────────────────────────────────
// Booking
// ─────────────────────────────────────────

type Booking struct {
	ID                 string    `json:"id"`
	ListingID          string    `json:"listing_id"`
	ClientID           string    `json:"client_id"`
	StartDatetime      time.Time `json:"start_datetime"`
	EndDatetime        time.Time `json:"end_datetime"`
	TotalPrice         float64   `json:"total_price"`
	DepositHeld        float64   `json:"deposit_held"`
	Status             string    `json:"status"` // pending | confirmed | cancelled | completed
	CancellationReason string    `json:"cancellation_reason,omitempty"`
	RefundAmount       float64   `json:"refund_amount,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// ─────────────────────────────────────────
// Tier / Verification
// ─────────────────────────────────────────

type KYCDocumentReq struct {
	DocType     string `json:"doc_type"`
	Description string `json:"description"`
}

type TierDefinition struct {
	ID                   string           `json:"id"`
	Level                int              `json:"level"`
	Name                 string           `json:"name"`
	IsActive             bool             `json:"is_active"`
	CostAmount           float64          `json:"cost_amount"`
	Currency             string           `json:"currency"`
	Recurring            bool             `json:"recurring"`
	RecurringPeriod      string           `json:"recurring_period"` // monthly | annual | none
	PropertyCap          int              `json:"property_cap"`     // 0 = unlimited
	UnlockedListingTypes []string         `json:"unlocked_listing_types"`
	RequiredKYCDocuments []KYCDocumentReq `json:"required_kyc_documents"`
	RequiresLicenseType  string           `json:"requires_license_type,omitempty"`
	CreatedBy            string           `json:"created_by"`
	UpdatedBy            string           `json:"updated_by"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

type GrantedSnapshot struct {
	TierDefinitionID     string    `json:"tier_definition_id"`
	PropertyCap          int       `json:"property_cap"`
	UnlockedListingTypes []string  `json:"unlocked_listing_types"`
	GrantedAt            time.Time `json:"granted_at"`
}

type KYCDocument struct {
	DocType         string    `json:"doc_type"`
	TierTarget      int       `json:"tier_target"`
	Status          string    `json:"status"` // pending | approved | rejected
	RejectionReason string    `json:"rejection_reason,omitempty"`
	UploadedAt      time.Time `json:"uploaded_at"`
	ReviewedBy      string    `json:"reviewed_by,omitempty"`
}

type TierHistoryItem struct {
	TierDefinitionID string    `json:"tier_definition_id"`
	Level            int       `json:"level"`
	AchievedAt       time.Time `json:"achieved_at"`
	VerifiedBy       string    `json:"verified_by"`
	PaymentRef       string    `json:"payment_ref"`
}

type LandlordVerification struct {
	ID               string            `json:"id"`
	UserID           string            `json:"user_id"`
	CurrentTierLevel int               `json:"current_tier_level"`
	GrantedSnapshot  GrantedSnapshot   `json:"granted_snapshot"`
	TierHistory      []TierHistoryItem `json:"tier_history"`
	KYCDocuments     []KYCDocument     `json:"kyc_documents"`
	PropertyCount    int               `json:"property_count"`
	Status           string            `json:"status"` // active | under_review | suspended
}

// ─────────────────────────────────────────
// Staff / Team
// ─────────────────────────────────────────

type StaffMembership struct {
	ID                 string    `json:"id"`
	StaffUserID        string    `json:"staff_user_id"`
	PrincipalID        string    `json:"principal_id"`
	PrincipalType      string    `json:"principal_type"` // landlord | agent
	AssignedProperties []string  `json:"assigned_properties"`
	AssignedRegions    []string  `json:"assigned_regions"`
	CanAutoPublish     bool      `json:"can_auto_publish"`
	Status             string    `json:"status"` // invited | active | suspended | removed
	InvitedAt          time.Time `json:"invited_at"`
	AcceptedAt         time.Time `json:"accepted_at,omitempty"`
}

type PrincipalChainItem struct {
	PrincipalID   string `json:"principal_id"`
	PrincipalType string `json:"principal_type"`
}

type TeamAction struct {
	ID             string               `json:"id"`
	StaffID        string               `json:"staff_id"`
	ActionType     string               `json:"action_type"` // property_created | listing_edited | lead_updated
	RefID          string               `json:"ref_id"`
	ReviewStatus   string               `json:"review_status"` // pending | approved | rejected
	ReviewedBy     string               `json:"reviewed_by,omitempty"`
	ReviewedAt     time.Time            `json:"reviewed_at,omitempty"`
	PrincipalChain []PrincipalChainItem `json:"principal_chain"`
	OverrideBy     string               `json:"override_by,omitempty"`
	OverrideReason string               `json:"override_reason,omitempty"`
}

type Lead struct {
	ID              string    `json:"id"`
	PrincipalID     string    `json:"principal_id"`
	PrincipalType   string    `json:"principal_type"` // landlord | agent
	AssignedStaffID string    `json:"assigned_staff_id,omitempty"`
	ClientID        string    `json:"client_id"`
	ListingID       string    `json:"listing_id"`
	Source          string    `json:"source"`
	Status          string    `json:"status"` // new | contacted | viewing_scheduled | converted | lost
	CreatedAt       time.Time `json:"created_at"`
	LastActivityAt  time.Time `json:"last_activity_at"`
}

// Commission (earned transaction record)
type Commission struct {
	ID             string    `json:"id"`
	PrincipalID    string    `json:"principal_id"`
	PrincipalType  string    `json:"principal_type"` // landlord | agent
	StaffID        string    `json:"staff_id,omitempty"`
	LeadID         string    `json:"lead_id"`
	ListingID      string    `json:"listing_id"`
	TransactionRef string    `json:"transaction_ref"`
	Amount         float64   `json:"amount"`
	Status         string    `json:"status"` // pending | paid
	PaidAt         time.Time `json:"paid_at,omitempty"`
}

type ContextRef struct {
	Type  string `json:"type"` // lead | viewing | listing
	RefID string `json:"ref_id"`
}

type StaffReview struct {
	ID         string     `json:"id"`
	StaffID    string     `json:"staff_id"`
	Source     string     `json:"source"` // principal | client
	Rating     float64    `json:"rating"`
	Comment    string     `json:"comment"`
	ContextRef ContextRef `json:"context_ref"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ─────────────────────────────────────────
// User
// ─────────────────────────────────────────

type User struct {
	ID                    string               `json:"id"`
	Role                  string               `json:"role"`
	IdentityVerification  string               `json:"identity_verification"`
	Phone                 string               `json:"phone"`
	Email                 string               `json:"email"`
	PasswordHash          string               `json:"-"`
	Jurisdiction          string               `json:"jurisdiction"`
	CreatedAt             time.Time            `json:"created_at"`
	Status                string               `json:"status"` // active, suspended, pending
	GoogleID              string               `json:"google_id,omitempty"`
	SubscriptionTier      string               `json:"subscription_tier"`
	SubscriptionStatus    string               `json:"subscription_status"`
	SubscriptionExpiresAt time.Time            `json:"subscription_expires_at,omitempty"`
	BankName              string               `json:"bank_name"`
	BankAccount           string               `json:"bank_account"`
	BankAccountName       string               `json:"bank_account_name"`
	MobileMoneyPhone      string               `json:"mobile_money_phone"`
	MobileMoneyName       string               `json:"mobile_money_name"`
	ProfileImage          string               `json:"profile_image"`
	ProfileImages         []string             `json:"profile_images"`
	EmailNotifications    bool                 `json:"email_notifications"`
	SMSNotifications      bool                 `json:"sms_notifications"`
	MFAEnabled            bool                 `json:"mfa_enabled"`
	MFASecret             string               `json:"mfa_secret,omitempty"`
	Passkeys              []WebAuthnCredential `json:"passkeys"`
	RecoveryPhrase        string               `json:"recovery_phrase,omitempty"`
	OTP                   string               `json:"otp,omitempty"`
	AuthProvider          string               `json:"auth_provider"`
	Sessions              []string             `json:"sessions"`
	Scope                 Scope                `json:"scope"`
}

type Jurisdiction struct {
	ID             string       `json:"id"`
	Code           string       `json:"code"` // e.g. KE-NBI
	Name           string       `json:"name"`
	Country        string       `json:"country"`
	VATRate        float64      `json:"vat_rate"` // e.g. 16.0
	WHTRate        float64      `json:"wht_rate"` // e.g. 10.0
	Active         bool         `json:"active"`
	BoundaryPoints [][2]float64 `json:"boundary_points,omitempty"`
}

type PlatformCommissionSettings struct {
	ID                  string  `json:"id"`
	BaseFeePercentage   float64 `json:"base_fee_percentage"`
	ProductionMarkupPct float64 `json:"production_markup_pct"`
	VATEnabled          bool    `json:"vat_enabled"`
	WHTEnabled          bool    `json:"wht_enabled"`
}

type VacationNotice struct {
	ID          string    `json:"id"`
	LeaseID     string    `json:"lease_id"`
	TenantID    string    `json:"tenant_id"`
	NoticeDate  time.Time `json:"notice_date"`
	MoveOutDate time.Time `json:"move_out_date"`
	Status      string    `json:"status"` // pending | approved | cancelled
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

type Application struct {
	ID            string    `json:"id"`
	ListingID     string    `json:"listing_id"`
	ListingTitle  string    `json:"listing_title"`
	TenantID      string    `json:"tenant_id"`
	TenantName    string    `json:"tenant_name"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	Employment    string    `json:"employment"`
	MonthlyIncome float64   `json:"monthly_income"`
	Occupants     int       `json:"occupants"`
	AppliedAt     time.Time `json:"applied_at"`
	Status        string    `json:"status"` // pending | approved | rejected
	MatchScore    int       `json:"match_score"`
	CreditScore   int       `json:"credit_score"`
	Notes         string    `json:"notes"`
}

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type Inspection struct {
	ID            string    `json:"id"`
	LeaseID       string    `json:"lease_id"`
	UnitID        string    `json:"unit_id"`
	CaretakerID   string    `json:"caretaker_id"`
	Type          string    `json:"type"` // move_in | move_out
	ChecklistJson string    `json:"checklist_json"` // itemized checks
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
