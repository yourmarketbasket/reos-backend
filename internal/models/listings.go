package models

import "time"

// Commission rule rate types & trigger events
const (
	CommissionRatePercentage = "percentage"
	CommissionRateFixed      = "fixed"

	CommissionTriggerLeaseSigned   = "lease_signed"
	CommissionTriggerRentCollected = "rent_collected"
	CommissionTriggerSaleClosed    = "sale_closed"
	CommissionTriggerBooking       = "booking_confirmed"
)

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
	CancellationPolicy string            `json:"cancellation_policy"`
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
	EventType         string   `json:"event_type"`
	EquipmentIncluded []string `json:"equipment_included"`
	DamageDeposit     float64  `json:"damage_deposit"`
	EarliestStart     string   `json:"earliest_start"`
	LatestEnd         string   `json:"latest_end"`
	PermitRequired    bool     `json:"permit_required"`
}

type ListingImage struct {
	URL       string `json:"url"`
	Category  string `json:"category"`
	Caption   string `json:"caption"`
	SortOrder int    `json:"sort_order"`
}

type Listing struct {
	ID                    string `json:"id"`
	PropertyID            string `json:"property_id"`
	UnitID                string `json:"unit_id,omitempty"`
	ApplicationReviewMode string `json:"application_review_mode"`
	CreatedBy             string `json:"created_by"`
	RegionID              string `json:"region_id"`

	Title       string `json:"title"`
	Description string `json:"description"`
	ListingType string `json:"listing_type"`
	Status      string `json:"status"`

	Bedrooms      int     `json:"bedrooms"`
	Bathrooms     int     `json:"bathrooms"`
	SizeM2        float64 `json:"size_m2"`
	Furnished     string  `json:"furnished"`
	PetFriendly   bool    `json:"pet_friendly"`
	ParkingSpaces int     `json:"parking_spaces"`
	Floor         int     `json:"floor"`

	RentAmount    float64 `json:"rent_amount,omitempty"`
	DepositAmount float64 `json:"deposit_amount,omitempty"`
	ServiceCharge float64 `json:"service_charge,omitempty"`

	SaleDetails        *SaleDetails        `json:"sale_details,omitempty"`
	ShortStayDetails   *ShortStayDetails   `json:"short_stay_details,omitempty"`
	EventRentalDetails *EventRentalDetails `json:"event_rental_details,omitempty"`

	EscrowRequired bool `json:"escrow_required"`

	Amenities []string       `json:"amenities"`
	Images    []ListingImage `json:"images"`
	VideoURL  string         `json:"video_url,omitempty"`

	ApprovalStatus string     `json:"approval_status"`
	ApprovalNote   string     `json:"approval_note,omitempty"`
	ApprovedBy     string     `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`

	SubmitForReviewAt time.Time  `json:"submit_for_review_at"`
	RejectionReason   string     `json:"rejection_reason,omitempty"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Booking struct {
	ID                 string    `json:"id"`
	ListingID          string    `json:"listing_id"`
	ClientID           string    `json:"client_id"`
	StartDatetime      time.Time `json:"start_datetime"`
	EndDatetime        time.Time `json:"end_datetime"`
	TotalPrice         float64   `json:"total_price"`
	DepositHeld        float64   `json:"deposit_held"`
	Status             string    `json:"status"`
	CancellationReason string    `json:"cancellation_reason,omitempty"`
	RefundAmount       float64   `json:"refund_amount,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

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
	RecurringPeriod      string           `json:"recurring_period"`
	PropertyCap          int              `json:"property_cap"`
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
	Status          string    `json:"status"`
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
	Status           string            `json:"status"`
}

type CommissionRule struct {
	ID           string    `json:"id"`
	SetByID      string    `json:"set_by_id"`
	SetByRole    string    `json:"set_by_role"`
	TargetID     string    `json:"target_id"`
	TargetRole   string    `json:"target_role"`
	PropertyID   string    `json:"property_id,omitempty"`
	RateType     string    `json:"rate_type"`
	Rate         float64   `json:"rate"`
	Currency     string    `json:"currency"`
	TriggerEvent string    `json:"trigger_event"`
	IsActive     bool      `json:"is_active"`
	Notes        string    `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Commission struct {
	ID             string    `json:"id"`
	PrincipalID    string    `json:"principal_id"`
	PrincipalType  string    `json:"principal_type"`
	StaffID        string    `json:"staff_id,omitempty"`
	LeadID         string    `json:"lead_id"`
	ListingID      string    `json:"listing_id"`
	TransactionRef string    `json:"transaction_ref"`
	Amount         float64   `json:"amount"`
	Status         string    `json:"status"`
	PaidAt         time.Time `json:"paid_at,omitempty"`
}

type ContextRef struct {
	Type  string `json:"type"`
	RefID string `json:"ref_id"`
}

type SMSNotification struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Phone           string    `json:"phone"`
	TemplateType    string    `json:"template_type"`
	LinkedEntityRef string    `json:"linked_entity_ref"`
	Status          string    `json:"status"`
	SentAt          time.Time `json:"sent_at"`
}

type PlatformCommissionSettings struct {
	ID                  string  `json:"id"`
	BaseFeePercentage   float64 `json:"base_fee_percentage"`
	ProductionMarkupPct float64 `json:"production_markup_pct"`
	VATEnabled          bool    `json:"vat_enabled"`
	WHTEnabled          bool    `json:"wht_enabled"`
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
	Status        string    `json:"status"`
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

type Lead struct {
	ID              string    `json:"id"`
	PrincipalID     string    `json:"principal_id"`
	PrincipalType   string    `json:"principal_type"`
	AssignedStaffID string    `json:"assigned_staff_id,omitempty"`
	ClientID        string    `json:"client_id"`
	ListingID       string    `json:"listing_id"`
	Source          string    `json:"source"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	LastActivityAt  time.Time `json:"last_activity_at"`
}
