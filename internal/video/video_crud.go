package video

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/validate"
	"github.com/sendrec/sendrec/internal/webhook"
)

type createRequest struct {
	Title             string `json:"title"`
	Duration          int    `json:"duration"`
	FileSize          int64  `json:"fileSize"`
	WebcamFileSize    int64  `json:"webcamFileSize,omitempty"`
	ContentType       string `json:"contentType,omitempty"`
	WebcamContentType string `json:"webcamContentType,omitempty"`
}

type createResponse struct {
	ID              string `json:"id"`
	UploadURL       string `json:"uploadUrl"`
	ShareToken      string `json:"shareToken"`
	WebcamUploadURL string `json:"webcamUploadUrl,omitempty"`
}

type updateRequest struct {
	Status string `json:"status"`
	Title  string `json:"title"`
}

type uploadRequest struct {
	Title       string `json:"title"`
	FileSize    int64  `json:"fileSize"`
	ContentType string `json:"contentType"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	plan, _ := h.getUserPlan(r.Context(), userID)
	maxVideos := h.maxVideosPerMonth
	maxDuration := h.maxVideoDurationSeconds
	if plan == "pro" || plan == "business" {
		maxVideos = 0
		maxDuration = 0
	}

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FileSize <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "fileSize must be positive")
		return
	}

	if req.Duration < 1 {
		httputil.WriteError(w, http.StatusBadRequest, "video duration must be at least 1 second")
		return
	}

	if h.maxUploadBytes > 0 && req.FileSize > h.maxUploadBytes {
		httputil.WriteError(w, http.StatusBadRequest, "file too large")
		return
	}

	if maxDuration > 0 && req.Duration > maxDuration {
		httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("video duration exceeds the maximum of %d minutes", maxDuration/60))
		return
	}

	if maxVideos > 0 {
		count, err := h.countVideosThisMonth(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to check video limit")
			return
		}
		if count >= maxVideos {
			httputil.WriteError(w, http.StatusForbidden, fmt.Sprintf("monthly video limit of %d reached", maxVideos))
			return
		}
	}

	title := req.Title
	if title == "" {
		title = "Untitled Recording"
	}
	if msg := validate.Title(title); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "video/webm"
	}
	if contentType != "video/webm" && contentType != "video/mp4" {
		httputil.WriteError(w, http.StatusBadRequest, "only video/webm and video/mp4 recordings are supported")
		return
	}

	shareToken, err := generateShareToken()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate share token")
		return
	}

	fileKey := videoFileKey(userID, shareToken, contentType)

	webcamContentType := req.WebcamContentType
	if webcamContentType == "" {
		webcamContentType = "video/webm"
	}

	var webcamKey *string
	if req.WebcamFileSize > 0 {
		k := webcamFileKey(userID, shareToken, webcamContentType)
		webcamKey = &k
	}

	var videoID string
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO videos (user_id, title, duration, file_size, file_key, share_token, webcam_key, content_type)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		userID, title, req.Duration, req.FileSize, fileKey, shareToken, webcamKey, contentType,
	).Scan(&videoID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create video")
		return
	}

	h.dispatchWebhook(userID, webhook.Event{
		Name:      "video.created",
		Timestamp: time.Now().UTC(),
		Data: map[string]any{
			"videoId":   videoID,
			"title":     title,
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		},
	})

	uploadURL, err := h.storage.GenerateUploadURL(r.Context(), fileKey, contentType, req.FileSize, 30*time.Minute)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate upload URL")
		return
	}

	resp := createResponse{
		ID:         videoID,
		UploadURL:  uploadURL,
		ShareToken: shareToken,
	}

	if webcamKey != nil {
		webcamURL, err := h.storage.GenerateUploadURL(r.Context(), *webcamKey, webcamContentType, req.WebcamFileSize, 30*time.Minute)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to generate webcam upload URL")
			return
		}
		resp.WebcamUploadURL = webcamURL
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	plan, _ := h.getUserPlan(r.Context(), userID)
	maxVideos := h.maxVideosPerMonth
	if plan == "pro" || plan == "business" {
		maxVideos = 0
	}

	var req uploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ContentType != "video/mp4" && req.ContentType != "video/webm" && req.ContentType != "video/quicktime" {
		httputil.WriteError(w, http.StatusBadRequest, "only video/mp4, video/webm, and video/quicktime uploads are supported")
		return
	}

	if req.FileSize <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "fileSize must be positive")
		return
	}

	if h.maxUploadBytes > 0 && req.FileSize > h.maxUploadBytes {
		httputil.WriteError(w, http.StatusBadRequest, "file too large")
		return
	}

	if maxVideos > 0 {
		count, err := h.countVideosThisMonth(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to check video limit")
			return
		}
		if count >= maxVideos {
			httputil.WriteError(w, http.StatusForbidden, fmt.Sprintf("monthly video limit of %d reached", maxVideos))
			return
		}
	}

	title := req.Title
	if title == "" {
		title = "Untitled Video"
	}
	if msg := validate.Title(title); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	shareToken, err := generateShareToken()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate share token")
		return
	}

	fileKey := videoFileKey(userID, shareToken, req.ContentType)

	var videoID string
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO videos (user_id, title, duration, file_size, file_key, share_token, content_type)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		userID, title, 0, req.FileSize, fileKey, shareToken, req.ContentType,
	).Scan(&videoID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create video")
		return
	}

	uploadURL, err := h.storage.GenerateUploadURL(r.Context(), fileKey, req.ContentType, req.FileSize, 30*time.Minute)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate upload URL")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, createResponse{
		ID:         videoID,
		UploadURL:  uploadURL,
		ShareToken: shareToken,
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status != "" && req.Status != "ready" {
		httputil.WriteError(w, http.StatusBadRequest, "status can only be set to ready")
		return
	}

	if req.Status == "" && req.Title == "" {
		httputil.WriteError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if req.Status == "ready" {
		var fileKey string
		var fileSize int64
		var shareToken string
		var webcamKey *string
		var expectedContentType string
		var duration int
		err := h.db.QueryRow(r.Context(),
			`SELECT file_key, file_size, share_token, webcam_key, content_type, duration FROM videos
			 WHERE id = $1 AND user_id = $2 AND status = 'uploading'`,
			videoID, userID,
		).Scan(&fileKey, &fileSize, &shareToken, &webcamKey, &expectedContentType, &duration)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}

		size, contentType, err := h.storage.HeadObject(r.Context(), fileKey)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "could not verify upload")
			return
		}
		if size <= 0 || (h.maxUploadBytes > 0 && size > h.maxUploadBytes) {
			httputil.WriteError(w, http.StatusBadRequest, "uploaded file invalid size")
			return
		}
		if fileSize > 0 && size != fileSize {
			httputil.WriteError(w, http.StatusBadRequest, "uploaded file size mismatch")
			return
		}
		if contentType != expectedContentType {
			httputil.WriteError(w, http.StatusBadRequest, "uploaded file invalid type")
			return
		}

		newStatus := "ready"
		if webcamKey != nil {
			newStatus = "processing"
		}

		tag, err := h.db.Exec(r.Context(),
			`UPDATE videos SET status = $1, updated_at = now()
			 WHERE id = $2 AND user_id = $3 AND status = 'uploading'`,
			newStatus, videoID, userID,
		)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update video")
			return
		}
		if tag.RowsAffected() == 0 {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}

		h.dispatchWebhook(userID, webhook.Event{
			Name:      "video.ready",
			Timestamp: time.Now().UTC(),
			Data: map[string]any{
				"videoId":    videoID,
				"duration":   duration,
				"shareToken": shareToken,
				"watchUrl":   h.baseURL + "/watch/" + shareToken,
			},
		})

		audioFilter := ""
		if h.noiseReductionFilter != "" {
			var noiseReduction bool
			if err := h.db.QueryRow(r.Context(),
				"SELECT noise_reduction FROM users WHERE id = $1", userID,
			).Scan(&noiseReduction); err == nil && noiseReduction {
				audioFilter = h.noiseReductionFilter
			}
		}

		if audioFilter != "" {
			if _, err := h.db.Exec(r.Context(),
				"UPDATE videos SET noise_reduction = true WHERE id = $1", videoID,
			); err != nil {
				slog.Error("failed to set noise_reduction on video", "video_id", videoID, "error", err)
			}
		}

		if webcamKey != nil {
			h.EnqueueJob(r.Context(), JobTypeComposite, videoID, map[string]any{
				"fileKey":      fileKey,
				"webcamKey":    *webcamKey,
				"thumbnailKey": thumbnailFileKey(userID, shareToken),
				"contentType":  expectedContentType,
			})
		} else {
			h.EnqueueJob(r.Context(), JobTypeThumbnail, videoID, map[string]any{
				"fileKey":      fileKey,
				"thumbnailKey": thumbnailFileKey(userID, shareToken),
			})
			h.EnqueueJob(r.Context(), JobTypeTranscribe, videoID, nil)

			if duration == 0 {
				h.EnqueueJob(r.Context(), JobTypeProbe, videoID, map[string]any{"fileKey": fileKey})
			}
			if expectedContentType == "video/webm" {
				h.EnqueueJob(r.Context(), JobTypeTranscode, videoID, map[string]any{
					"fileKey":     fileKey,
					"audioFilter": audioFilter,
				})
			}
			if expectedContentType == "video/mp4" || expectedContentType == "video/quicktime" {
				h.EnqueueJob(r.Context(), JobTypeNormalize, videoID, map[string]any{
					"fileKey":     fileKey,
					"audioFilter": audioFilter,
				})
			}
		}
	}

	if req.Title != "" {
		if msg := validate.Title(req.Title); msg != "" {
			httputil.WriteError(w, http.StatusBadRequest, msg)
			return
		}
		tag, err := h.db.Exec(r.Context(),
			`UPDATE videos SET title = $1, updated_at = now()
			 WHERE id = $2 AND user_id = $3 AND status != 'deleted'`,
			req.Title, videoID, userID,
		)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update video")
			return
		}
		if tag.RowsAffected() == 0 {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

type trimRequest struct {
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
}

func (h *Handler) Trim(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req trimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.StartSeconds < 0 {
		httputil.WriteError(w, http.StatusBadRequest, "startSeconds must not be negative")
		return
	}
	if req.EndSeconds <= req.StartSeconds {
		httputil.WriteError(w, http.StatusBadRequest, "endSeconds must be greater than startSeconds")
		return
	}

	var duration int
	var fileKey string
	var shareToken string
	var status string
	var contentType string
	err := h.db.QueryRow(r.Context(),
		`SELECT duration, file_key, share_token, status, content_type FROM videos WHERE id = $1 AND user_id = $2`,
		videoID, userID,
	).Scan(&duration, &fileKey, &shareToken, &status, &contentType)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}
	if status != "ready" {
		httputil.WriteError(w, http.StatusConflict, "video is currently being processed")
		return
	}

	if req.EndSeconds > float64(duration) {
		httputil.WriteError(w, http.StatusBadRequest, "endSeconds exceeds video duration")
		return
	}
	if req.EndSeconds-req.StartSeconds < 1.0 {
		httputil.WriteError(w, http.StatusBadRequest, "trimmed video must be at least 1 second")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET status = 'processing', updated_at = now() WHERE id = $1 AND user_id = $2 AND status = 'ready'`,
		videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update video status")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusConflict, "video is already being processed")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		TrimVideoAsync(ctx, h.db, h.storage, videoID, fileKey, thumbnailFileKey(userID, shareToken), contentType, req.StartSeconds, req.EndSeconds)
	}()

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var fileKey string
	var thumbnailKey *string
	var webcamKey *string
	var transcriptKey *string
	var title string
	err := h.db.QueryRow(r.Context(),
		`UPDATE videos SET status = 'deleted', updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status != 'deleted'
		 RETURNING file_key, thumbnail_key, webcam_key, transcript_key, title`,
		videoID, userID,
	).Scan(&fileKey, &thumbnailKey, &webcamKey, &transcriptKey, &title)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	h.dispatchWebhook(userID, webhook.Event{
		Name:      "video.deleted",
		Timestamp: time.Now().UTC(),
		Data: map[string]any{
			"videoId": videoID,
			"title":   title,
		},
	})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := deleteWithRetry(ctx, h.storage, fileKey, 3); err != nil {
			slog.Error("video: all delete retries failed", "key", fileKey, "error", err)
			return
		}
		if thumbnailKey != nil {
			if err := deleteWithRetry(ctx, h.storage, *thumbnailKey, 3); err != nil {
				slog.Error("video: thumbnail delete failed", "key", *thumbnailKey, "error", err)
			}
		}
		if webcamKey != nil {
			if err := deleteWithRetry(ctx, h.storage, *webcamKey, 3); err != nil {
				slog.Error("video: webcam delete failed", "key", *webcamKey, "error", err)
			}
		}
		if transcriptKey != nil {
			if err := deleteWithRetry(ctx, h.storage, *transcriptKey, 3); err != nil {
				slog.Error("video: transcript delete failed", "key", *transcriptKey, "error", err)
			}
		}
		if _, err := h.db.Exec(ctx,
			`UPDATE videos SET file_purged_at = now() WHERE file_key = $1`,
			fileKey,
		); err != nil {
			slog.Error("video: failed to mark file_purged_at", "key", fileKey, "error", err)
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}
