package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

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
	json.NewEncoder(w).Encode(v)
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
	h.writeJSON(w, http.StatusOK, []map[string]interface{}{
		{
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
		},
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

	if _, err := h.db.Exec(r.Context(),
		"INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		orgID, userID, "member",
	); err != nil {
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

	var emailVerified bool
	err = h.db.QueryRow(ctx,
		"SELECT id, email_verified FROM users WHERE email = $1",
		email,
	).Scan(&userID, &emailVerified)
	if err == nil {
		h.db.Exec(ctx,
			"INSERT INTO external_identities (user_id, provider, external_id, email) VALUES ($1, $2, $3, $4) ON CONFLICT (provider, external_id) DO NOTHING",
			userID, orgID, externalID, email,
		)
		return userID, false, nil
	}

	err = h.db.QueryRow(ctx,
		`INSERT INTO users (email, password, name, email_verified) VALUES ($1, $2, $3, true)
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING id`,
		email, "", name,
	).Scan(&userID)
	if err != nil {
		return "", false, fmt.Errorf("create user: %w", err)
	}

	h.db.Exec(ctx,
		"INSERT INTO external_identities (user_id, provider, external_id, email) VALUES ($1, $2, $3, $4) ON CONFLICT (provider, external_id) DO NOTHING",
		userID, orgID, externalID, email,
	)

	return userID, true, nil
}

func (h *Handler) fetchSCIMUser(ctx context.Context, userID, orgID string) (*SCIMUser, error) {
	var id, email, name string
	err := h.db.QueryRow(ctx,
		`SELECT u.id, u.email, u.name FROM users u
		 JOIN organization_members om ON om.user_id = u.id AND om.organization_id = $2
		 WHERE u.id = $1`,
		userID, orgID,
	).Scan(&id, &email, &name)
	if err != nil {
		return nil, err
	}

	return &SCIMUser{
		Schemas:  []string{UserSchema},
		ID:       id,
		UserName: email,
		Name:     SCIMName{Formatted: name},
		Emails:   []SCIMEmail{{Value: email, Primary: true}},
		Active:   true,
		Meta: &SCIMMeta{
			ResourceType: "User",
			Location:     h.baseURL + "/api/organizations/" + orgID + "/scim/v2/Users/" + id,
		},
	}, nil
}
