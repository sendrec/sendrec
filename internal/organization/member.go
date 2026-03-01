package organization

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

type memberResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	JoinedAt string `json:"joinedAt"`
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
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

	rows, err := h.db.Query(r.Context(),
		`SELECT u.id, u.name, u.email, om.role, om.joined_at
		 FROM organization_members om
		 JOIN users u ON u.id = om.user_id
		 WHERE om.organization_id = $1
		 ORDER BY om.joined_at`,
		orgID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	defer rows.Close()

	members := make([]memberResponse, 0)
	for rows.Next() {
		var m memberResponse
		var joinedAt time.Time
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.Role, &joinedAt); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan member")
			return
		}
		m.JoinedAt = joinedAt.Format(time.RFC3339)
		members = append(members, m)
	}

	httputil.WriteJSON(w, http.StatusOK, members)
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	targetUserID := chi.URLParam(r, "userId")

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
		httputil.WriteError(w, http.StatusForbidden, "only owners and admins can remove members")
		return
	}

	if targetUserID == userID {
		httputil.WriteError(w, http.StatusBadRequest, "cannot remove yourself")
		return
	}

	var targetRole string
	err = h.db.QueryRow(r.Context(),
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, targetUserID,
	).Scan(&targetRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "member not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to check target member")
		return
	}

	if targetRole == "owner" {
		var ownerCount int
		if err := h.db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM organization_members WHERE organization_id = $1 AND role = 'owner'`,
			orgID,
		).Scan(&ownerCount); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to count owners")
			return
		}
		if ownerCount <= 1 {
			httputil.WriteError(w, http.StatusBadRequest, "cannot remove the only owner")
			return
		}
	}

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, targetUserID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "member not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	targetUserID := chi.URLParam(r, "userId")

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
	if callerRole != "owner" {
		httputil.WriteError(w, http.StatusForbidden, "only owners can change roles")
		return
	}

	if targetUserID == userID {
		httputil.WriteError(w, http.StatusBadRequest, "cannot change your own role")
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != "admin" && req.Role != "member" {
		httputil.WriteError(w, http.StatusBadRequest, "role must be admin or member")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE organization_members SET role = $1 WHERE organization_id = $2 AND user_id = $3`,
		req.Role, orgID, targetUserID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update role")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "member not found")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"role": req.Role})
}
