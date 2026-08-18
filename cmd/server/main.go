package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/reos/api/internal/handlers"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
	"go.mongodb.org/mongo-driver/bson"
)

func claimNisokoHandles(s *store.Store) {
	if s.GetDB() == nil {
		log.Printf("MongoDB client is not active in store. Skipping Nisoko registration check.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	col := s.GetDB().Collection("system_settings")
	var doc bson.M
	err := col.FindOne(ctx, bson.M{"key": "nisoko_setup_done"}).Decode(&doc)
	if err == nil {
		log.Printf("Nisoko handles and webhook already claimed/registered in past sessions. Skipping.")
		return
	}

	handles := []string{"reos.security", "reos.support", "reos.billing"}
	apiKey := "nsk_live_6a7fbfd80c696c3460f34cbb"
	for _, h := range handles {
		payload, _ := json.Marshal(map[string]string{"handle": h})
		req, err := http.NewRequest("POST", "https://nes.nisoko.co.ke/api/v1/nes/handles", bytes.NewBuffer(payload))
		if err != nil {
			log.Printf("Error creating request for handle %s: %v", h, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", apiKey)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error claiming handle %s: %v", h, err)
			continue
		}
		resp.Body.Close()
		log.Printf("Claimed handle %s: status %s", h, resp.Status)
	}

	// Register webhook
	webhookPayload, _ := json.Marshal(map[string]interface{}{
		"name":   "REOS App Webhook",
		"url":    "https://reos.deployments.nisoko.co.ke/api/webhooks/nes",
		"events": []string{"email.sent", "email.received", "email.bounced", "email.opened", "email.clicked", "email.failed"},
	})
	wReq, wErr := http.NewRequest("POST", "https://nes.nisoko.co.ke/api/v1/nes/webhooks", bytes.NewBuffer(webhookPayload))
	if wErr == nil {
		wReq.Header.Set("Content-Type", "application/json")
		wReq.Header.Set("X-API-Key", apiKey)
		wClient := &http.Client{Timeout: 10 * time.Second}
		wResp, wErr2 := wClient.Do(wReq)
		if wErr2 == nil {
			wResp.Body.Close()
			log.Printf("NES Webhook registered: status %s", wResp.Status)
		} else {
			log.Printf("Error registering NES Webhook: %v", wErr2)
		}
	} else {
		log.Printf("Error creating NES Webhook request: %v", wErr)
	}

	// Persist registration status
	_, _ = col.InsertOne(ctx, bson.M{"key": "nisoko_setup_done", "done": true, "updated_at": time.Now()})
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Idempotency-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Compress response using Gzip if the client supports it and this is not a WebSocket upgrade request
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && r.Header.Get("Upgrade") == "" {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
			next(gzw, r)
			return
		}

		next(w, r)
	}
}

func startListingReviewWorker(s *store.Store) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			now := time.Now()
			s.Lock()
			for _, l := range s.Listings {
				if l.ApprovalStatus == models.ApprovalPending {
					age := now.Sub(l.SubmitForReviewAt)
					if age >= 24*time.Hour {
						l.ApprovalStatus = models.ApprovalApproved
						l.ApprovalNote = "Automatically approved after 24 hours of no manual review."
						l.ApprovedBy = "system_cron"
						l.ApprovedAt = &now
						l.Status = "published"
						l.UpdatedAt = now
						s.Unlock()
						s.CreateListing(l)
						handlers.BroadcastNotification(l.CreatedBy, "Listing Auto-Approved", "Your listing '"+l.Title+"' has been automatically approved and published.")
						s.Lock()
					} else if age >= 4*time.Hour {
						for _, u := range s.Users {
							if u.Role == models.RoleSupportAdmin || u.Role == models.RoleSuperAdmin {
								handlers.BroadcastNotification(u.ID, "Pending Verification Warning", "Listing '"+l.Title+"' has been pending verification for over 4 hours.")
							}
						}
					}
				}
			}
			s.Unlock()
		}
	}()
}

func main() {
	dbStore := store.NewStore()
	store.InitRedis()
	startListingReviewWorker(dbStore)

	go claimNisokoHandles(dbStore)

	authHandler := &handlers.AuthHandler{Store: dbStore}
	invHandler := &handlers.InvitationsHandler{Store: dbStore}
	propHandler := &handlers.PropertiesHandler{Store: dbStore}
	dashHandler := &handlers.DashboardsHandler{Store: dbStore}
	opsHandler := &handlers.OperationsHandler{Store: dbStore}
	regHandler := handlers.NewRegionsHandler(dbStore)
	commHandler := handlers.NewCommissionRulesHandler(dbStore)
	uploadHandler := handlers.NewUploadsHandler(dbStore)
	supportHandler := handlers.NewSupportHandler(dbStore)
	jurisHandler := handlers.NewJurisdictionsHandler(dbStore)

	// RBAC middle-tier filters
	adminOnly := handlers.RequireRole(dbStore, models.RoleSuperAdmin)
	caretakerOnly := handlers.RequireRole(dbStore, models.RoleCaretaker)
	landlordOrAgentOnly := handlers.RequireRole(dbStore, models.RoleLandlord, models.RoleAgent)
	landlordAgentOrStaffOnly := handlers.RequireRole(dbStore, models.RoleLandlord, models.RoleAgent, models.RoleStaff, models.RoleCaretaker)
	anyAdmin := handlers.RequireRole(dbStore, models.RoleSuperAdmin, models.RoleSupportAdmin, models.RoleBillingAdmin, models.RoleTechAdmin)
	
	// canInvite: superadmin + all platform admins + landlord + agent
	canInvite := handlers.RequireRole(dbStore,
		models.RoleSuperAdmin, models.RoleTechAdmin, models.RoleSupportAdmin, models.RoleBillingAdmin,
		models.RoleLandlord, models.RoleAgent,
	)

	// WebSockets & Real-time status
	http.HandleFunc("/api/ws", corsMiddleware(handlers.HandleWS(dbStore)))

	// Jurisdictions Endpoints
	http.HandleFunc("/api/jurisdictions", corsMiddleware(jurisHandler.ListJurisdictions))
	http.HandleFunc("/api/jurisdictions/create", corsMiddleware(adminOnly(jurisHandler.CreateJurisdiction)))
	http.HandleFunc("/api/jurisdictions/update", corsMiddleware(adminOnly(jurisHandler.UpdateJurisdiction)))
	http.HandleFunc("/api/jurisdictions/toggle", corsMiddleware(adminOnly(jurisHandler.ToggleJurisdiction)))
	http.HandleFunc("/api/jurisdictions/delete", corsMiddleware(adminOnly(jurisHandler.DeleteJurisdiction)))
	http.HandleFunc("/api/admin/commission-settings", corsMiddleware(adminOnly(jurisHandler.GetCommissionSettings)))
	http.HandleFunc("/api/admin/commission-settings/save", corsMiddleware(adminOnly(jurisHandler.SaveCommissionSettings)))

	// Support Actions & KYC
	http.HandleFunc("/api/support/listings/unpublish", corsMiddleware(anyAdmin(supportHandler.UnpublishListing)))
	http.HandleFunc("/api/support/users/suspend", corsMiddleware(anyAdmin(supportHandler.SuspendUser)))
	http.HandleFunc("/api/support/kyc/verify", corsMiddleware(anyAdmin(supportHandler.VerifyKYC)))
	http.HandleFunc("/api/support/kyc/list", corsMiddleware(anyAdmin(supportHandler.ListKYCQueues)))

	// Leases, Vacations & Transfers
	http.HandleFunc("/api/leases/vacate", corsMiddleware(supportHandler.VacateLease))
	http.HandleFunc("/api/leases/transfer", corsMiddleware(landlordOrAgentOnly(supportHandler.TransferTenant)))
	http.HandleFunc("/api/inspections/create", corsMiddleware(supportHandler.CreateInspection))
	http.HandleFunc("/api/inspections/list", corsMiddleware(supportHandler.ListInspections))
	http.HandleFunc("/api/deductions/create", corsMiddleware(supportHandler.CreateDeductionDraft))
	http.HandleFunc("/api/deductions/list", corsMiddleware(supportHandler.ListDeductions))

	http.HandleFunc("/api/applications/create", corsMiddleware(supportHandler.CreateApplication))
	http.HandleFunc("/api/applications/list", corsMiddleware(supportHandler.ListApplications))
	http.HandleFunc("/api/applications/update", corsMiddleware(supportHandler.UpdateApplication))
	http.HandleFunc("/api/viewings/create", corsMiddleware(supportHandler.CreateViewing))
	http.HandleFunc("/api/viewings/list", corsMiddleware(supportHandler.ListViewings))

	// Auth Endpoints
	http.HandleFunc("/api/auth/register", corsMiddleware(authHandler.Register))
	http.HandleFunc("/api/auth/login", corsMiddleware(authHandler.Login))
	http.HandleFunc("/api/auth/verify-otp", corsMiddleware(authHandler.VerifyOTP))
	http.HandleFunc("/api/auth/google", corsMiddleware(authHandler.GoogleAuth))
	http.HandleFunc("/api/auth/me", corsMiddleware(authHandler.Me))
	http.HandleFunc("/api/auth/profile/update", corsMiddleware(authHandler.UpdateProfile))
	http.HandleFunc("/api/auth/recover", corsMiddleware(authHandler.RecoverPassword))
	http.HandleFunc("/api/auth/webauthn/register/options", corsMiddleware(authHandler.WebAuthnRegisterOptions))
	http.HandleFunc("/api/auth/webauthn/register/verify", corsMiddleware(authHandler.WebAuthnRegisterVerify))
	http.HandleFunc("/api/auth/webauthn/login/options", corsMiddleware(authHandler.WebAuthnLoginOptions))
	http.HandleFunc("/api/auth/webauthn/login/verify", corsMiddleware(authHandler.WebAuthnLoginVerify))
	http.HandleFunc("/api/webhooks/nes", corsMiddleware(authHandler.NESWebhook))

	// Invitation Endpoints
	http.HandleFunc("/api/invitations/create", corsMiddleware(canInvite(invHandler.CreateInvitation)))
	http.HandleFunc("/api/invitations/list", corsMiddleware(canInvite(invHandler.ListInvitations)))
	http.HandleFunc("/api/invitations/revoke", corsMiddleware(canInvite(invHandler.RevokeInvitation)))
	http.HandleFunc("/api/invitations/resend", corsMiddleware(canInvite(invHandler.ResendInvitation)))
	http.HandleFunc("/api/invitations/detail", corsMiddleware(invHandler.GetInvitationDetails))
	http.HandleFunc("/api/invitations/accept", corsMiddleware(invHandler.AcceptInvitation))
	http.HandleFunc("/api/invitations/debug", corsMiddleware(invHandler.DebugListInvitations))

	// Properties, Units & Leases Endpoints
	http.HandleFunc("/api/properties/create", corsMiddleware(landlordAgentOrStaffOnly(propHandler.CreateProperty)))
	http.HandleFunc("/api/properties/list", corsMiddleware(propHandler.ListProperties))
	http.HandleFunc("/api/properties/update", corsMiddleware(landlordAgentOrStaffOnly(propHandler.UpdateProperty)))
	http.HandleFunc("/api/properties/approve", corsMiddleware(propHandler.ApproveProperty))
	http.HandleFunc("/api/properties/reject", corsMiddleware(propHandler.RejectProperty))
	http.HandleFunc("/api/properties/publish", corsMiddleware(landlordAgentOrStaffOnly(propHandler.PublishProperty)))
	http.HandleFunc("/api/properties/unpublish", corsMiddleware(landlordAgentOrStaffOnly(propHandler.UnpublishProperty)))
	http.HandleFunc("/api/properties/review", corsMiddleware(propHandler.SubmitReview))
	http.HandleFunc("/api/properties/review/respond", corsMiddleware(propHandler.RespondToReview))
	http.HandleFunc("/api/units/create", corsMiddleware(landlordAgentOrStaffOnly(propHandler.CreateUnit)))
	http.HandleFunc("/api/units/list", corsMiddleware(propHandler.ListUnits))
	http.HandleFunc("/api/leases/list", corsMiddleware(propHandler.ListLeases))
	http.HandleFunc("/api/payments/pay-rent", corsMiddleware(propHandler.PayRent))
	http.HandleFunc("/api/payments/ledger", corsMiddleware(propHandler.ListLedger))

	// Regions Endpoints
	http.HandleFunc("/api/regions", corsMiddleware(regHandler.ListRegions))
	http.HandleFunc("/api/regions/create", corsMiddleware(adminOnly(regHandler.CreateRegion)))
	http.HandleFunc("/api/regions/update", corsMiddleware(adminOnly(regHandler.UpdateRegion)))
	http.HandleFunc("/api/regions/toggle", corsMiddleware(adminOnly(regHandler.ToggleRegion)))
	http.HandleFunc("/api/regions/delete", corsMiddleware(adminOnly(regHandler.DeleteRegion)))

	// Commission Rules Endpoints
	http.HandleFunc("/api/commission-rules", corsMiddleware(commHandler.ListCommissionRules))
	http.HandleFunc("/api/commission-rules/create", corsMiddleware(commHandler.CreateCommissionRule))
	http.HandleFunc("/api/commission-rules/update", corsMiddleware(commHandler.UpdateCommissionRule))
	http.HandleFunc("/api/commission-rules/delete", corsMiddleware(commHandler.DeleteCommissionRule))

	// Uploads Endpoints
	http.HandleFunc("/api/uploads/image", corsMiddleware(uploadHandler.UploadImage))

	// File Server for Uploads
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// Dashboards & Actions
	http.HandleFunc("/api/dashboard/stats", corsMiddleware(dashHandler.GetDashboardStats))
	http.HandleFunc("/api/maintenance/report", corsMiddleware(dashHandler.ReportMaintenance))
	http.HandleFunc("/api/maintenance/list", corsMiddleware(dashHandler.ListMaintenance))
	http.HandleFunc("/api/maintenance/update", corsMiddleware(caretakerOnly(dashHandler.UpdateMaintenance)))
	http.HandleFunc("/api/disputes/create", corsMiddleware(dashHandler.CreateDispute))
	http.HandleFunc("/api/disputes/list", corsMiddleware(dashHandler.ListDisputes))
	http.HandleFunc("/api/disputes/message", corsMiddleware(dashHandler.AddDisputeMessage))
	http.HandleFunc("/api/disputes/resolve", corsMiddleware(anyAdmin(dashHandler.ResolveDispute)))
	http.HandleFunc("/api/disputes/escalate", corsMiddleware(dashHandler.EscalateDispute))

	// Operations Endpoints
	http.HandleFunc("/api/listings/create", corsMiddleware(opsHandler.CreateListing))
	http.HandleFunc("/api/listings/list", corsMiddleware(opsHandler.ListListings))
	http.HandleFunc("/api/listings/update", corsMiddleware(opsHandler.UpdateListing))
	http.HandleFunc("/api/listings/approve", corsMiddleware(anyAdmin(opsHandler.ApproveListing)))
	http.HandleFunc("/api/listings/reject", corsMiddleware(anyAdmin(opsHandler.RejectListing)))
	http.HandleFunc("/api/bookings/create", corsMiddleware(opsHandler.CreateBooking))
	http.HandleFunc("/api/bookings/list", corsMiddleware(opsHandler.ListBookings))
	http.HandleFunc("/api/team/invite", corsMiddleware(opsHandler.InviteStaff))
	http.HandleFunc("/api/team/list", corsMiddleware(opsHandler.ListStaff))
	http.HandleFunc("/api/tiers/list", corsMiddleware(opsHandler.ListTiers))
	http.HandleFunc("/api/tiers/upgrade", corsMiddleware(opsHandler.UpgradeTier))
	http.HandleFunc("/api/leads/list", corsMiddleware(opsHandler.ListLeads))
	http.HandleFunc("/api/commissions/list", corsMiddleware(opsHandler.ListCommissions))
	http.HandleFunc("/api/debug/db", corsMiddleware(opsHandler.DebugDB))

	// Admin Actions
	http.HandleFunc("/api/admin/gateway", corsMiddleware(adminOnly(dashHandler.UpdateGatewayConfig)))
	http.HandleFunc("/api/admin/users", corsMiddleware(adminOnly(dashHandler.ListSystemUsers)))
	http.HandleFunc("/api/admin/sms-logs", corsMiddleware(adminOnly(dashHandler.GetSMSLogs)))

	port := ":8080"
	fmt.Printf("REOS Monolith API starting on %s...\n", port)

	// Wrap serve multiplexer globally with rate limiting, security headers, and size limiters
	globalHandler := handlers.SecurityHeaders(
		handlers.DynamicRequestSizeLimit(
			handlers.DynamicRateLimit(http.DefaultServeMux),
		),
	)

	if err := http.ListenAndServe(port, globalHandler); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}
