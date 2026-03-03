package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
)

var validProviders = map[string]bool{
	"github": true,
	"jira":   true,
}

var tokenFields = map[string][]string{
	"github": {"token"},
	"jira":   {"api_token"},
}

var requiredFields = map[string][]string{
	"github": {"token", "owner", "repo"},
	"jira":   {"base_url", "email", "api_token", "project_key"},
}

// Handler provides HTTP endpoints for managing user integrations.
type Handler struct {
	db      database.DBTX
	encKey  []byte
	baseURL string
}

// NewHandler creates a Handler with the given database, encryption key, and application base URL.
func NewHandler(db database.DBTX, encKey []byte, baseURL string) *Handler {
	return &Handler{db: db, encKey: encKey, baseURL: baseURL}
}

// List returns all integrations for the authenticated user with masked tokens.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	rows, err := h.db.Query(r.Context(),
		"SELECT id, provider, config, created_at, updated_at FROM user_integrations WHERE user_id = $1 ORDER BY created_at",
		userID,
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	integrations := []Integration{}
	for rows.Next() {
		var ig Integration
		var configBytes []byte
		if err := rows.Scan(&ig.ID, &ig.Provider, &configBytes, &ig.CreatedAt, &ig.UpdatedAt); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if err := json.Unmarshal(configBytes, &ig.Config); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		maskTokenFields(ig.Provider, ig.Config, h.encKey)
		integrations = append(integrations, ig)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(integrations)
}

// Save validates and upserts an integration configuration for the authenticated user.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	provider := chi.URLParam(r, "provider")

	if !validProviders[provider] {
		http.Error(w, fmt.Sprintf("unsupported provider: %s", provider), http.StatusBadRequest)
		return
	}

	var config map[string]any
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateConfig(provider, config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.preserveOrEncryptTokens(r, userID, provider, config); err != nil {
		http.Error(w, "encryption error", http.StatusInternalServerError)
		return
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		"INSERT INTO user_integrations (user_id, provider, config) VALUES ($1, $2, $3) ON CONFLICT (user_id, provider) DO UPDATE SET config = $3, updated_at = now()",
		userID, provider, configJSON,
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Delete removes an integration configuration for the authenticated user.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	provider := chi.URLParam(r, "provider")

	if !validProviders[provider] {
		http.Error(w, fmt.Sprintf("unsupported provider: %s", provider), http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(r.Context(),
		"DELETE FROM user_integrations WHERE user_id = $1 AND provider = $2",
		userID, provider,
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Test loads the integration config from the database, decrypts tokens,
// builds a provider client, and calls ValidateConfig.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	provider := chi.URLParam(r, "provider")

	config, err := h.loadConfig(r, userID, provider)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	creator, err := newCreator(provider, config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := creator.ValidateConfig(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// CreateIssue loads the video and integration config, then files an issue
// with the configured provider.
func (h *Handler) CreateIssue(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var title, shareToken string
	var transcriptJSON []byte

	orgID := auth.OrgIDFromContext(r.Context())
	var err error
	if orgID != "" {
		role := auth.OrgRoleFromContext(r.Context())
		if role == "owner" || role == "admin" {
			err = h.db.QueryRow(r.Context(),
				"SELECT title, share_token, transcript_json FROM videos WHERE id = $1 AND organization_id = $2",
				videoID, orgID,
			).Scan(&title, &shareToken, &transcriptJSON)
		} else {
			err = h.db.QueryRow(r.Context(),
				"SELECT title, share_token, transcript_json FROM videos WHERE id = $1 AND user_id = $2 AND organization_id = $3",
				videoID, userID, orgID,
			).Scan(&title, &shareToken, &transcriptJSON)
		}
	} else {
		err = h.db.QueryRow(r.Context(),
			"SELECT title, share_token, transcript_json FROM videos WHERE id = $1 AND user_id = $2 AND organization_id IS NULL",
			videoID, userID,
		).Scan(&title, &shareToken, &transcriptJSON)
	}
	if err != nil {
		http.Error(w, "video not found", http.StatusNotFound)
		return
	}

	var body struct {
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Provider == "" {
		http.Error(w, "provider is required", http.StatusBadRequest)
		return
	}

	config, err := h.loadConfig(r, userID, body.Provider)
	if err != nil {
		http.Error(w, "integration not configured", http.StatusNotFound)
		return
	}

	creator, err := newCreator(body.Provider, config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	videoURL := h.baseURL + "/watch/" + shareToken
	description := extractTranscriptText(transcriptJSON, 500)

	resp, err := creator.CreateIssue(r.Context(), CreateIssueRequest{
		Title:       title,
		Description: description,
		VideoURL:    videoURL,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) loadConfig(r *http.Request, userID, provider string) (map[string]any, error) {
	var configBytes []byte
	err := h.db.QueryRow(r.Context(),
		"SELECT config FROM user_integrations WHERE user_id = $1 AND provider = $2",
		userID, provider,
	).Scan(&configBytes)
	if err != nil {
		return nil, fmt.Errorf("integration not found")
	}

	var config map[string]any
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("invalid config")
	}

	if err := decryptTokenFields(provider, config, h.encKey); err != nil {
		return nil, fmt.Errorf("decryption error")
	}

	return config, nil
}

func validateConfig(provider string, config map[string]any) error {
	fields, ok := requiredFields[provider]
	if !ok {
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	for _, field := range fields {
		if stringVal(config, field) == "" {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	return nil
}

func isMasked(val string) bool {
	return strings.HasSuffix(val, "******")
}

func (h *Handler) preserveOrEncryptTokens(r *http.Request, userID, provider string, config map[string]any) error {
	var existing map[string]any
	for _, field := range tokenFields[provider] {
		val := stringVal(config, field)
		if val == "" || !isMasked(val) {
			continue
		}
		if existing == nil {
			var err error
			existing, err = h.loadConfig(r, userID, provider)
			if err != nil {
				return fmt.Errorf("cannot resolve masked token: no existing config")
			}
		}
	}

	for _, field := range tokenFields[provider] {
		val := stringVal(config, field)
		if val == "" {
			continue
		}
		if isMasked(val) && existing != nil {
			encrypted, err := Encrypt(h.encKey, stringVal(existing, field))
			if err != nil {
				return err
			}
			config[field] = encrypted
		} else {
			encrypted, err := Encrypt(h.encKey, val)
			if err != nil {
				return err
			}
			config[field] = encrypted
		}
	}
	return nil
}

func decryptTokenFields(provider string, config map[string]any, key []byte) error {
	for _, field := range tokenFields[provider] {
		val := stringVal(config, field)
		if val == "" {
			continue
		}
		decrypted, err := Decrypt(key, val)
		if err != nil {
			return err
		}
		config[field] = decrypted
	}
	return nil
}

func maskTokenFields(provider string, config map[string]any, key []byte) {
	for _, field := range tokenFields[provider] {
		val := stringVal(config, field)
		if val == "" {
			continue
		}
		decrypted, err := Decrypt(key, val)
		if err != nil {
			config[field] = MaskToken(val)
			continue
		}
		config[field] = MaskToken(decrypted)
	}
}

func newCreator(provider string, config map[string]any) (IssueCreator, error) {
	switch provider {
	case "github":
		return NewGitHubClient(
			stringVal(config, "token"),
			stringVal(config, "owner"),
			stringVal(config, "repo"),
		), nil
	case "jira":
		return NewJiraClient(
			stringVal(config, "base_url"),
			stringVal(config, "email"),
			stringVal(config, "api_token"),
			stringVal(config, "project_key"),
		), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func stringVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func extractTranscriptText(transcriptJSON []byte, maxLen int) string {
	if len(transcriptJSON) == 0 {
		return ""
	}
	var segments []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(transcriptJSON, &segments); err != nil {
		return ""
	}
	var result string
	for _, seg := range segments {
		if result != "" {
			result += " "
		}
		result += seg.Text
		if len(result) >= maxLen {
			return result[:maxLen]
		}
	}
	return result
}
