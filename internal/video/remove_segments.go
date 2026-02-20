package video

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
)

type removeSegmentsRequest struct {
	Segments []segmentRange `json:"segments"`
}

type segmentRange struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

const maxSegments = 200

func (h *Handler) RemoveSegments(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req removeSegmentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Segments) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "segments must not be empty")
		return
	}
	if len(req.Segments) > maxSegments {
		httputil.WriteError(w, http.StatusBadRequest, "too many segments (max 200)")
		return
	}

	for _, seg := range req.Segments {
		if seg.Start < 0 {
			httputil.WriteError(w, http.StatusBadRequest, "segment start must not be negative")
			return
		}
		if seg.End <= seg.Start {
			httputil.WriteError(w, http.StatusBadRequest, "segment end must be greater than start")
			return
		}
	}

	for i := 1; i < len(req.Segments); i++ {
		if req.Segments[i].Start < req.Segments[i-1].Start {
			httputil.WriteError(w, http.StatusBadRequest, "segments must be sorted by start time")
			return
		}
		if req.Segments[i].Start < req.Segments[i-1].End {
			httputil.WriteError(w, http.StatusBadRequest, "segments must not overlap")
			return
		}
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

	for _, seg := range req.Segments {
		if seg.End > float64(duration) {
			httputil.WriteError(w, http.StatusBadRequest, "segment end exceeds video duration")
			return
		}
	}

	var removedTime float64
	for _, seg := range req.Segments {
		removedTime += seg.End - seg.Start
	}
	resultDuration := float64(duration) - removedTime
	if resultDuration < 1.0 {
		httputil.WriteError(w, http.StatusBadRequest, "resulting video must be at least 1 second")
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
		RemoveSegmentsAsync(ctx, h.db, h.storage, videoID, fileKey, thumbnailFileKey(userID, shareToken), contentType, req.Segments, duration)
	}()

	w.WriteHeader(http.StatusAccepted)
}

func buildSegmentFilter(segments []segmentRange) string {
	parts := make([]string, len(segments))
	for i, seg := range segments {
		parts[i] = fmt.Sprintf("between(t,%.3f,%.3f)", seg.Start, seg.End)
	}
	return strings.Join(parts, "+")
}

func removeSegmentsFromVideo(inputPath, outputPath, contentType string, segments []segmentRange, audioPresent bool) error {
	betweenExpr := buildSegmentFilter(segments)

	var args []string
	args = append(args, "-i", inputPath)

	if audioPresent {
		filterComplex := fmt.Sprintf(
			"[0:v]select='not(%s)',setpts=N/FRAME_RATE/TB[v];[0:a]aselect='not(%s)',asetpts=N/SR/TB[a]",
			betweenExpr, betweenExpr,
		)
		args = append(args, "-filter_complex", filterComplex)
		args = append(args, "-map", "[v]", "-map", "[a]")
		args = append(args, "-c:v", videoCodecForContentType(contentType), "-c:a", "aac")
	} else {
		filterComplex := fmt.Sprintf(
			"[0:v]select='not(%s)',setpts=N/FRAME_RATE/TB[v]",
			betweenExpr,
		)
		args = append(args, "-filter_complex", filterComplex)
		args = append(args, "-map", "[v]")
		args = append(args, "-c:v", videoCodecForContentType(contentType), "-an")
	}

	args = append(args, "-y", outputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg remove segments: %w: %s", err, string(output))
	}
	return nil
}

func RemoveSegmentsAsync(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, thumbnailKey, contentType string, segments []segmentRange, originalDuration int) {
	log.Printf("remove-segments: starting for video %s (%d segments)", videoID, len(segments))

	setReadyFallback := func() {
		if _, err := db.Exec(ctx,
			`UPDATE videos SET status = 'ready', updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			log.Printf("remove-segments: failed to set fallback ready status for %s: %v", videoID, err)
		}
	}

	ext := extensionForContentType(contentType)
	tmpInput, err := os.CreateTemp("", "sendrec-remseg-input-*"+ext)
	if err != nil {
		log.Printf("remove-segments: failed to create temp input file: %v", err)
		setReadyFallback()
		return
	}
	tmpInputPath := tmpInput.Name()
	_ = tmpInput.Close()
	defer func() { _ = os.Remove(tmpInputPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpInputPath); err != nil {
		log.Printf("remove-segments: failed to download video %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	audioPresent := hasAudioStream(tmpInputPath)

	tmpOutput, err := os.CreateTemp("", "sendrec-remseg-output-*"+ext)
	if err != nil {
		log.Printf("remove-segments: failed to create temp output file: %v", err)
		setReadyFallback()
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	if err := removeSegmentsFromVideo(tmpInputPath, tmpOutputPath, contentType, segments, audioPresent); err != nil {
		log.Printf("remove-segments: ffmpeg failed for %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	if err := storage.UploadFile(ctx, fileKey, tmpOutputPath, contentType); err != nil {
		log.Printf("remove-segments: failed to upload processed video %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	var removedTime float64
	for _, seg := range segments {
		removedTime += seg.End - seg.Start
	}
	newDuration := int(float64(originalDuration) - removedTime)

	if _, err := db.Exec(ctx,
		`UPDATE videos SET status = 'ready', duration = $1, updated_at = now() WHERE id = $2`,
		newDuration, videoID,
	); err != nil {
		log.Printf("remove-segments: failed to update status for %s: %v", videoID, err)
		return
	}

	GenerateThumbnail(ctx, db, storage, videoID, fileKey, thumbnailKey)
	if err := EnqueueTranscription(ctx, db, videoID); err != nil {
		log.Printf("remove-segments: failed to enqueue transcription for %s: %v", videoID, err)
	}
	log.Printf("remove-segments: completed for video %s", videoID)
}
