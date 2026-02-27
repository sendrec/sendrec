package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/validate"
)

const (
	apiKeyPrefix    = "sr_"
	apiKeyRandBytes = 32
	maxAPIKeys      = 10
)

type generateAPIKeyRequest struct {
	Name string `json:"name"`
}

type generateAPIKeyResponse struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

type apiKeyItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt"`
}

func generateAPIKeyString() (string, error) {
	b := make([]byte, apiKeyRandBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return apiKeyPrefix + hex.EncodeToString(b), nil
}

func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func GenerateAPIKey(db database.DBTX) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		var req generateAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if msg := validate.APIKeyName(req.Name); msg != "" {
			httputil.WriteError(w, http.StatusBadRequest, msg)
			return
		}

		var count int
		err := db.QueryRow(r.Context(),
			"SELECT COUNT(*) FROM api_keys WHERE user_id = $1", userID,
		).Scan(&count)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to check key count")
			return
		}

		if count >= maxAPIKeys {
			httputil.WriteError(w, http.StatusBadRequest, "maximum number of API keys reached")
			return
		}

		plaintext, err := generateAPIKeyString()
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to generate API key")
			return
		}

		keyHash := HashAPIKey(plaintext)

		var id string
		var createdAt time.Time
		err = db.QueryRow(r.Context(),
			"INSERT INTO api_keys (user_id, key_hash, name) VALUES ($1, $2, $3) RETURNING id, created_at",
			userID, keyHash, req.Name,
		).Scan(&id, &createdAt)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to create API key")
			return
		}

		httputil.WriteJSON(w, http.StatusCreated, generateAPIKeyResponse{
			ID:        id,
			Key:       plaintext,
			Name:      req.Name,
			CreatedAt: createdAt.Format(time.RFC3339),
		})
	}
}

func ListAPIKeys(db database.DBTX) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		rows, err := db.Query(r.Context(),
			"SELECT id, name, created_at, last_used_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC",
			userID,
		)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list API keys")
			return
		}
		defer rows.Close()

		items := make([]apiKeyItem, 0)
		for rows.Next() {
			var item apiKeyItem
			var createdAt time.Time
			var lastUsedAt *time.Time
			if err := rows.Scan(&item.ID, &item.Name, &createdAt, &lastUsedAt); err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "failed to scan API key")
				return
			}
			item.CreatedAt = createdAt.Format(time.RFC3339)
			if lastUsedAt != nil {
				formatted := lastUsedAt.Format(time.RFC3339)
				item.LastUsedAt = &formatted
			}
			items = append(items, item)
		}

		httputil.WriteJSON(w, http.StatusOK, items)
	}
}

func DeleteAPIKey(db database.DBTX) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		keyID := chi.URLParam(r, "id")

		result, err := db.Exec(r.Context(),
			"DELETE FROM api_keys WHERE id = $1 AND user_id = $2",
			keyID, userID,
		)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to delete API key")
			return
		}

		if result.RowsAffected() == 0 {
			httputil.WriteError(w, http.StatusNotFound, "API key not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

var errAPIKeyNotFound = errors.New("API key not found")

func LookupAPIKey(ctx context.Context, db database.DBTX, token string) (string, error) {
	if len(token) < len(apiKeyPrefix) || token[:len(apiKeyPrefix)] != apiKeyPrefix {
		return "", errAPIKeyNotFound
	}

	keyHash := HashAPIKey(token)

	var userID string
	err := db.QueryRow(ctx,
		"SELECT user_id FROM api_keys WHERE key_hash = $1", keyHash,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errAPIKeyNotFound
		}
		return "", fmt.Errorf("lookup API key: %w", err)
	}

	go func() {
		if _, err := db.Exec(context.Background(),
			"UPDATE api_keys SET last_used_at = now() WHERE key_hash = $1", keyHash,
		); err != nil {
			slog.Error("failed to update API key last_used_at", "error", err)
		}
	}()

	return userID, nil
}
