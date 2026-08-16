package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/reos/api/internal/store"
)

// RBAC Middleware: Require user to have one of the allowed roles
func RequireRole(s *store.Store, allowedRoles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, err := getUserIdFromAuthHeader(r, s)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := s.GetUserByID(userID)
			if err != nil {
				http.Error(w, "User identity not found", http.StatusUnauthorized)
				return
			}

			// Validate role matches
			roleAllowed := false
			for _, r := range allowedRoles {
				if user.Role == r {
					roleAllowed = true
					break
				}
			}

			if !roleAllowed {
				http.Error(w, fmt.Sprintf("Forbidden: this endpoint requires role permissions of %s", strings.Join(allowedRoles, ", ")), http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}

// SBAC Middleware: Require landlord user to have a minimum subscription plan tier
func RequireSubscriptionTier(s *store.Store, minTier string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, err := getUserIdFromAuthHeader(r, s)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := s.GetUserByID(userID)
			if err != nil {
				http.Error(w, "User identity not found", http.StatusUnauthorized)
				return
			}

			// Subscriptions only apply to Landlords in REOS
			if user.Role == "landlord" {
				tierWeights := map[string]int{
					"":         1,
					"free":     1,
					"standard": 2,
					"premium":  3,
				}

				currentWeight := tierWeights[user.SubscriptionTier]
				requiredWeight := tierWeights[minTier]

				if currentWeight < requiredWeight {
					http.Error(w, fmt.Sprintf("Forbidden: access to this premium feature requires subscription tier '%s' or higher", minTier), http.StatusPaymentRequired)
					return
				}

				// Optional: check active subscription status
				if user.SubscriptionStatus != "active" && user.SubscriptionStatus != "trialing" && user.SubscriptionTier != "free" {
					http.Error(w, "Payment Required: your subscription status is past due or inactive", http.StatusPaymentRequired)
					return
				}
			}

			next(w, r)
		}
	}
}
