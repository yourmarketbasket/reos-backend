package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

func GetUserIdFromAuthHeader(r *http.Request, s *store.Store) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("missing or invalid auth header")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !strings.HasPrefix(token, "session_") {
		return "", fmt.Errorf("invalid token format")
	}

	s.RLock()
	defer s.RUnlock()

	var matchedUser *models.User
	for _, u := range s.Users {
		for _, sess := range u.Sessions {
			if sess == token {
				matchedUser = u
				break
			}
		}
		if matchedUser != nil {
			break
		}
	}

	// Fallback for old format
	if matchedUser == nil {
		userID := strings.TrimPrefix(token, "session_")
		if u, ok := s.Users[userID]; ok && len(u.Sessions) == 0 {
			matchedUser = u
		}
	}

	if matchedUser == nil {
		return "", fmt.Errorf("invalid or expired session token")
	}

	if matchedUser.Status == "suspended" {
		return "", fmt.Errorf("this account has been suspended by system support")
	}

	return matchedUser.ID, nil
}
