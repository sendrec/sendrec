package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/plans"
)

const maxWebhookBodyBytes = 64 * 1024

type Handlers struct {
	db              database.DBTX
	creem           *Client
	baseURL         string
	proProductID    string
	orgProProductID string
	webhookSecret   string
}

func NewHandlers(db database.DBTX, creem *Client, baseURL, proProductID, orgProProductID, webhookSecret string) *Handlers {
	return &Handlers{
		db:              db,
		creem:           creem,
		baseURL:         baseURL,
		proProductID:    proProductID,
		orgProProductID: orgProProductID,
		webhookSecret:   webhookSecret,
	}
}

type checkoutPlanRequest struct {
	Plan string `json:"plan"`
}

func (h *Handlers) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req checkoutPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Plan != "pro" {
		httputil.WriteError(w, http.StatusBadRequest, "unsupported plan")
		return
	}

	successURL := h.baseURL + "/settings?billing=success"
	checkoutURL, err := h.creem.CreateCheckout(r.Context(), h.proProductID, userID, successURL)
	if err != nil {
		slog.Error("failed to create checkout", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create checkout")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"checkoutUrl": checkoutURL})
}

type billingResponse struct {
	Plan               string  `json:"plan"`
	EffectivePlan      string  `json:"effectivePlan,omitempty"`
	SubscriptionID     *string `json:"subscriptionId"`
	SubscriptionStatus *string `json:"subscriptionStatus"`
	PortalURL          *string `json:"portalUrl"`
}

func (h *Handlers) GetBilling(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var plan string
	var subscriptionID *string
	var customerID *string

	err := h.db.QueryRow(r.Context(),
		"SELECT subscription_plan, creem_subscription_id, creem_customer_id FROM users WHERE id = $1",
		userID,
	).Scan(&plan, &subscriptionID, &customerID)
	if err != nil {
		slog.Error("failed to get billing info", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get billing info")
		return
	}

	resp := billingResponse{
		Plan:           plan,
		SubscriptionID: subscriptionID,
	}

	if subscriptionID != nil {
		info, err := h.creem.GetSubscription(r.Context(), *subscriptionID)
		if err != nil {
			slog.Error("failed to get subscription info", "error", err)
		} else {
			if info.Customer.PortalURL != "" {
				resp.PortalURL = &info.Customer.PortalURL
			}
			if info.Status != "" {
				resp.SubscriptionStatus = &info.Status
			}
		}
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handlers) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var subscriptionID *string
	err := h.db.QueryRow(r.Context(),
		"SELECT creem_subscription_id FROM users WHERE id = $1",
		userID,
	).Scan(&subscriptionID)
	if err != nil {
		slog.Error("failed to get subscription for cancel", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get subscription")
		return
	}

	if subscriptionID == nil {
		httputil.WriteError(w, http.StatusBadRequest, "no active subscription")
		return
	}

	if err := h.creem.CancelSubscription(r.Context(), *subscriptionID); err != nil {
		slog.Error("failed to cancel subscription", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to cancel subscription")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) CreateOrgCheckout(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var role string
	err := h.db.QueryRow(r.Context(),
		"SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2",
		orgID, userID,
	).Scan(&role)
	if err != nil || role != "owner" {
		httputil.WriteError(w, http.StatusForbidden, "only org owners can manage billing")
		return
	}

	var req checkoutPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Plan != "pro" {
		httputil.WriteError(w, http.StatusBadRequest, "unsupported plan")
		return
	}

	if h.orgProProductID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "org billing not configured")
		return
	}

	successURL := h.baseURL + "/organizations/" + orgID + "/settings?billing=success"
	metadata := map[string]string{"userId": userID, "orgId": orgID}
	checkoutURL, err := h.creem.CreateCheckoutWithMetadata(r.Context(), h.orgProProductID, successURL, metadata)
	if err != nil {
		slog.Error("failed to create org checkout", "error", err, "org_id", orgID)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create checkout")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"checkoutUrl": checkoutURL})
}

func (h *Handlers) GetOrgBilling(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var memberRole string
	err := h.db.QueryRow(r.Context(),
		"SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2",
		orgID, userID,
	).Scan(&memberRole)
	if err != nil {
		httputil.WriteError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	var plan string
	var subscriptionID *string
	var customerID *string

	err = h.db.QueryRow(r.Context(),
		"SELECT subscription_plan, creem_subscription_id, creem_customer_id FROM organizations WHERE id = $1",
		orgID,
	).Scan(&plan, &subscriptionID, &customerID)
	if err != nil {
		slog.Error("failed to get org billing info", "error", err, "org_id", orgID)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get billing info")
		return
	}

	var ownerPlan string
	_ = h.db.QueryRow(r.Context(),
		`SELECT COALESCE(u.subscription_plan, 'free')
		 FROM organization_members om
		 JOIN users u ON u.id = om.user_id
		 WHERE om.organization_id = $1 AND om.role = 'owner'
		 ORDER BY CASE u.subscription_plan WHEN 'business' THEN 2 WHEN 'pro' THEN 1 ELSE 0 END DESC
		 LIMIT 1`,
		orgID,
	).Scan(&ownerPlan)

	effectivePlan := plan
	if plans.Rank(ownerPlan) > plans.Rank(plan) {
		effectivePlan = ownerPlan
	}

	resp := billingResponse{
		Plan:           plan,
		EffectivePlan:  effectivePlan,
		SubscriptionID: subscriptionID,
	}

	if subscriptionID != nil {
		info, err := h.creem.GetSubscription(r.Context(), *subscriptionID)
		if err != nil {
			slog.Error("failed to get org subscription info", "error", err, "org_id", orgID)
		} else {
			if info.Customer.PortalURL != "" {
				resp.PortalURL = &info.Customer.PortalURL
			}
			if info.Status != "" {
				resp.SubscriptionStatus = &info.Status
			}
		}
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handlers) CancelOrgSubscription(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var role string
	err := h.db.QueryRow(r.Context(),
		"SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2",
		orgID, userID,
	).Scan(&role)
	if err != nil || role != "owner" {
		httputil.WriteError(w, http.StatusForbidden, "only org owners can manage billing")
		return
	}

	var subscriptionID *string
	err = h.db.QueryRow(r.Context(),
		"SELECT creem_subscription_id FROM organizations WHERE id = $1",
		orgID,
	).Scan(&subscriptionID)
	if err != nil {
		slog.Error("failed to get org subscription for cancel", "error", err, "org_id", orgID)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get subscription")
		return
	}

	if subscriptionID == nil {
		httputil.WriteError(w, http.StatusBadRequest, "no active subscription")
		return
	}

	if err := h.creem.CancelSubscription(r.Context(), *subscriptionID); err != nil {
		slog.Error("failed to cancel org subscription", "error", err, "org_id", orgID)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to cancel subscription")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type webhookPayload struct {
	ID        string        `json:"id"`
	EventType string        `json:"eventType"`
	CreatedAt int64         `json:"created_at"`
	Object    webhookObject `json:"object"`
}

type webhookObject struct {
	ID       string           `json:"id"`
	Product  webhookProduct   `json:"product"`
	Customer webhookCustomer  `json:"customer"`
	Metadata webhookMetadata  `json:"metadata"`
}

type webhookProduct struct {
	ID string `json:"id"`
}

type webhookCustomer struct {
	ID string `json:"id"`
}

type webhookMetadata struct {
	UserID string `json:"userId"`
	OrgID  string `json:"orgId"`
}

func (h *Handlers) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodyBytes))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	signature := r.Header.Get("creem-signature")
	if !h.verifySignature(body, signature) {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if payload.ID == "" {
		slog.Warn("webhook: missing event ID", "event_type", payload.EventType)
		w.WriteHeader(http.StatusOK)
		return
	}

	duplicate, err := h.recordEvent(r.Context(), payload, body)
	if err != nil {
		slog.Error("webhook: failed to record event", "error", err, "event_id", payload.ID)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to process webhook")
		return
	}
	if duplicate {
		slog.Info("webhook: duplicate event ignored", "event_id", payload.ID, "event_type", payload.EventType)
		w.WriteHeader(http.StatusOK)
		return
	}

	userID := payload.Object.Metadata.UserID
	orgID := payload.Object.Metadata.OrgID

	if userID == "" && orgID == "" {
		slog.Warn("webhook: missing userId and orgId in metadata", "event_type", payload.EventType, "event_id", payload.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	switch payload.EventType {
	case "subscription.active", "subscription.paid":
		h.handleSubscriptionActivated(r, w, payload, userID, orgID)
	case "subscription.canceled", "subscription.scheduled_cancel":
		slog.Info("webhook: subscription cancel requested, keeping plan until expiry", "user_id", userID, "org_id", orgID, "event_id", payload.ID)
		w.WriteHeader(http.StatusOK)
	case "subscription.expired":
		h.handleSubscriptionCanceled(r, w, userID, orgID)
	case "subscription.past_due":
		slog.Warn("webhook: subscription past due, payment retrying", "user_id", userID, "org_id", orgID, "event_id", payload.ID)
		w.WriteHeader(http.StatusOK)
	case "refund.created":
		h.handleSubscriptionCanceled(r, w, userID, orgID)
	case "dispute.created":
		slog.Error("webhook: dispute created, manual review needed", "user_id", userID, "org_id", orgID, "event_id", payload.ID)
		w.WriteHeader(http.StatusOK)
	default:
		slog.Warn("webhook: unhandled event", "event_type", payload.EventType, "event_id", payload.ID)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handlers) handleSubscriptionActivated(r *http.Request, w http.ResponseWriter, payload webhookPayload, userID, orgID string) {
	plan := h.planFromProductID(payload.Object.Product.ID)
	if plan == "" {
		slog.Warn("webhook: unknown product ID", "product_id", payload.Object.Product.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	if orgID != "" {
		_, err := h.db.Exec(r.Context(),
			"UPDATE organizations SET subscription_plan = $1, creem_subscription_id = $2, creem_customer_id = $3, updated_at = now() WHERE id = $4",
			plan, payload.Object.ID, payload.Object.Customer.ID, orgID,
		)
		if err != nil {
			slog.Error("failed to update org subscription", "error", err, "org_id", orgID)
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update subscription")
			return
		}
	} else {
		_, err := h.db.Exec(r.Context(),
			"UPDATE users SET subscription_plan = $1, creem_subscription_id = $2, creem_customer_id = $3 WHERE id = $4",
			plan, payload.Object.ID, payload.Object.Customer.ID, userID,
		)
		if err != nil {
			slog.Error("failed to update subscription", "error", err)
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update subscription")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleSubscriptionCanceled downgrades to free but preserves creem_subscription_id
// and creem_customer_id for audit trail and potential re-subscription.
func (h *Handlers) handleSubscriptionCanceled(r *http.Request, w http.ResponseWriter, userID, orgID string) {
	if orgID != "" {
		_, err := h.db.Exec(r.Context(),
			"UPDATE organizations SET subscription_plan = $1, updated_at = now() WHERE id = $2",
			"free", orgID,
		)
		if err != nil {
			slog.Error("failed to cancel org subscription", "error", err, "org_id", orgID)
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update subscription")
			return
		}
	} else {
		_, err := h.db.Exec(r.Context(),
			"UPDATE users SET subscription_plan = $1 WHERE id = $2",
			"free", userID,
		)
		if err != nil {
			slog.Error("failed to cancel subscription", "error", err)
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update subscription")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) verifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *Handlers) planFromProductID(productID string) string {
	if productID == h.proProductID || (h.orgProProductID != "" && productID == h.orgProProductID) {
		return "pro"
	}
	return ""
}

func (h *Handlers) recordEvent(ctx context.Context, payload webhookPayload, body []byte) (duplicate bool, err error) {
	var userID *string
	if uid := payload.Object.Metadata.UserID; uid != "" {
		userID = &uid
	}

	createdAt := time.UnixMilli(payload.CreatedAt)

	tag, err := h.db.Exec(ctx,
		`INSERT INTO creem_webhook_events (event_id, event_type, user_id, payload, created_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (event_id) DO NOTHING`,
		payload.ID, payload.EventType, userID, body, createdAt,
	)
	if err != nil {
		return false, err
	}

	return tag.RowsAffected() == 0, nil
}
