package video

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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
	videoID := chi.URLParam(r, "id")

	var req setDownloadEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	where, args := orgVideoFilter(r.Context(), videoID, []any{req.DownloadEnabled}, "AND status != 'deleted'")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET download_enabled = $1 WHERE `+where, args...,
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
	videoID := chi.URLParam(r, "id")

	var req setEmailGateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	where, args := orgVideoFilter(r.Context(), videoID, []any{req.Enabled}, "AND status != 'deleted'")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET email_gate_enabled = $1 WHERE `+where, args...,
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
	videoID := chi.URLParam(r, "id")

	var req setLinkExpiryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status != 'deleted'")
	var query string
	if req.NeverExpires {
		query = `UPDATE videos SET share_expires_at = NULL, updated_at = now() WHERE ` + where
	} else {
		query = `UPDATE videos SET share_expires_at = now() + INTERVAL '7 days', updated_at = now() WHERE ` + where
	}

	tag, err := h.db.Exec(r.Context(), query, args...)
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

	where, args := orgVideoFilter(r.Context(), videoID, []any{req.Text, req.URL}, "AND status != 'deleted'")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET cta_text = $1, cta_url = $2 WHERE `+where, args...,
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
		where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status != 'deleted'")
		tag, err := h.db.Exec(r.Context(),
			`UPDATE videos SET share_password = NULL, updated_at = now() WHERE `+where, args...,
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

	where, args := orgVideoFilter(r.Context(), videoID, []any{hash}, "AND status != 'deleted'")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET share_password = $1, updated_at = now() WHERE `+where, args...,
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
	videoID := chi.URLParam(r, "id")

	var req retranscribeRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status = 'ready'")
	var exists bool
	err := h.db.QueryRow(r.Context(),
		`SELECT true FROM videos WHERE `+where, args...,
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
		langWhere, langArgs := orgVideoFilter(r.Context(), videoID, []any{req.Language}, "")
		if _, err := h.db.Exec(r.Context(),
			`UPDATE videos SET transcription_language = $1, updated_at = now() WHERE `+langWhere, langArgs...,
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
	videoID := chi.URLParam(r, "id")

	where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status != 'deleted'")
	var shareExpiresAt *time.Time
	err := h.db.QueryRow(r.Context(),
		`SELECT share_expires_at FROM videos WHERE `+where, args...,
	).Scan(&shareExpiresAt)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if shareExpiresAt == nil {
		httputil.WriteError(w, http.StatusBadRequest, "link does not expire")
		return
	}

	_, updateArgs := orgVideoFilter(r.Context(), videoID, nil, "AND status != 'deleted'")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET share_expires_at = now() + INTERVAL '7 days', updated_at = now()
		 WHERE `+where, updateArgs...,
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

func (h *Handler) Summarize(w http.ResponseWriter, r *http.Request) {
	if !h.aiEnabled {
		httputil.WriteError(w, http.StatusForbidden, "AI summaries not enabled")
		return
	}

	videoID := chi.URLParam(r, "id")

	where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status != 'deleted' AND transcript_status = 'ready'")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET summary_status = 'pending', summary = NULL, chapters = NULL, updated_at = now()
		 WHERE `+where, args...,
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

	videoID := chi.URLParam(r, "id")

	where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status != 'deleted' AND transcript_status = 'ready'")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET document_status = 'pending', document = NULL, updated_at = now()
		 WHERE `+where, args...,
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
	videoID := chi.URLParam(r, "id")
	where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status != 'deleted'")
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET suggested_title = NULL, updated_at = now() WHERE `+where, args...,
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
