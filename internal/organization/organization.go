package organization

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/validate"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

type orgResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	SubscriptionPlan string `json:"subscriptionPlan"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type orgListItem struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	SubscriptionPlan string `json:"subscriptionPlan"`
	Role             string `json:"role"`
	MemberCount      int64  `json:"memberCount"`
}

type orgDetailResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	SubscriptionPlan string `json:"subscriptionPlan"`
	Role             string `json:"role"`
	MemberCount      int64  `json:"memberCount"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type createOrgRequest struct {
	Name string `json:"name"`
}

type updateOrgRequest struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = nonAlphanumeric.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "org"
	}
	return slug
}

func randomHexSuffix() (string, error) {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req createOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "organization name is required")
		return
	}
	if msg := validate.OrgName(name); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	var plan string
	if err := h.db.QueryRow(r.Context(),
		`SELECT subscription_plan FROM users WHERE id = $1`, userID,
	).Scan(&plan); err != nil {
		plan = "free"
	}

	if plan == "free" {
		var ownerCount int
		if err := h.db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM organization_members WHERE user_id = $1 AND role = 'owner'`,
			userID,
		).Scan(&ownerCount); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to check organization limit")
			return
		}
		if ownerCount >= 1 {
			httputil.WriteError(w, http.StatusForbidden, "free plan allows 1 organization")
			return
		}
	}

	slug := generateSlug(name)
	if msg := validate.OrgSlug(slug); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	var resp orgResponse
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id, slug, created_at, updated_at`,
		name, slug,
	).Scan(&resp.ID, &resp.Slug, &createdAt, &updatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			suffix, suffixErr := randomHexSuffix()
			if suffixErr != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "failed to create organization")
				return
			}
			slug = slug + "-" + suffix
			err = h.db.QueryRow(r.Context(),
				`INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id, slug, created_at, updated_at`,
				name, slug,
			).Scan(&resp.ID, &resp.Slug, &createdAt, &updatedAt)
			if err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "failed to create organization")
				return
			}
		} else {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to create organization")
			return
		}
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner')`,
		resp.ID, userID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to add owner to organization")
		return
	}

	resp.Name = name
	resp.SubscriptionPlan = "free"
	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT o.id, o.name, o.slug, o.subscription_plan, om.role,
		        (SELECT COUNT(*) FROM organization_members WHERE organization_id = o.id) AS member_count
		 FROM organizations o
		 JOIN organization_members om ON om.organization_id = o.id
		 WHERE om.user_id = $1
		 ORDER BY o.name`,
		userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}
	defer rows.Close()

	items := make([]orgListItem, 0)
	for rows.Next() {
		var item orgListItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.SubscriptionPlan, &item.Role, &item.MemberCount); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan organization")
			return
		}
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var resp orgDetailResponse
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(r.Context(),
		`SELECT o.id, o.name, o.slug, o.subscription_plan, o.created_at, o.updated_at, om.role,
		        (SELECT COUNT(*) FROM organization_members WHERE organization_id = o.id) AS member_count
		 FROM organizations o
		 JOIN organization_members om ON om.organization_id = o.id
		 WHERE o.id = $1 AND om.user_id = $2`,
		orgID, userID,
	).Scan(&resp.ID, &resp.Name, &resp.Slug, &resp.SubscriptionPlan, &createdAt, &updatedAt, &resp.Role, &resp.MemberCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "organization not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get organization")
		return
	}

	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var role string
	err := h.db.QueryRow(r.Context(),
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "organization not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify membership")
		return
	}
	if role != "owner" && role != "admin" {
		httputil.WriteError(w, http.StatusForbidden, "only owners and admins can update the organization")
		return
	}

	var req updateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == nil && req.Slug == nil {
		httputil.WriteError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			httputil.WriteError(w, http.StatusBadRequest, "organization name is required")
			return
		}
		if msg := validate.OrgName(trimmed); msg != "" {
			httputil.WriteError(w, http.StatusBadRequest, msg)
			return
		}
		req.Name = &trimmed
	}

	if req.Slug != nil {
		trimmed := strings.TrimSpace(*req.Slug)
		if trimmed == "" {
			httputil.WriteError(w, http.StatusBadRequest, "organization slug is required")
			return
		}
		if msg := validate.OrgSlug(trimmed); msg != "" {
			httputil.WriteError(w, http.StatusBadRequest, msg)
			return
		}
		req.Slug = &trimmed
	}

	setClauses := []string{}
	args := []any{}
	paramIdx := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", paramIdx))
		args = append(args, *req.Name)
		paramIdx++
	}
	if req.Slug != nil {
		setClauses = append(setClauses, fmt.Sprintf("slug = $%d", paramIdx))
		args = append(args, *req.Slug)
		paramIdx++
	}

	setClauses = append(setClauses, "updated_at = now()")

	query := fmt.Sprintf("UPDATE organizations SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), paramIdx)
	args = append(args, orgID)

	tag, err := h.db.Exec(r.Context(), query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httputil.WriteError(w, http.StatusConflict, "an organization with this slug already exists")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update organization")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "organization not found")
		return
	}

	var resp orgDetailResponse
	var createdAt, updatedAt time.Time
	err = h.db.QueryRow(r.Context(),
		`SELECT o.id, o.name, o.slug, o.subscription_plan, o.created_at, o.updated_at,
		        (SELECT role FROM organization_members WHERE organization_id = o.id AND user_id = $2) AS role,
		        (SELECT COUNT(*) FROM organization_members WHERE organization_id = o.id) AS member_count
		 FROM organizations o
		 WHERE o.id = $1`,
		orgID, userID,
	).Scan(&resp.ID, &resp.Name, &resp.Slug, &resp.SubscriptionPlan, &createdAt, &updatedAt, &resp.Role, &resp.MemberCount)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to read updated organization")
		return
	}

	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var role string
	err := h.db.QueryRow(r.Context(),
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "organization not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify membership")
		return
	}
	if role != "owner" {
		httputil.WriteError(w, http.StatusForbidden, "only owners can delete the organization")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM organizations WHERE id = $1`,
		orgID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete organization")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "organization not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
