package video

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
)

const maxThumbnailUploadBytes = 2 * 1024 * 1024 // 2MB

type thumbnailUploadResponse struct {
	UploadURL string `json:"uploadUrl"`
}

func (h *Handler) UploadThumbnail(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ContentType != "image/jpeg" && req.ContentType != "image/png" && req.ContentType != "image/webp" {
		httputil.WriteError(w, http.StatusBadRequest, "thumbnail must be JPEG, PNG, or WebP")
		return
	}
	if req.ContentLength <= 0 || req.ContentLength > maxThumbnailUploadBytes {
		httputil.WriteError(w, http.StatusBadRequest, "thumbnail must be 2MB or smaller")
		return
	}

	var shareToken string
	err := h.db.QueryRow(r.Context(),
		`SELECT share_token FROM videos WHERE id = $1 AND user_id = $2 AND organization_id IS NOT DISTINCT FROM $3 AND status = 'ready'`,
		videoID, userID, orgScope(r.Context()),
	).Scan(&shareToken)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	thumbKey := thumbnailFileKey(userID, shareToken)

	uploadURL, err := h.storage.GenerateUploadURL(r.Context(), thumbKey, req.ContentType, req.ContentLength, 15*time.Minute)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate upload URL")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		`UPDATE videos SET thumbnail_key = $1, updated_at = now() WHERE id = $2`,
		thumbKey, videoID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update thumbnail")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, thumbnailUploadResponse{UploadURL: uploadURL})
}

func (h *Handler) ResetThumbnail(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var shareToken, fileKey string
	var thumbKey *string
	err := h.db.QueryRow(r.Context(),
		`SELECT share_token, file_key, thumbnail_key FROM videos WHERE id = $1 AND user_id = $2 AND organization_id IS NOT DISTINCT FROM $3 AND status = 'ready'`,
		videoID, userID, orgScope(r.Context()),
	).Scan(&shareToken, &fileKey, &thumbKey)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	thumbnailKey := thumbnailFileKey(userID, shareToken)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	go func() {
		defer cancel()
		GenerateThumbnail(ctx, h.db, h.storage, videoID, fileKey, thumbnailKey)
	}()

	w.WriteHeader(http.StatusAccepted)
}

func thumbnailFileKey(userID, shareToken string) string {
	return fmt.Sprintf("recordings/%s/%s.jpg", userID, shareToken)
}

func extractFrameAt(inputPath, outputPath string, seekSeconds int) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ss", fmt.Sprintf("%d", seekSeconds),
		"-frames:v", "1",
		"-vf", "scale=640:-1",
		"-q:v", "5",
		"-y",
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, string(output))
	}
	return nil
}

func extractFrame(inputPath, outputPath string) error {
	return extractFrameAt(inputPath, outputPath, 2)
}

func GenerateThumbnail(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, thumbnailKey string) {
	tmpVideo, err := os.CreateTemp("", "sendrec-thumb-*.webm")
	if err != nil {
		slog.Error("thumbnail: failed to create temp video file", "error", err)
		return
	}
	tmpVideoPath := tmpVideo.Name()
	_ = tmpVideo.Close()
	defer func() { _ = os.Remove(tmpVideoPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpVideoPath); err != nil {
		slog.Error("thumbnail: failed to download video", "video_id", videoID, "error", err)
		return
	}

	tmpThumb, err := os.CreateTemp("", "sendrec-thumb-*.jpg")
	if err != nil {
		slog.Error("thumbnail: failed to create temp thumbnail file", "error", err)
		return
	}
	tmpThumbPath := tmpThumb.Name()
	_ = tmpThumb.Close()
	defer func() { _ = os.Remove(tmpThumbPath) }()

	if err := extractFrame(tmpVideoPath, tmpThumbPath); err != nil {
		slog.Error("thumbnail: ffmpeg failed", "video_id", videoID, "error", err)
		return
	}

	// If -ss 2 produced a 0-byte file (video shorter than 2s), retry at the start
	if info, err := os.Stat(tmpThumbPath); err == nil && info.Size() == 0 {
		slog.Warn("thumbnail: video too short for seek=2, retrying at seek=0", "video_id", videoID)
		if err := extractFrameAt(tmpVideoPath, tmpThumbPath, 0); err != nil {
			slog.Error("thumbnail: ffmpeg retry failed", "video_id", videoID, "error", err)
			return
		}
	}

	// Skip upload if thumbnail is still empty
	if info, err := os.Stat(tmpThumbPath); err != nil || info.Size() == 0 {
		slog.Warn("thumbnail: no frame extracted, skipping", "video_id", videoID)
		return
	}

	if err := storage.UploadFile(ctx, thumbnailKey, tmpThumbPath, "image/jpeg"); err != nil {
		slog.Error("thumbnail: failed to upload", "video_id", videoID, "error", err)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET thumbnail_key = $1, updated_at = now() WHERE id = $2`,
		thumbnailKey, videoID,
	); err != nil {
		slog.Error("thumbnail: failed to update thumbnail_key", "video_id", videoID, "error", err)
	}
}
