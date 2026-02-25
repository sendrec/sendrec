package video

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var title string
	var fileKey string
	var contentType string
	err := h.db.QueryRow(r.Context(),
		`SELECT title, file_key, content_type FROM videos WHERE id = $1 AND user_id = $2 AND status = 'ready'`,
		videoID, userID,
	).Scan(&title, &fileKey, &contentType)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	filename := title + extensionForContentType(contentType)
	downloadURL, err := h.storage.GenerateDownloadURLWithDisposition(r.Context(), fileKey, filename, 1*time.Hour)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate download URL")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"downloadUrl": downloadURL})
}

func (h *Handler) WatchDownload(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var title string
	var fileKey string
	var shareExpiresAt *time.Time
	var sharePassword *string
	var contentType string
	var downloadEnabled bool

	err := h.db.QueryRow(r.Context(),
		`SELECT title, file_key, share_expires_at, share_password, content_type, download_enabled FROM videos WHERE share_token = $1 AND status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&title, &fileKey, &shareExpiresAt, &sharePassword, &contentType, &downloadEnabled)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if !downloadEnabled {
		httputil.WriteError(w, http.StatusForbidden, "downloads are disabled for this video")
		return
	}

	if shareExpiresAt != nil && time.Now().After(*shareExpiresAt) {
		httputil.WriteError(w, http.StatusGone, "link expired")
		return
	}

	if sharePassword != nil {
		if !hasValidWatchCookie(r, h.hmacSecret, shareToken, *sharePassword) {
			httputil.WriteError(w, http.StatusForbidden, "password required")
			return
		}
	}

	filename := title + extensionForContentType(contentType)
	downloadURL, err := h.storage.GenerateDownloadURLWithDisposition(r.Context(), fileKey, filename, 1*time.Hour)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate download URL")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"downloadUrl": downloadURL})
}

func (h *Handler) Summarize(w http.ResponseWriter, r *http.Request) {
	if !h.aiEnabled {
		httputil.WriteError(w, http.StatusForbidden, "AI summaries not enabled")
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET summary_status = 'pending', summary = NULL, chapters = NULL, updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status != 'deleted' AND transcript_status = 'ready'`,
		videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not enqueue summary")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found or transcript not ready")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GenerateDocument(w http.ResponseWriter, r *http.Request) {
	if !h.aiEnabled {
		httputil.WriteError(w, http.StatusForbidden, "AI features not enabled")
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET document_status = 'pending', document = NULL, updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status != 'deleted' AND transcript_status = 'ready'`,
		videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not enqueue document generation")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found or transcript not ready")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DismissTitle(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET suggested_title = NULL, updated_at = now() WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to dismiss title")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
