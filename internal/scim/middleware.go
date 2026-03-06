package scim

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sendrec/sendrec/internal/database"
)

func BearerAuth(db database.DBTX) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgID := chi.URLParam(r, "orgId")

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")

			var storedHash string
			var subscriptionPlan string
			err := db.QueryRow(r.Context(),
				`SELECT st.token_hash, o.subscription_plan
				 FROM organization_scim_tokens st
				 JOIN organizations o ON o.id = st.organization_id
				 WHERE st.organization_id = $1`,
				orgID,
			).Scan(&storedHash, &subscriptionPlan)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					writeError(w, http.StatusUnauthorized, "SCIM not configured for this organization")
					return
				}
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if subscriptionPlan != "business" {
				writeError(w, http.StatusForbidden, "business plan required")
				return
			}

			hash := sha256.Sum256([]byte(token))
			if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(hash[:])), []byte(storedHash)) != 1 {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(SCIMError{
		Schemas: []string{ErrorSchema},
		Status:  strconv.Itoa(status),
		Detail:  detail,
	})
}
