package store

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/reos/api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	mu           sync.RWMutex
	Users        map[string]*models.User
	Properties   map[string]*models.Property
	Units        map[string]*models.Unit
	Leases       map[string]*models.Lease
	Maintenance  map[string]*models.Maintenance
	Ledger       map[string]*models.LedgerEntry
	Invitations  map[string]*models.Invitation
	Disputes     map[string]*models.Dispute
	SMSLogs      map[string]*models.SMSNotification

	Listings              map[string]*models.Listing
	Bookings              map[string]*models.Booking
	StaffMemberships      map[string]*models.StaffMembership
	TeamActions           map[string]*models.TeamAction
	Leads                 map[string]*models.Lead
	Commissions           map[string]*models.Commission
	StaffReviews          map[string]*models.StaffReview
	TierDefinitions       map[string]*models.TierDefinition
	LandlordVerifications map[string]*models.LandlordVerification
	Regions               map[string]*models.Region
	CommissionRules       map[string]*models.CommissionRule
	Jurisdictions         map[string]*models.Jurisdiction
	PlatformCommissions   map[string]*models.PlatformCommissionSettings
	VacationNotices       map[string]*models.VacationNotice
	Applications          map[string]*models.Application
	Notifications         map[string]*models.Notification
	Inspections           map[string]*models.Inspection
	Viewings              map[string]*models.Viewing

	// MongoDB
	mongoClient *mongo.Client
	db          *mongo.Database
}

func loadEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
				val = val[1 : len(val)-1]
			}
			os.Setenv(key, val)
		}
	}
}

func NewStore() *Store {
	loadEnv(".env")
	loadEnv("../.env")

	s := &Store{
		Users:                 make(map[string]*models.User),
		Properties:            make(map[string]*models.Property),
		Units:                 make(map[string]*models.Unit),
		Leases:                make(map[string]*models.Lease),
		Maintenance:           make(map[string]*models.Maintenance),
		Ledger:                make(map[string]*models.LedgerEntry),
		Invitations:           make(map[string]*models.Invitation),
		Disputes:              make(map[string]*models.Dispute),
		SMSLogs:               make(map[string]*models.SMSNotification),
		Listings:              make(map[string]*models.Listing),
		Bookings:              make(map[string]*models.Booking),
		StaffMemberships:      make(map[string]*models.StaffMembership),
		TeamActions:           make(map[string]*models.TeamAction),
		Leads:                 make(map[string]*models.Lead),
		Commissions:           make(map[string]*models.Commission),
		StaffReviews:          make(map[string]*models.StaffReview),
		TierDefinitions:       make(map[string]*models.TierDefinition),
		LandlordVerifications: make(map[string]*models.LandlordVerification),
		Regions:               make(map[string]*models.Region),
		CommissionRules:       make(map[string]*models.CommissionRule),
		Jurisdictions:         make(map[string]*models.Jurisdiction),
		PlatformCommissions:   make(map[string]*models.PlatformCommissionSettings),
		VacationNotices:       make(map[string]*models.VacationNotice),
		Applications:          make(map[string]*models.Application),
		Notifications:         make(map[string]*models.Notification),
		Inspections:           make(map[string]*models.Inspection),
		Viewings:              make(map[string]*models.Viewing),
	}

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI != "" {
		fmt.Printf("Connecting to MongoDB at: %s...\n", mongoURI)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		clientOptions := options.Client().ApplyURI(mongoURI)
		client, err := mongo.Connect(ctx, clientOptions)
		if err == nil {
			err = client.Ping(ctx, nil)
			if err == nil {
				s.mongoClient = client
				s.db = client.Database("reos")
				fmt.Println("Successfully connected to MongoDB!")

				// Purge old preseeded credentials to enforce registering first
				purgeCtx, purgeCancel := context.WithTimeout(context.Background(), 5*time.Second)
				s.db.Collection("users").DeleteMany(purgeCtx, bson.M{"id": bson.M{"$in": []string{"admin-1", "landlord-1", "caretaker-1", "tenant-1"}}})
				s.db.Collection("properties").DeleteMany(purgeCtx, bson.M{"landlord_id": bson.M{"$in": []string{"admin-1", "landlord-1", "caretaker-1", "tenant-1"}}})
				s.db.Collection("units").DeleteMany(purgeCtx, bson.M{"property_id": bson.M{"$in": []string{"prop-1", "prop-2"}}})
				s.db.Collection("leases").DeleteMany(purgeCtx, bson.M{"landlord_id": "landlord-1"})
				s.db.Collection("maintenance").DeleteMany(purgeCtx, bson.M{"tenant_id": "tenant-1"})
				s.db.Collection("ledger").DeleteMany(purgeCtx, bson.M{"landlord_id": "landlord-1"})
				s.db.Collection("disputes").DeleteMany(purgeCtx, bson.M{"landlord_id": "landlord-1"})
				purgeCancel()

				s.LoadFromMongo()
			} else {
				fmt.Printf("MongoDB Ping failed: %v. Falling back to memory-only.\n", err)
			}
		} else {
			fmt.Printf("MongoDB connection failed: %v. Falling back to memory-only.\n", err)
		}
	} else {
		fmt.Println("No MONGODB_URI found. Falling back to memory-only database.")
	}

	return s
}

func (s *Store) LoadFromMongo() {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()

	// Users
	if cursor, err := s.db.Collection("users").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var u models.User
			if err := cursor.Decode(&u); err == nil {
				s.Users[u.ID] = &u
			}
		}
	}

	// Properties
	if cursor, err := s.db.Collection("properties").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var p models.Property
			if err := cursor.Decode(&p); err == nil {
				s.Properties[p.ID] = &p
			}
		}
	}

	// Units
	if cursor, err := s.db.Collection("units").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var u models.Unit
			if err := cursor.Decode(&u); err == nil {
				s.Units[u.ID] = &u
			}
		}
	}

	// Leases
	if cursor, err := s.db.Collection("leases").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var l models.Lease
			if err := cursor.Decode(&l); err == nil {
				s.Leases[l.ID] = &l
			}
		}
	}

	// Maintenance
	if cursor, err := s.db.Collection("maintenance").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var m models.Maintenance
			if err := cursor.Decode(&m); err == nil {
				s.Maintenance[m.ID] = &m
			}
		}
	}

	// Ledger
	if cursor, err := s.db.Collection("ledger").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var l models.LedgerEntry
			if err := cursor.Decode(&l); err == nil {
				s.Ledger[l.ID] = &l
			}
		}
	}

	// Invitations
	if cursor, err := s.db.Collection("invitations").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var i models.Invitation
			if err := cursor.Decode(&i); err == nil {
				s.Invitations[i.ID] = &i
			}
		}
	}

	// Disputes
	if cursor, err := s.db.Collection("disputes").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var d models.Dispute
			if err := cursor.Decode(&d); err == nil {
				s.Disputes[d.ID] = &d
			}
		}
	}

	// SMSLogs
	if cursor, err := s.db.Collection("sms_logs").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var l models.SMSNotification
			if err := cursor.Decode(&l); err == nil {
				s.SMSLogs[l.ID] = &l
			}
		}
	}

	// Listings
	if cursor, err := s.db.Collection("listings").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var l models.Listing
			if err := cursor.Decode(&l); err == nil {
				s.Listings[l.ID] = &l
			}
		}
	}

	// Bookings
	if cursor, err := s.db.Collection("bookings").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var b models.Booking
			if err := cursor.Decode(&b); err == nil {
				s.Bookings[b.ID] = &b
			}
		}
	}

	// Staff Memberships
	if cursor, err := s.db.Collection("staff_memberships").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var sm models.StaffMembership
			if err := cursor.Decode(&sm); err == nil {
				s.StaffMemberships[sm.ID] = &sm
			}
		}
	}

	// Team Actions
	if cursor, err := s.db.Collection("team_actions").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var ta models.TeamAction
			if err := cursor.Decode(&ta); err == nil {
				s.TeamActions[ta.ID] = &ta
			}
		}
	}

	// Leads
	if cursor, err := s.db.Collection("leads").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var l models.Lead
			if err := cursor.Decode(&l); err == nil {
				s.Leads[l.ID] = &l
			}
		}
	}

	// Commissions
	if cursor, err := s.db.Collection("commissions").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var c models.Commission
			if err := cursor.Decode(&c); err == nil {
				s.Commissions[c.ID] = &c
			}
		}
	}

	// Staff Reviews
	if cursor, err := s.db.Collection("staff_reviews").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var sr models.StaffReview
			if err := cursor.Decode(&sr); err == nil {
				s.StaffReviews[sr.ID] = &sr
			}
		}
	}

	// Tier Definitions
	if cursor, err := s.db.Collection("tier_definitions").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var td models.TierDefinition
			if err := cursor.Decode(&td); err == nil {
				s.TierDefinitions[td.ID] = &td
			}
		}
	}

	// Landlord Verifications
	if cursor, err := s.db.Collection("landlord_verifications").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var lv models.LandlordVerification
			if err := cursor.Decode(&lv); err == nil {
				s.LandlordVerifications[lv.ID] = &lv
			}
		}
	}

	// Regions
	if cursor, err := s.db.Collection("regions").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var r models.Region
			if err := cursor.Decode(&r); err == nil {
				s.Regions[r.ID] = &r
			}
		}
	}

	// Commission Rules
	if cursor, err := s.db.Collection("commission_rules").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var cr models.CommissionRule
			if err := cursor.Decode(&cr); err == nil {
				s.CommissionRules[cr.ID] = &cr
			}
		}
	}

	// Jurisdictions
	if cursor, err := s.db.Collection("jurisdictions").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var j models.Jurisdiction
			if err := cursor.Decode(&j); err == nil {
				s.Jurisdictions[j.ID] = &j
			}
		}
	}

	// Platform Commissions
	if cursor, err := s.db.Collection("platform_commissions").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var pc models.PlatformCommissionSettings
			if err := cursor.Decode(&pc); err == nil {
				s.PlatformCommissions[pc.ID] = &pc
			}
		}
	}

	// Vacation Notices
	if cursor, err := s.db.Collection("vacation_notices").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var vn models.VacationNotice
			if err := cursor.Decode(&vn); err == nil {
				s.VacationNotices[vn.ID] = &vn
			}
		}
	}

	// Applications
	if cursor, err := s.db.Collection("applications").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var app models.Application
			if err := cursor.Decode(&app); err == nil {
				s.Applications[app.ID] = &app
			}
		}
	}

	// Notifications
	if cursor, err := s.db.Collection("notifications").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var n models.Notification
			if err := cursor.Decode(&n); err == nil {
				s.Notifications[n.ID] = &n
			}
		}
	}

	// Inspections
	if cursor, err := s.db.Collection("inspections").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var insp models.Inspection
			if err := cursor.Decode(&insp); err == nil {
				s.Inspections[insp.ID] = &insp
			}
		}
	}

	// Viewings
	if cursor, err := s.db.Collection("viewings").Find(ctx, bson.M{}); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var v models.Viewing
			if err := cursor.Decode(&v); err == nil {
				s.Viewings[v.ID] = &v
			}
		}
	}

	s.SeedTiers()

	fmt.Printf("Loaded existing records from MongoDB: %d users, %d properties, %d units, %d leases, %d maintenance, %d ledger entries, %d invitations, %d disputes, %d SMS logs, %d listings, %d bookings, %d staff_memberships, %d tier_definitions, %d landlord_verifications, %d regions, %d commission_rules, %d jurisdictions, %d platform_commissions, %d vacation_notices, %d applications, %d notifications, %d inspections, %d viewings.\n",
		len(s.Users), len(s.Properties), len(s.Units), len(s.Leases), len(s.Maintenance), len(s.Ledger), len(s.Invitations), len(s.Disputes), len(s.SMSLogs), len(s.Listings), len(s.Bookings), len(s.StaffMemberships), len(s.TierDefinitions), len(s.LandlordVerifications), len(s.Regions), len(s.CommissionRules), len(s.Jurisdictions), len(s.PlatformCommissions), len(s.VacationNotices), len(s.Applications), len(s.Notifications), len(s.Inspections), len(s.Viewings))
}

func (s *Store) writeDoc(collection string, id string, doc interface{}) {
	if s.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Replace().SetUpsert(true)
	filter := bson.M{"id": id}
	_, err := s.db.Collection(collection).ReplaceOne(ctx, filter, doc, opts)
	if err != nil {
		fmt.Printf("Failed to write doc to MongoDB collection '%s' (ID: %s): %v\n", collection, id, err)
	}
}



func HashPassword(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hex.EncodeToString(hasher.Sum(nil))
}



// User Actions
func (s *Store) GetUserByID(id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.Users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

func (s *Store) GetUserByEmail(email string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.Users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (s *Store) GetUserByPhone(phone string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.Users {
		if u.Phone == phone {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (s *Store) GetUserByGoogleID(googleID string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.Users {
		if u.GoogleID == googleID {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (s *Store) CreateUser(u *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check duplicates
	if u.Email != "" {
		for _, ex := range s.Users {
			if ex.Email == u.Email {
				return errors.New("email already registered")
			}
		}
	}
	if u.Phone != "" {
		for _, ex := range s.Users {
			if ex.Phone == u.Phone {
				return errors.New("phone already registered")
			}
		}
	}

	s.Users[u.ID] = u
	s.writeDoc("users", u.ID, u)
	return nil
}

// Property Actions
func (s *Store) CreateProperty(p *models.Property) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Properties[p.ID] = p
	s.writeDoc("properties", p.ID, p)
}

func (s *Store) GetPropertiesByOwner(ownerID string) []*models.Property {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Property
	for _, p := range s.Properties {
		if p.OwnerID == ownerID {
			list = append(list, p)
		}
	}
	return list
}

func (s *Store) GetAllProperties() []*models.Property {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Property
	for _, p := range s.Properties {
		list = append(list, p)
	}
	return list
}

// Unit Actions
func (s *Store) CreateUnit(u *models.Unit) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Units[u.ID] = u
	s.writeDoc("units", u.ID, u)
}

func (s *Store) GetUnitsByProperty(propertyID string) []*models.Unit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Unit
	for _, u := range s.Units {
		if u.PropertyID == propertyID {
			list = append(list, u)
		}
	}
	return list
}

func (s *Store) GetUnit(id string) (*models.Unit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.Units[id]
	if !ok {
		return nil, errors.New("unit not found")
	}
	return u, nil
}

func (s *Store) UpdateUnitStatus(id string, status string, currentLeaseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Units[id]
	if !ok {
		return errors.New("unit not found")
	}
	u.Status = status
	if currentLeaseID != "" {
		u.CurrentLeaseID = currentLeaseID
	} else if status == models.UnitStatusAvailable {
		u.CurrentLeaseID = ""
	}
	s.writeDoc("units", u.ID, u)
	return nil
}

// Lease Actions
func (s *Store) CreateLease(l *models.Lease) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Leases[l.ID] = l
	s.writeDoc("leases", l.ID, l)
}

func (s *Store) GetLease(id string) (*models.Lease, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.Leases[id]
	if !ok {
		return nil, errors.New("lease not found")
	}
	return l, nil
}

func (s *Store) GetLeasesByTenant(tenantID string) []*models.Lease {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Lease
	for _, l := range s.Leases {
		if l.TenantID == tenantID {
			list = append(list, l)
		}
	}
	return list
}

func (s *Store) GetLeasesByLandlord(landlordID string) []*models.Lease {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Lease
	for _, l := range s.Leases {
		if l.LandlordID == landlordID {
			list = append(list, l)
		}
	}
	return list
}

// Invitation Actions
func (s *Store) CreateInvitation(i *models.Invitation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Invitations[i.Token] = i
	s.writeDoc("invitations", i.ID, i)
}

func (s *Store) GetInvitationByToken(token string) (*models.Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.Invitations[token]
	if !ok {
		return nil, errors.New("invitation not found")
	}
	return i, nil
}

func (s *Store) GetInvitationsByLandlord(landlordID string) []*models.Invitation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Invitation
	for _, i := range s.Invitations {
		if i.SenderID == landlordID {
			list = append(list, i)
		}
	}
	return list
}

func (s *Store) GetAllInvitations() []*models.Invitation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Invitation
	for _, i := range s.Invitations {
		list = append(list, i)
	}
	return list
}

func (s *Store) UpdateInvitationStatus(token string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.Invitations[token]
	if !ok {
		return errors.New("invitation not found")
	}
	i.Status = status
	s.writeDoc("invitations", i.ID, i)
	return nil
}

// Maintenance Actions
func (s *Store) CreateMaintenance(m *models.Maintenance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Maintenance[m.ID] = m
	s.writeDoc("maintenance", m.ID, m)
}

func (s *Store) GetMaintenanceByUnit(unitID string) []*models.Maintenance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Maintenance
	for _, m := range s.Maintenance {
		if m.UnitID == unitID {
			list = append(list, m)
		}
	}
	return list
}

func (s *Store) GetMaintenanceByCaretaker(caretakerID string) []*models.Maintenance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Maintenance
	for _, m := range s.Maintenance {
		if m.CaretakerID == caretakerID {
			list = append(list, m)
		}
	}
	return list
}

func (s *Store) GetMaintenance(id string) (*models.Maintenance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.Maintenance[id]
	if !ok {
		return nil, errors.New("maintenance record not found")
	}
	return m, nil
}

func (s *Store) UpdateMaintenance(m *models.Maintenance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Maintenance[m.ID] = m
	s.writeDoc("maintenance", m.ID, m)
}

// Ledger Actions (Append Only)
func (s *Store) AddLedgerEntry(l *models.LedgerEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Ledger[l.ID] = l
	s.writeDoc("ledger", l.ID, l)
}

func (s *Store) GetLedgerByLease(leaseID string) []*models.LedgerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.LedgerEntry
	for _, l := range s.Ledger {
		if l.LeaseID == leaseID {
			list = append(list, l)
		}
	}
	return list
}

func (s *Store) GetLedgerByTenant(tenantID string) []*models.LedgerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.LedgerEntry
	for _, l := range s.Ledger {
		if l.TenantID == tenantID {
			list = append(list, l)
		}
	}
	return list
}

func (s *Store) GetLedgerByLandlord(landlordID string) []*models.LedgerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.LedgerEntry
	for _, l := range s.Ledger {
		if l.LandlordID == landlordID {
			list = append(list, l)
		}
	}
	return list
}

func (s *Store) GetAllLedger() []*models.LedgerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.LedgerEntry
	for _, l := range s.Ledger {
		list = append(list, l)
	}
	return list
}

// Dispute Actions
func (s *Store) CreateDispute(d *models.Dispute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Disputes[d.ID] = d
	s.writeDoc("disputes", d.ID, d)
}

func (s *Store) GetDispute(id string) (*models.Dispute, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.Disputes[id]
	if !ok {
		return nil, errors.New("dispute not found")
	}
	return d, nil
}

func (s *Store) GetDisputesByProperty(propertyID string) []*models.Dispute {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Dispute
	for _, d := range s.Disputes {
		if d.PropertyID == propertyID {
			list = append(list, d)
		}
	}
	return list
}

func (s *Store) GetAllDisputes() []*models.Dispute {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Dispute
	for _, d := range s.Disputes {
		list = append(list, d)
	}
	return list
}

func (s *Store) AddDisputeMessage(disputeID string, msg models.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.Disputes[disputeID]
	if !ok {
		return errors.New("dispute not found")
	}
	d.Messages = append(d.Messages, msg)
	s.writeDoc("disputes", d.ID, d)
	return nil
}

func (s *Store) ResolveDispute(disputeID string, adminID string, notes string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.Disputes[disputeID]
	if !ok {
		return errors.New("dispute not found")
	}
	d.Status = models.DisputeStatusResolved
	d.AssignedAdminID = adminID
	d.ResolutionNotes = notes
	d.ResolvedAt = time.Now()
	s.writeDoc("disputes", d.ID, d)
	return nil
}

// SMS Actions
func (s *Store) AddSMSLog(log *models.SMSNotification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SMSLogs[log.ID] = log
	s.writeDoc("sms_logs", log.ID, log)
}

func (s *Store) GetSMSLogs() []*models.SMSNotification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.SMSNotification
	for _, log := range s.SMSLogs {
		list = append(list, log)
	}
	return list
}

func (s *Store) UpdateUser(u *models.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Users[u.ID] = u
	s.writeDoc("users", u.ID, u)
}

func (s *Store) Lock() {
	s.mu.Lock()
}

func (s *Store) Unlock() {
	s.mu.Unlock()
}

func (s *Store) RLock() {
	s.mu.RLock()
}

func (s *Store) RUnlock() {
	s.mu.RUnlock()
}

func (s *Store) SeedTiers() {
	if len(s.TierDefinitions) > 0 {
		return
	}

	// Level 1: Free
	t1 := &models.TierDefinition{
		ID:                   "tier_free",
		Level:                1,
		Name:                 "Free Standard Entry",
		IsActive:             true,
		CostAmount:           0.0,
		Currency:             "KES",
		Recurring:            false,
		PropertyCap:          100,
		UnlockedListingTypes: []string{"rental", "storage"},
		RequiredKYCDocuments: []models.KYCDocumentReq{
			{DocType: "national_id", Description: "National ID Card or Passport Scan"},
			{DocType: "phone_verification", Description: "OTP Verification of Phone"},
		},
		UpdatedAt:            time.Now(),
	}

	// Level 2: Verified Host
	t2 := &models.TierDefinition{
		ID:                   "tier_verified",
		Level:                2,
		Name:                 "Verified Host (Paid Upgrade)",
		IsActive:             true,
		CostAmount:           5000.0,
		Currency:             "KES",
		Recurring:            true,
		RecurringPeriod:      "monthly",
		PropertyCap:          500,
		UnlockedListingTypes: []string{"rental", "storage", "short_stay", "event_hourly", "coworking"},
		RequiredKYCDocuments: []models.KYCDocumentReq{
			{DocType: "title_deed", Description: "Proof of Property Ownership or Management Authority"},
			{DocType: "verified_payout", Description: "Bank Account / Mobile Money Payout Verification"},
		},
		UpdatedAt:            time.Now(),
	}

	// Level 3: Full Access incl. Sale
	t3 := &models.TierDefinition{
		ID:                   "tier_full",
		Level:                3,
		Name:                 "Full Access incl. Sale Listings",
		IsActive:             true,
		CostAmount:           15000.0,
		Currency:             "KES",
		Recurring:            true,
		RecurringPeriod:      "monthly",
		PropertyCap:          -1,
		UnlockedListingTypes: []string{"rental", "storage", "short_stay", "event_hourly", "coworking", "sale"},
		RequiredKYCDocuments: []models.KYCDocumentReq{
			{DocType: "earb_license", Description: "Estate Agents Registration Board (EARB) License"},
		},
		RequiresLicenseType:  "estate_agent",
		UpdatedAt:            time.Now(),
	}

	s.TierDefinitions[t1.ID] = t1
	s.writeDoc("tier_definitions", t1.ID, t1)

	s.TierDefinitions[t2.ID] = t2
	s.writeDoc("tier_definitions", t2.ID, t2)

	s.TierDefinitions[t3.ID] = t3
	s.writeDoc("tier_definitions", t3.ID, t3)
}

func (s *Store) CreateListing(l *models.Listing) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Listings[l.ID] = l
	s.writeDoc("listings", l.ID, l)
}

func (s *Store) GetListing(id string) (*models.Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.Listings[id]
	if !ok {
		return nil, errors.New("listing not found")
	}
	return l, nil
}

func (s *Store) GetAllListings() []*models.Listing {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Listing
	for _, l := range s.Listings {
		list = append(list, l)
	}
	return list
}

func (s *Store) CreateBooking(b *models.Booking) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Bookings[b.ID] = b
	s.writeDoc("bookings", b.ID, b)
}

func (s *Store) GetAllBookings() []*models.Booking {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Booking
	for _, b := range s.Bookings {
		list = append(list, b)
	}
	return list
}

func (s *Store) CreateStaffMembership(sm *models.StaffMembership) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StaffMemberships[sm.ID] = sm
	s.writeDoc("staff_memberships", sm.ID, sm)
}

func (s *Store) GetAllStaffMemberships() []*models.StaffMembership {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.StaffMembership
	for _, sm := range s.StaffMemberships {
		list = append(list, sm)
	}
	return list
}

func (s *Store) CreateTeamAction(ta *models.TeamAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TeamActions[ta.ID] = ta
	s.writeDoc("team_actions", ta.ID, ta)
}

func (s *Store) GetAllTeamActions() []*models.TeamAction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.TeamAction
	for _, ta := range s.TeamActions {
		list = append(list, ta)
	}
	return list
}

func (s *Store) CreateLead(l *models.Lead) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Leads[l.ID] = l
	s.writeDoc("leads", l.ID, l)
}

func (s *Store) GetAllLeads() []*models.Lead {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Lead
	for _, l := range s.Leads {
		list = append(list, l)
	}
	return list
}

func (s *Store) CreateCommission(c *models.Commission) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Commissions[c.ID] = c
	s.writeDoc("commissions", c.ID, c)
}

func (s *Store) GetAllCommissions() []*models.Commission {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Commission
	for _, c := range s.Commissions {
		list = append(list, c)
	}
	return list
}

func (s *Store) CreateStaffReview(sr *models.StaffReview) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StaffReviews[sr.ID] = sr
	s.writeDoc("staff_reviews", sr.ID, sr)
}

func (s *Store) GetAllStaffReviews() []*models.StaffReview {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.StaffReview
	for _, sr := range s.StaffReviews {
		list = append(list, sr)
	}
	return list
}

func (s *Store) CreateTierDefinition(td *models.TierDefinition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TierDefinitions[td.ID] = td
	s.writeDoc("tier_definitions", td.ID, td)
}

func (s *Store) GetAllTierDefinitions() []*models.TierDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.TierDefinition
	for _, td := range s.TierDefinitions {
		list = append(list, td)
	}
	return list
}

func (s *Store) CreateLandlordVerification(lv *models.LandlordVerification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LandlordVerifications[lv.ID] = lv
	s.writeDoc("landlord_verifications", lv.ID, lv)
}

func (s *Store) GetLandlordVerificationByUserID(userID string) (*models.LandlordVerification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, lv := range s.LandlordVerifications {
		if lv.UserID == userID {
			return lv, nil
		}
	}
	return nil, errors.New("landlord verification not found")
}

func (s *Store) GetAllLandlordVerifications() []*models.LandlordVerification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.LandlordVerification
	for _, lv := range s.LandlordVerifications {
		list = append(list, lv)
	}
	return list
}

func (s *Store) GetPropertyByID(id string) (*models.Property, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.Properties[id]
	if !ok {
		return nil, errors.New("property not found")
	}
	return p, nil
}

// Region CRUD
func (s *Store) CreateRegion(r *models.Region) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Regions[r.ID] = r
	s.writeDoc("regions", r.ID, r)
}

func (s *Store) UpdateRegion(r *models.Region) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Regions[r.ID] = r
	s.writeDoc("regions", r.ID, r)
}

func (s *Store) DeleteRegion(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Regions, id)
	if s.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.db.Collection("regions").DeleteOne(ctx, bson.M{"id": id})
	}
}

func (s *Store) GetAllRegions() []*models.Region {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Region
	for _, r := range s.Regions {
		list = append(list, r)
	}
	return list
}

// CommissionRule CRUD
func (s *Store) CreateCommissionRule(cr *models.CommissionRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CommissionRules[cr.ID] = cr
	s.writeDoc("commission_rules", cr.ID, cr)
}

func (s *Store) UpdateCommissionRule(cr *models.CommissionRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CommissionRules[cr.ID] = cr
	s.writeDoc("commission_rules", cr.ID, cr)
}

func (s *Store) DeleteCommissionRule(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.CommissionRules, id)
	if s.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.db.Collection("commission_rules").DeleteOne(ctx, bson.M{"id": id})
	}
}

func (s *Store) GetAllCommissionRules() []*models.CommissionRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.CommissionRule
	for _, cr := range s.CommissionRules {
		list = append(list, cr)
	}
	return list
}

// Jurisdiction CRUD
func (s *Store) CreateJurisdiction(j *models.Jurisdiction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Jurisdictions[j.ID] = j
	s.writeDoc("jurisdictions", j.ID, j)
}

func (s *Store) UpdateJurisdiction(j *models.Jurisdiction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Jurisdictions[j.ID] = j
	s.writeDoc("jurisdictions", j.ID, j)
}

func (s *Store) DeleteJurisdiction(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Jurisdictions, id)
	if s.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.db.Collection("jurisdictions").DeleteOne(ctx, bson.M{"id": id})
	}
}

func (s *Store) GetAllJurisdictions() []*models.Jurisdiction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Jurisdiction
	for _, j := range s.Jurisdictions {
		list = append(list, j)
	}
	return list
}

// Platform Commission settings CRUD
func (s *Store) GetPlatformCommissionSettings() *models.PlatformCommissionSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, pc := range s.PlatformCommissions {
		return pc
	}
	// Return default settings if none found
	return &models.PlatformCommissionSettings{
		ID:                  "default-comm",
		BaseFeePercentage:   5.0,
		ProductionMarkupPct: 10.0,
		VATEnabled:          true,
		WHTEnabled:          true,
	}
}

func (s *Store) SavePlatformCommissionSettings(pc *models.PlatformCommissionSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PlatformCommissions[pc.ID] = pc
	s.writeDoc("platform_commissions", pc.ID, pc)
}

// VacationNotice CRUD
func (s *Store) CreateVacationNotice(vn *models.VacationNotice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VacationNotices[vn.ID] = vn
	s.writeDoc("vacation_notices", vn.ID, vn)
}

func (s *Store) UpdateVacationNotice(vn *models.VacationNotice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VacationNotices[vn.ID] = vn
	s.writeDoc("vacation_notices", vn.ID, vn)
}

func (s *Store) GetAllVacationNotices() []*models.VacationNotice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.VacationNotice
	for _, vn := range s.VacationNotices {
		list = append(list, vn)
	}
	return list
}

// Application CRUD
func (s *Store) CreateApplication(app *models.Application) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Applications[app.ID] = app
	s.writeDoc("applications", app.ID, app)
}

func (s *Store) UpdateApplication(app *models.Application) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Applications[app.ID] = app
	s.writeDoc("applications", app.ID, app)
}

func (s *Store) GetApplicationByID(id string) (*models.Application, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.Applications[id]
	if !ok {
		return nil, errors.New("application not found")
	}
	return app, nil
}

func (s *Store) GetAllApplications() []*models.Application {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Application
	for _, app := range s.Applications {
		list = append(list, app)
	}
	return list
}

// Notification CRUD
func (s *Store) CreateNotification(n *models.Notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Notifications[n.ID] = n
	s.writeDoc("notifications", n.ID, n)
}

func (s *Store) GetNotificationsByUserID(userID string) []*models.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Notification
	for _, n := range s.Notifications {
		if n.UserID == userID {
			list = append(list, n)
		}
	}
	return list
}

// Inspection CRUD
func (s *Store) CreateInspection(insp *models.Inspection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Inspections[insp.ID] = insp
	s.writeDoc("inspections", insp.ID, insp)
}

func (s *Store) GetInspectionsByLeaseID(leaseID string) []*models.Inspection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Inspection
	for _, insp := range s.Inspections {
		if insp.LeaseID == leaseID {
			list = append(list, insp)
		}
	}
	return list
}

func (s *Store) GetDB() *mongo.Database {
	return s.db
}

// Viewing CRUD
func (s *Store) CreateViewing(v *models.Viewing) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Viewings[v.ID] = v
	s.writeDoc("viewings", v.ID, v)
}

func (s *Store) GetViewingsByStaffID(staffID string) []*models.Viewing {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Viewing
	for _, v := range s.Viewings {
		if v.StaffID == staffID {
			list = append(list, v)
		}
	}
	return list
}

