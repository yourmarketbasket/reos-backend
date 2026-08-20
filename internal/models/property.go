package models

import "time"

// Property types
const (
	PropTypeApartment   = "apartment"
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

// Property publish statuses
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

type Property struct {
	ID        string `json:"id"`
	OwnerID   string `json:"owner_id"`
	CreatedBy string `json:"created_by"`
	RegionID  string `json:"region_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`

	Description   string `json:"description"`
	PropertyType  string `json:"property_type"`
	OwnershipType string `json:"ownership_type"`
	YearBuilt     int    `json:"year_built,omitempty"`
	TotalUnits    int    `json:"total_units"`
	TotalFloors   int    `json:"total_floors,omitempty"`

	Location      Location `json:"location"`
	Address       string   `json:"address"`
	Neighbourhood string   `json:"neighbourhood"`
	City          string   `json:"city"`
	Country       string   `json:"country"`
	Jurisdiction  string   `json:"jurisdiction"`

	IsGated        bool   `json:"is_gated"`
	IsBeachfront   bool   `json:"is_beachfront"`
	BeachDistanceM int    `json:"beach_distance_m,omitempty"`
	IsWaterfront   bool   `json:"is_waterfront"`
	LakeRiverName  string `json:"lake_river_name,omitempty"`
	AltitudeM      int    `json:"altitude_m,omitempty"`
	IsGolfEstate   bool   `json:"is_golf_estate"`
	IsEcoReserve   bool   `json:"is_eco_reserve"`

	Utilities        Utilities `json:"utilities"`
	ParkingSpaces    int       `json:"parking_spaces"`
	ParkingType      string    `json:"parking_type"`
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

	NearbyFacilities []NearbyFacility `json:"nearby_facilities"`
	Images           []PropertyImage  `json:"images"`
	VideoTourURL     string           `json:"video_tour_url,omitempty"`
	VirtualTourURL   string           `json:"virtual_tour_url,omitempty"`
	Documents        []string         `json:"documents"`

	Ratings RatingsSummary   `json:"ratings"`
	Reviews []PropertyReview `json:"reviews"`

	ApprovalStatus string     `json:"approval_status"`
	ApprovalNote   string     `json:"approval_note,omitempty"`
	ApprovedBy     string     `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`

	PublishStatus string     `json:"publish_status"`
	IsFeatured    bool       `json:"is_featured"`
	FeaturedUntil *time.Time `json:"featured_until,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Utilities struct {
	Water            string `json:"water"`
	WaterStatus      string `json:"water_status"`
	Electricity      string `json:"electricity"`
	ElectricBilling  string `json:"electric_billing"`
	Gas              string `json:"gas"`
	Internet         string `json:"internet"`
	InternetProvider string `json:"internet_provider,omitempty"`
	Sewerage         string `json:"sewerage"`
	Garbage          string `json:"garbage"`
	SecuritySystem   string `json:"security_system"`
}

type NearbyFacility struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	DistanceKm  float64 `json:"distance_km"`
	WalkMinutes int     `json:"walk_minutes,omitempty"`
}

type PropertyImage struct {
	URL       string `json:"url"`
	Category  string `json:"category"`
	Caption   string `json:"caption"`
	IsCover   bool   `json:"is_cover"`
	SortOrder int    `json:"sort_order"`
}

type PropertyReview struct {
	ID         string    `json:"id"`
	ReviewerID string    `json:"reviewer_id"`
	LeaseID    string    `json:"lease_id,omitempty"`
	Rating     float64   `json:"rating"`
	Headline   string    `json:"headline"`
	Body       string    `json:"body"`
	Tags       []string  `json:"tags"`
	IsVerified bool      `json:"is_verified"`
	CreatedAt  time.Time `json:"created_at"`
	Response   string    `json:"response,omitempty"`
	ResponseAt time.Time `json:"response_at,omitempty"`
}

type Region struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Country        string       `json:"country"`
	Jurisdiction   string       `json:"jurisdiction"`
	IsActive       bool         `json:"is_active"`
	CreatedBy      string       `json:"created_by"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	BoundaryPoints [][2]float64 `json:"boundary_points,omitempty"`
}

type Jurisdiction struct {
	ID             string       `json:"id"`
	Code           string       `json:"code"`
	Name           string       `json:"name"`
	Country        string       `json:"country"`
	VATRate        float64      `json:"vat_rate"`
	WHTRate        float64      `json:"wht_rate"`
	Active         bool         `json:"active"`
	BoundaryPoints [][2]float64 `json:"boundary_points,omitempty"`
}

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
	BuildingLabel  string          `json:"building_label"`
	Label          string          `json:"label"`
	Status         string          `json:"status"`
	RentAmount     float64         `json:"rent_amount"`
	DepositAmount  float64         `json:"deposit_amount"`
	CurrentLeaseID string          `json:"current_lease_id,omitempty"`
	History        UnitHistory     `json:"history"`
	Images         []PropertyImage `json:"images"`
}
