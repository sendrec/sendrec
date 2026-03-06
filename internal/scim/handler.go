package scim

import (
	"encoding/json"
	"net/http"

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
