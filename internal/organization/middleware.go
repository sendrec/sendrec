package organization

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
)

// Middleware reads X-Organization-Id header. If present, verifies the
// authenticated user is a member and injects org context. If absent,
// the request proceeds in personal context.
func Middleware(db database.DBTX) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgID := r.Header.Get("X-Organization-Id")
			if orgID == "" {
				next.ServeHTTP(w, r)
				return
			}

			userID := auth.UserIDFromContext(r.Context())
			if userID == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			var role string
			err := db.QueryRow(r.Context(),
				`SELECT om.role FROM organization_members om
				 JOIN organizations o ON o.id = om.organization_id
				 WHERE om.organization_id = $1 AND om.user_id = $2`,
				orgID, userID,
			).Scan(&role)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					httputil.WriteError(w, http.StatusForbidden, "not a member of this organization")
					return
				}
				httputil.WriteError(w, http.StatusInternalServerError, "failed to verify organization membership")
				return
			}

			ctx := auth.ContextWithOrg(r.Context(), orgID, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks if the caller has one of the allowed roles in the current
// org context. Returns the role string if allowed, or writes 403 and returns "".
func RequireRole(w http.ResponseWriter, r *http.Request, allowed ...string) string {
	role := auth.OrgRoleFromContext(r.Context())
	for _, a := range allowed {
		if role == a {
			return role
		}
	}
	httputil.WriteError(w, http.StatusForbidden, "insufficient permissions")
	return ""
}
