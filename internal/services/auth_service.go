package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

type AuthService struct {
	Store *store.Store
}

func NewAuthService(s *store.Store) *AuthService {
	return &AuthService{Store: s}
}

func (s *AuthService) RegisterUser(email, phone, password, role string) (*models.User, error) {
	s.Store.RLock()
	for _, u := range s.Store.Users {
		if u.Email == email || u.Phone == phone {
			s.Store.RUnlock()
			return nil, errors.New("email or phone already registered")
		}
	}
	s.Store.RUnlock()

	user := &models.User{
		ID:                   uuid.New().String(),
		Role:                 role,
		Email:                email,
		Phone:                phone,
		PasswordHash:         store.HashPassword(password),
		Status:               "active",
		IdentityVerification: "pending",
		CreatedAt:            time.Now(),
	}

	if err := s.Store.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) LoginUser(email, password string) (*models.User, string, error) {
	user, err := s.Store.GetUserByEmail(email)
	if err != nil {
		s.Store.RLock()
		for _, u := range s.Store.Users {
			if u.Phone == email {
				user = u
				err = nil
				break
			}
		}
		s.Store.RUnlock()
		if err != nil {
			return nil, "", errors.New("invalid credentials")
		}
	}

	if store.HashPassword(password) != user.PasswordHash {
		return nil, "", errors.New("invalid credentials")
	}

	if user.Status == "suspended" {
		return nil, "", errors.New("this account is suspended")
	}

	token := "session_" + user.ID
	s.Store.Lock()
	user.Sessions = append(user.Sessions, token)
	s.Store.Unlock()
	s.Store.CreateUser(user)

	return user, token, nil
}
