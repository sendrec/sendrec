package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sendrec/sendrec/internal/database"
)

type Handler struct {
	db      database.DBTX
	baseURL string
}

func NewHandler(db database.DBTX, baseURL string) *Handler {
	return &Handler{db: db, baseURL: baseURL}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) ServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"schemas":        []string{SPConfigSchema},
		"patch":          map[string]bool{"supported": true},
		"bulk":           map[string]interface{}{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":         map[string]interface{}{"supported": true, "maxResults": 100},
		"changePassword": map[string]bool{"supported": false},
		"sort":           map[string]bool{"supported": false},
		"etag":           map[string]bool{"supported": false},
		"authenticationSchemes": []map[string]string{{
			"type":        "oauthbearertoken",
			"name":        "OAuth Bearer Token",
			"description": "Authentication scheme using the OAuth Bearer Token Standard",
		}},
	})
}

func (h *Handler) Schemas(w http.ResponseWriter, r *http.Request) {
	resources := []map[string]interface{}{{
		"id":          UserSchema,
		"name":        "User",
		"description": "User Account",
		"attributes": []map[string]interface{}{
			{"name": "userName", "type": "string", "required": true, "uniqueness": "server"},
			{"name": "name", "type": "complex", "subAttributes": []map[string]string{
				{"name": "formatted", "type": "string"},
				{"name": "givenName", "type": "string"},
				{"name": "familyName", "type": "string"},
			}},
			{"name": "emails", "type": "complex", "multiValued": true},
			{"name": "active", "type": "boolean", "required": true},
			{"name": "externalId", "type": "string"},
		},
	}}
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"schemas":      []string{ListResponseSchema},
		"totalResults": len(resources),
		"itemsPerPage": len(resources),
		"startIndex":   1,
		"Resources":    resources,
	})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var req SCIMUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	email := req.UserName
	if email == "" && len(req.Emails) > 0 {
		email = req.Emails[0].Value
	}
	if email == "" {
		writeError(w, http.StatusBadRequest, "userName or emails required")
		return
	}

	name := req.Name.Formatted
	if name == "" && req.Name.GivenName != "" {
		name = req.Name.GivenName
		if req.Name.FamilyName != "" {
			name += " " + req.Name.FamilyName
		}
	}

	externalID := req.ExternalID
	if externalID == "" {
		externalID = email
	}

	userID, created, err := h.resolveUser(r.Context(), orgID, externalID, email, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "provision user failed")
		return
	}

	if err := h.setMembership(r.Context(), orgID, userID, true); err != nil {
		writeError(w, http.StatusInternalServerError, "add member failed")
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}

	user, err := h.fetchSCIMUser(r.Context(), userID, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch user failed")
		return
	}
	h.writeJSON(w, status, user)
}

func (h *Handler) resolveUser(ctx context.Context, orgID, externalID, email, name string) (string, bool, error) {
	var userID string
	err := h.db.QueryRow(ctx,
		"SELECT user_id FROM external_identities WHERE provider = $1 AND external_id = $2",
		orgID, externalID,
	).Scan(&userID)
	if err == nil {
		return userID, false, nil
	}
	if err != nil && err != pgx.ErrNoRows {
		return "", false, fmt.Errorf("lookup external identity: %w", err)
	}

	var emailVerified bool
	err = h.db.QueryRow(ctx,
		"SELECT id, email_verified FROM users WHERE email = $1",
		email,
	).Scan(&userID, &emailVerified)
	if err == nil {
		if !emailVerified {
			return "", false, fmt.Errorf("email not verified")
		}
		if err := h.upsertExternalIdentity(ctx, userID, orgID, externalID, email); err != nil {
			return "", false, fmt.Errorf("link identity: %w", err)
		}
		return userID, false, nil
	}
	if err != nil && err != pgx.ErrNoRows {
		return "", false, fmt.Errorf("lookup user by email: %w", err)
	}

	err = h.db.QueryRow(ctx,
		`INSERT INTO users (email, password, name, email_verified) VALUES ($1, $2, $3, true)
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING id`,
		email, "", name,
	).Scan(&userID)
	if err != nil {
		return "", false, fmt.Errorf("create user: %w", err)
	}

	if err := h.upsertExternalIdentity(ctx, userID, orgID, externalID, email); err != nil {
		return "", false, fmt.Errorf("create identity: %w", err)
	}

	return userID, true, nil
}

func (h *Handler) fetchSCIMUser(ctx context.Context, userID, orgID string) (*SCIMUser, error) {
	var id, email, name, externalID string
	var active bool
	err := h.db.QueryRow(ctx,
		`SELECT u.id, u.email, u.name, COALESCE(ei.external_id, '') AS external_id, (om.user_id IS NOT NULL) AS active FROM users u
		 LEFT JOIN external_identities ei ON ei.user_id = u.id AND ei.provider = $2
		 LEFT JOIN organization_members om ON om.user_id = u.id AND om.organization_id = $2
		 WHERE u.id = $1 AND (ei.user_id IS NOT NULL OR om.user_id IS NOT NULL)`,
		userID, orgID,
	).Scan(&id, &email, &name, &externalID, &active)
	if err != nil {
		return nil, err
	}

	return h.scimUser(id, email, name, externalID, active, orgID), nil
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	userID := chi.URLParam(r, "id")

	user, err := h.fetchSCIMUser(r.Context(), userID, orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}
	h.writeJSON(w, http.StatusOK, user)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	count := 100
	startIndex := 1
	if c := r.URL.Query().Get("count"); c != "" {
		if v, err := strconv.Atoi(c); err == nil && v > 0 && v <= 100 {
			count = v
		}
	}
	if s := r.URL.Query().Get("startIndex"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			startIndex = v
		}
	}
	offset := startIndex - 1

	filterEmail := parseUserNameFilter(r.URL.Query().Get("filter"))

	var total int
	var rows pgx.Rows
	var err error

	if filterEmail != "" {
		err = h.db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM users u
			 LEFT JOIN external_identities ei ON ei.user_id = u.id AND ei.provider = $1
			 LEFT JOIN organization_members om ON om.user_id = u.id AND om.organization_id = $1
			 WHERE (ei.user_id IS NOT NULL OR om.user_id IS NOT NULL) AND u.email = $2`,
			orgID, filterEmail,
		).Scan(&total)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		rows, err = h.db.Query(r.Context(),
			`SELECT u.id, u.email, u.name, COALESCE(ei.external_id, '') AS external_id, (om.user_id IS NOT NULL) AS active FROM users u
			 LEFT JOIN external_identities ei ON ei.user_id = u.id AND ei.provider = $1
			 LEFT JOIN organization_members om ON om.user_id = u.id AND om.organization_id = $1
			 WHERE (ei.user_id IS NOT NULL OR om.user_id IS NOT NULL) AND u.email = $2
			 ORDER BY u.email LIMIT $3 OFFSET $4`,
			orgID, filterEmail, count, offset,
		)
	} else {
		err = h.db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM users u
			 LEFT JOIN external_identities ei ON ei.user_id = u.id AND ei.provider = $1
			 LEFT JOIN organization_members om ON om.user_id = u.id AND om.organization_id = $1
			 WHERE ei.user_id IS NOT NULL OR om.user_id IS NOT NULL`,
			orgID,
		).Scan(&total)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		rows, err = h.db.Query(r.Context(),
			`SELECT u.id, u.email, u.name, COALESCE(ei.external_id, '') AS external_id, (om.user_id IS NOT NULL) AS active FROM users u
			 LEFT JOIN external_identities ei ON ei.user_id = u.id AND ei.provider = $1
			 LEFT JOIN organization_members om ON om.user_id = u.id AND om.organization_id = $1
			 WHERE ei.user_id IS NOT NULL OR om.user_id IS NOT NULL
			 ORDER BY u.email LIMIT $2 OFFSET $3`,
			orgID, count, offset,
		)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	users := make([]SCIMUser, 0)
	for rows.Next() {
		var id, email, name, externalID string
		var active bool
		if err := rows.Scan(&id, &email, &name, &externalID, &active); err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		users = append(users, *h.scimUser(id, email, name, externalID, active, orgID))
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	h.writeJSON(w, http.StatusOK, SCIMListResponse{
		Schemas:      []string{ListResponseSchema},
		TotalResults: total,
		ItemsPerPage: len(users),
		StartIndex:   startIndex,
		Resources:    users,
	})
}

func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	userID := chi.URLParam(r, "id")

	var patch SCIMPatchOp
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	var returnNoContent bool
	for _, op := range patch.Operations {
		if !strings.EqualFold(op.Op, "replace") {
			continue
		}

		update, err := parsePatchOperation(op)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid patch operation")
			return
		}
		if err := h.applyUserUpdate(r.Context(), orgID, userID, update); err != nil {
			writeError(w, http.StatusInternalServerError, "update user failed")
			return
		}
		if update.Active != nil && !*update.Active {
			returnNoContent = true
		}
	}

	if returnNoContent {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	user, err := h.fetchSCIMUser(r.Context(), userID, orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}
	h.writeJSON(w, http.StatusOK, user)
}

func (h *Handler) ReplaceUser(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	userID := chi.URLParam(r, "id")

	var req SCIMUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	email := req.UserName
	if email == "" && len(req.Emails) > 0 {
		email = req.Emails[0].Value
	}
	if email == "" {
		writeError(w, http.StatusBadRequest, "userName or emails required")
		return
	}

	update := scimUserUpdate{
		Email:      &email,
		Name:       stringPtr(formattedName(req.Name)),
		Active:     &req.Active,
		ExternalID: stringPtr(req.ExternalID),
	}
	if update.Name != nil && *update.Name == "" {
		update.Name = nil
	}
	if update.ExternalID != nil && *update.ExternalID == "" {
		update.ExternalID = nil
	}

	if err := h.applyUserUpdate(r.Context(), orgID, userID, update); err != nil {
		writeError(w, http.StatusInternalServerError, "replace user failed")
		return
	}

	user, err := h.fetchSCIMUser(r.Context(), userID, orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}
	h.writeJSON(w, http.StatusOK, user)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	userID := chi.URLParam(r, "id")

	if err := h.setMembership(r.Context(), orgID, userID, false); err != nil {
		writeError(w, http.StatusInternalServerError, "delete user failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseUserNameFilter extracts email from `userName eq "user@example.com"`.
func parseUserNameFilter(filter string) string {
	if filter == "" {
		return ""
	}
	parts := strings.SplitN(filter, " eq ", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) != "userName" {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parts[1]), `"`)
}

type scimUserUpdate struct {
	Email      *string
	Name       *string
	Active     *bool
	ExternalID *string
}

func (h *Handler) applyUserUpdate(ctx context.Context, orgID, userID string, update scimUserUpdate) error {
	if update.Email != nil || update.Name != nil {
		if err := h.updateUserProfile(ctx, userID, update.Email, update.Name); err != nil {
			return err
		}
	}
	if update.Active != nil {
		if err := h.setMembership(ctx, orgID, userID, *update.Active); err != nil {
			return err
		}
	}
	if update.ExternalID != nil {
		email := ""
		if update.Email != nil {
			email = *update.Email
		}
		if err := h.upsertExternalIdentity(ctx, userID, orgID, *update.ExternalID, email); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) updateUserProfile(ctx context.Context, userID string, email, name *string) error {
	switch {
	case email != nil && name != nil:
		_, err := h.db.Exec(ctx,
			"UPDATE users SET email = $1, name = $2 WHERE id = $3",
			*email, *name, userID,
		)
		return err
	case email != nil:
		_, err := h.db.Exec(ctx,
			"UPDATE users SET email = $1 WHERE id = $2",
			*email, userID,
		)
		return err
	case name != nil:
		_, err := h.db.Exec(ctx,
			"UPDATE users SET name = $1 WHERE id = $2",
			*name, userID,
		)
		return err
	default:
		return nil
	}
}

func (h *Handler) setMembership(ctx context.Context, orgID, userID string, active bool) error {
	if active {
		_, err := h.db.Exec(ctx,
			"INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
			orgID, userID, "member",
		)
		return err
	}
	_, err := h.db.Exec(ctx,
		"DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2",
		orgID, userID,
	)
	return err
}

func (h *Handler) upsertExternalIdentity(ctx context.Context, userID, orgID, externalID, email string) error {
	_, err := h.db.Exec(ctx,
		"INSERT INTO external_identities (user_id, provider, external_id, email) VALUES ($1, $2, $3, $4) ON CONFLICT (provider, external_id) DO NOTHING",
		userID, orgID, externalID, email,
	)
	return err
}

func (h *Handler) scimUser(id, email, name, externalID string, active bool, orgID string) *SCIMUser {
	user := &SCIMUser{
		Schemas:  []string{UserSchema},
		ID:       id,
		UserName: email,
		Name:     SCIMName{Formatted: name},
		Emails:   []SCIMEmail{{Value: email, Primary: true}},
		Active:   active,
		Meta: &SCIMMeta{
			ResourceType: "User",
			Location:     h.baseURL + "/api/organizations/" + orgID + "/scim/v2/Users/" + id,
		},
	}
	if externalID != "" {
		user.ExternalID = externalID
	}
	return user
}

func parsePatchOperation(op SCIMOperation) (scimUserUpdate, error) {
	update := scimUserUpdate{}
	if op.Path != "" {
		switch op.Path {
		case "active":
			active, ok := op.Value.(bool)
			if !ok {
				return scimUserUpdate{}, fmt.Errorf("invalid active value")
			}
			update.Active = &active
		case "name.formatted":
			name, ok := op.Value.(string)
			if !ok {
				return scimUserUpdate{}, fmt.Errorf("invalid name value")
			}
			update.Name = &name
		case "userName":
			email, ok := op.Value.(string)
			if !ok {
				return scimUserUpdate{}, fmt.Errorf("invalid userName value")
			}
			update.Email = &email
		}
		return update, nil
	}

	value, ok := op.Value.(map[string]interface{})
	if !ok {
		return scimUserUpdate{}, fmt.Errorf("invalid patch value")
	}
	if activeValue, ok := value["active"]; ok {
		active, ok := activeValue.(bool)
		if !ok {
			return scimUserUpdate{}, fmt.Errorf("invalid active value")
		}
		update.Active = &active
	}
	if emailValue, ok := value["userName"]; ok {
		email, ok := emailValue.(string)
		if !ok {
			return scimUserUpdate{}, fmt.Errorf("invalid userName value")
		}
		update.Email = &email
	}
	if nameValue, ok := value["name"].(map[string]interface{}); ok {
		if formattedValue, ok := nameValue["formatted"]; ok {
			name, ok := formattedValue.(string)
			if !ok {
				return scimUserUpdate{}, fmt.Errorf("invalid name value")
			}
			update.Name = &name
		}
	}
	if externalIDValue, ok := value["externalId"]; ok {
		externalID, ok := externalIDValue.(string)
		if !ok {
			return scimUserUpdate{}, fmt.Errorf("invalid externalId value")
		}
		update.ExternalID = &externalID
	}
	return update, nil
}

func formattedName(name SCIMName) string {
	if name.Formatted != "" {
		return name.Formatted
	}
	if name.GivenName == "" {
		return ""
	}
	if name.FamilyName == "" {
		return name.GivenName
	}
	return name.GivenName + " " + name.FamilyName
}

func stringPtr(value string) *string {
	return &value
}
