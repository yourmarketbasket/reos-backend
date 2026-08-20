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

type StaffReview struct {
	ID         string     `json:"id"`
	StaffID    string     `json:"staff_id"`
	Source     string     `json:"source"` // principal | client
	Rating     float64    `json:"rating"`
	Comment    string     `json:"comment"`
	ContextRef ContextRef `json:"context_ref"`
	CreatedAt  time.Time  `json:"created_at"`
}
