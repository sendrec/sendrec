package video

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/languages"
)

type setDownloadEnabledRequest struct {
	DownloadEnabled bool `json:"downloadEnabled"`
}

type setEmailGateRequest struct {
	Enabled bool `json:"enabled"`
}

type setLinkExpiryRequest struct {
	NeverExpires bool `json:"neverExpires"`
}

type setCTARequest struct {
	Text *string `json:"text"`
	URL  *string `json:"url"`
}

type setPasswordRequest struct {
	Password string `json:"password"`
}

type retranscribeRequest struct {
	Language string `json:"language"`
}

func (h *Handler) SetDownloadEnabled(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setDownloadEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET download_enabled = $1 WHERE id = $2 AND user_id = $3 AND status != 'deleted'`,
		req.DownloadEnabled, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not update download setting")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetEmailGate(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setEmailGateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET email_gate_enabled = $1 WHERE id = $2 AND user_id = $3 AND status != 'deleted'`,
		req.Enabled, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not update email gate setting")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetLinkExpiry(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setLinkExpiryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var query string
	if req.NeverExpires {
		query = `UPDATE videos SET share_expires_at = NULL, updated_at = now() WHERE id = $1 AND user_id = $2 AND status != 'deleted'`
	} else {
		query = `UPDATE videos SET share_expires_at = now() + INTERVAL '7 days', updated_at = now() WHERE id = $1 AND user_id = $2 AND status != 'deleted'`
	}

	tag, err := h.db.Exec(r.Context(), query, videoID, userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not update link expiry")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetCTA(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setCTARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Text != nil && req.URL != nil {
		if len(*req.Text) > 100 {
			httputil.WriteError(w, http.StatusBadRequest, "CTA text must be 100 characters or less")
			return
		}
		if len(*req.URL) > 2000 {
			httputil.WriteError(w, http.StatusBadRequest, "CTA URL must be 2000 characters or less")
			return
		}
		if !strings.HasPrefix(*req.URL, "http://") && !strings.HasPrefix(*req.URL, "https://") {
			httputil.WriteError(w, http.StatusBadRequest, "CTA URL must start with http:// or https://")
			return
		}
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET cta_text = $1, cta_url = $2 WHERE id = $3 AND user_id = $4 AND status != 'deleted'`,
		req.Text, req.URL, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not update CTA")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Password) > 128 {
		httputil.WriteError(w, http.StatusBadRequest, "password is too long")
		return
	}

	if req.Password == "" {
		tag, err := h.db.Exec(r.Context(),
			`UPDATE videos SET share_password = NULL, updated_at = now() WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
			videoID, userID,
		)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update password")
			return
		}
		if tag.RowsAffected() == 0 {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	hash, err := hashSharePassword(req.Password)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET share_password = $1, updated_at = now() WHERE id = $2 AND user_id = $3 AND status != 'deleted'`,
		hash, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update password")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Retranscribe(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req retranscribeRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	var exists bool
	err := h.db.QueryRow(r.Context(),
		`SELECT true FROM videos
		 WHERE id = $1 AND user_id = $2 AND status = 'ready'`,
		videoID, userID,
	).Scan(&exists)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if req.Language != "" {
		if !languages.IsValidTranscriptionLanguage(req.Language) {
			httputil.WriteError(w, http.StatusBadRequest, "invalid transcription language")
			return
		}
		if _, err := h.db.Exec(r.Context(),
			`UPDATE videos SET transcription_language = $1, updated_at = now() WHERE id = $2 AND user_id = $3`,
			req.Language, videoID, userID,
		); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to set language")
			return
		}
	}

	if err := EnqueueTranscription(r.Context(), h.db, videoID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to enqueue transcription")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) Extend(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var shareExpiresAt *time.Time
	err := h.db.QueryRow(r.Context(),
		`SELECT share_expires_at FROM videos WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	).Scan(&shareExpiresAt)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if shareExpiresAt == nil {
		httputil.WriteError(w, http.StatusBadRequest, "link does not expire")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET share_expires_at = now() + INTERVAL '7 days', updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to extend share link")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
