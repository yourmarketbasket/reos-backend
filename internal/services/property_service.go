package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type PropertyService struct {
	Store *store.Store
}

func NewPropertyService(s *store.Store) *PropertyService {
	return &PropertyService{Store: s}
}

func (s *PropertyService) CreateProperty(ownerID, name string, lat, lon float64, city, country, address, jurisdiction string) (*models.Property, error) {
	prop := &models.Property{
		ID:             uuid.New().String(),
		OwnerID:        ownerID,
		CreatedBy:      ownerID,
		Name:           name,
		Slug:           uuid.New().String()[:8],
		ApprovalStatus: models.ApprovalPending,
		PublishStatus:  models.PublishStatusDraft,
		Location: models.Location{
			Type:        "Point",
			Coordinates: []float64{lon, lat},
		},
		City:         city,
		Country:      country,
		Address:      address,
		Jurisdiction: jurisdiction,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	s.Store.CreateProperty(prop)
	return prop, nil
}

func (s *PropertyService) PublishProperty(propertyID string) error {
	s.Store.Lock()
	defer s.Store.Unlock()

	prop, ok := s.Store.Properties[propertyID]
	if !ok {
		return errors.New("property not found")
	}

	prop.PublishStatus = models.PublishStatusPublished
	prop.UpdatedAt = time.Now()

	s.Store.CreateProperty(prop)
	return nil
}

func (s *PropertyService) UnpublishProperty(propertyID string) error {
	s.Store.Lock()
	defer s.Store.Unlock()

	prop, ok := s.Store.Properties[propertyID]
	if !ok {
		return errors.New("property not found")
	}

	prop.PublishStatus = models.PublishStatusDraft
	prop.UpdatedAt = time.Now()

	s.Store.CreateProperty(prop)
	return nil
}
