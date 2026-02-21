package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
)

const maxWebhookBodyBytes = 64 * 1024

type Handlers struct {
	db            database.DBTX
	creem         *Client
	baseURL       string
	proProductID  string
	webhookSecret string
}

func NewHandlers(db database.DBTX, creem *Client, baseURL, proProductID, webhookSecret string) *Handlers {
	return &Handlers{
		db:            db,
		creem:         creem,
		baseURL:       baseURL,
		proProductID:  proProductID,
		webhookSecret: webhookSecret,
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
		log.Printf("create checkout: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create checkout")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"checkoutUrl": checkoutURL})
}

type billingResponse struct {
	Plan           string  `json:"plan"`
	SubscriptionID *string `json:"subscriptionId"`
	PortalURL      *string `json:"portalUrl"`
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
		log.Printf("get billing info: %v", err)
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
			log.Printf("get subscription info: %v", err)
		} else if info.Customer.PortalURL != "" {
			resp.PortalURL = &info.Customer.PortalURL
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
		log.Printf("get subscription for cancel: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get subscription")
		return
	}

	if subscriptionID == nil {
		httputil.WriteError(w, http.StatusBadRequest, "no active subscription")
		return
	}

	if err := h.creem.CancelSubscription(r.Context(), *subscriptionID); err != nil {
		log.Printf("cancel subscription: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to cancel subscription")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type webhookPayload struct {
	Event  string        `json:"event"`
	Object webhookObject `json:"object"`
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

	userID := payload.Object.Metadata.UserID
	if userID == "" {
		log.Printf("webhook %s: missing userId in metadata", payload.Event)
		w.WriteHeader(http.StatusOK)
		return
	}

	switch payload.Event {
	case "subscription.active", "subscription.paid":
		h.handleSubscriptionActivated(r, w, payload, userID)
	case "subscription.canceled", "subscription.expired":
		h.handleSubscriptionCanceled(r, w, userID)
	default:
		log.Printf("webhook: unhandled event %s", payload.Event)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handlers) handleSubscriptionActivated(r *http.Request, w http.ResponseWriter, payload webhookPayload, userID string) {
	plan := h.planFromProductID(payload.Object.Product.ID)
	if plan == "" {
		log.Printf("webhook: unknown product ID %s", payload.Object.Product.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	_, err := h.db.Exec(r.Context(),
		"UPDATE users SET subscription_plan = $1, creem_subscription_id = $2, creem_customer_id = $3 WHERE id = $4",
		plan, payload.Object.ID, payload.Object.Customer.ID, userID,
	)
	if err != nil {
		log.Printf("update subscription: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) handleSubscriptionCanceled(r *http.Request, w http.ResponseWriter, userID string) {
	_, err := h.db.Exec(r.Context(),
		"UPDATE users SET subscription_plan = $1 WHERE id = $2",
		"free", userID,
	)
	if err != nil {
		log.Printf("cancel subscription: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update subscription")
		return
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
	if productID == h.proProductID {
		return "pro"
	}
	return ""
}
