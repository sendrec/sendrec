package organization

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

const inviteTokenExpiry = 7 * 24 * time.Hour

type sendInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type inviteResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}

type inviteListItem struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	InvitedByName string `json:"invitedByName"`
	ExpiresAt     string `json:"expiresAt"`
	CreatedAt     string `json:"createdAt"`
}

type acceptInviteRequest struct {
	Token string `json:"token"`
}

type acceptInviteResponse struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Role           string `json:"role"`
}

func generateInviteToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

func hashInviteToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (h *Handler) SendInvite(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var callerRole string
	err := h.db.QueryRow(r.Context(),
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&callerRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "organization not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify membership")
		return
	}
	if callerRole != "owner" && callerRole != "admin" {
		httputil.WriteError(w, http.StatusForbidden, "only owners and admins can send invites")
		return
	}

	var req sendInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email is required")
		return
	}
	if req.Role != "admin" && req.Role != "member" {
		httputil.WriteError(w, http.StatusBadRequest, "role must be admin or member")
		return
	}

	var exists int
	err = h.db.QueryRow(r.Context(),
		`SELECT 1 FROM organization_members om JOIN users u ON u.id = om.user_id WHERE om.organization_id = $1 AND u.email = $2`,
		orgID, req.Email,
	).Scan(&exists)
	if err == nil {
		httputil.WriteError(w, http.StatusConflict, "user is already a member")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}

	err = h.db.QueryRow(r.Context(),
		`SELECT 1 FROM organization_invites WHERE organization_id = $1 AND email = $2 AND accepted_at IS NULL AND expires_at > now()`,
		orgID, req.Email,
	).Scan(&exists)
	if err == nil {
		httputil.WriteError(w, http.StatusConflict, "invite already pending")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to check pending invites")
		return
	}

	rawToken, tokenHash, err := generateInviteToken()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate invite token")
		return
	}

	expiresAt := time.Now().Add(inviteTokenExpiry)
	var inviteID string
	var createdAt time.Time
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO organization_invites (organization_id, email, role, invited_by, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		orgID, req.Email, req.Role, userID, tokenHash, expiresAt,
	).Scan(&inviteID, &createdAt)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	if h.emailSender != nil {
		var orgName string
		if err := h.db.QueryRow(r.Context(), `SELECT name FROM organizations WHERE id = $1`, orgID).Scan(&orgName); err != nil {
			slog.Error("send-invite: failed to load org name for email", "error", err)
		}
		var inviterName string
		if err := h.db.QueryRow(r.Context(), `SELECT name FROM users WHERE id = $1`, userID).Scan(&inviterName); err != nil {
			slog.Error("send-invite: failed to load inviter name for email", "error", err)
		}
		acceptLink := h.baseURL + "/invites/accept?token=" + rawToken
		if err := h.emailSender.SendOrgInvite(r.Context(), req.Email, orgName, inviterName, acceptLink); err != nil {
			slog.Error("send-invite: failed to send invite email", "error", err)
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, inviteResponse{
		ID:        inviteID,
		Email:     req.Email,
		Role:      req.Role,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		CreatedAt: createdAt.Format(time.RFC3339),
	})
}

func (h *Handler) ListInvites(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var callerRole string
	err := h.db.QueryRow(r.Context(),
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&callerRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "organization not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify membership")
		return
	}
	if callerRole != "owner" && callerRole != "admin" {
		httputil.WriteError(w, http.StatusForbidden, "only owners and admins can list invites")
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT i.id, i.email, i.role, u.name AS invited_by_name, i.expires_at, i.created_at
		 FROM organization_invites i
		 JOIN users u ON u.id = i.invited_by
		 WHERE i.organization_id = $1 AND i.accepted_at IS NULL AND i.expires_at > now()
		 ORDER BY i.created_at DESC`,
		orgID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list invites")
		return
	}
	defer rows.Close()

	items := make([]inviteListItem, 0)
	for rows.Next() {
		var item inviteListItem
		var expiresAt, createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Email, &item.Role, &item.InvitedByName, &expiresAt, &createdAt); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan invite")
			return
		}
		item.ExpiresAt = expiresAt.Format(time.RFC3339)
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	inviteID := chi.URLParam(r, "inviteId")

	var callerRole string
	err := h.db.QueryRow(r.Context(),
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&callerRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "organization not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify membership")
		return
	}
	if callerRole != "owner" && callerRole != "admin" {
		httputil.WriteError(w, http.StatusForbidden, "only owners and admins can revoke invites")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM organization_invites WHERE id = $1 AND organization_id = $2 AND accepted_at IS NULL`,
		inviteID, orgID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to revoke invite")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "invite not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		httputil.WriteError(w, http.StatusBadRequest, "token is required")
		return
	}

	tokenHash := hashInviteToken(req.Token)

	var inviteID, inviteOrgID, inviteRole string
	err := h.db.QueryRow(r.Context(),
		`SELECT i.id, i.organization_id, i.role
		 FROM organization_invites i
		 WHERE i.token_hash = $1 AND i.accepted_at IS NULL AND i.expires_at > now()`,
		tokenHash,
	).Scan(&inviteID, &inviteOrgID, &inviteRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusBadRequest, "invalid or expired invite")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to look up invite")
		return
	}

	var userEmail string
	err = h.db.QueryRow(r.Context(),
		`SELECT email FROM users WHERE id = $1`, userID,
	).Scan(&userEmail)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to look up user")
		return
	}

	var inviteEmail string
	err = h.db.QueryRow(r.Context(),
		`SELECT email FROM organization_invites WHERE id = $1`, inviteID,
	).Scan(&inviteEmail)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to look up invite email")
		return
	}

	if !strings.EqualFold(userEmail, inviteEmail) {
		httputil.WriteError(w, http.StatusForbidden, "invite was sent to a different email")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		`UPDATE organization_invites SET accepted_at = now() WHERE id = $1`, inviteID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to accept invite")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		inviteOrgID, userID, inviteRole,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to add membership")
		return
	}

	var resp acceptInviteResponse
	err = h.db.QueryRow(r.Context(),
		`SELECT id, name, slug FROM organizations WHERE id = $1`, inviteOrgID,
	).Scan(&resp.OrganizationID, &resp.Name, &resp.Slug)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load organization")
		return
	}
	resp.Role = inviteRole

	httputil.WriteJSON(w, http.StatusOK, resp)
}
