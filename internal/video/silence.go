package video

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

type detectSilenceRequest struct {
	NoiseDB     *int     `json:"noiseDB"`
	MinDuration *float64 `json:"minDuration"`
}

type detectSilenceResponse struct {
	Segments []segmentRange `json:"segments"`
}

func (h *Handler) DetectSilence(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req detectSilenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	noiseDB := -30
	if req.NoiseDB != nil {
		noiseDB = *req.NoiseDB
	}
	minDuration := 1.0
	if req.MinDuration != nil {
		minDuration = *req.MinDuration
	}

	if noiseDB < -90 || noiseDB > 0 {
		httputil.WriteError(w, http.StatusBadRequest, "noiseDB must be between -90 and 0")
		return
	}
	if minDuration < 0.5 || minDuration > 10.0 {
		httputil.WriteError(w, http.StatusBadRequest, "minDuration must be between 0.5 and 10.0")
		return
	}

	var duration int
	var fileKey string
	var status string
	var contentType string
	err := h.db.QueryRow(r.Context(),
		`SELECT duration, file_key, status, content_type FROM videos WHERE id = $1 AND user_id = $2`,
		videoID, userID,
	).Scan(&duration, &fileKey, &status, &contentType)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}
	if status != "ready" {
		httputil.WriteError(w, http.StatusConflict, "video is currently being processed")
		return
	}

	ext := extensionForContentType(contentType)
	tmpFile, err := os.CreateTemp("", "sendrec-silence-*"+ext)
	if err != nil {
		slog.Error("detect-silence: failed to create temp file", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to detect silence")
		return
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := h.storage.DownloadToFile(r.Context(), fileKey, tmpPath); err != nil {
		slog.Error("detect-silence: failed to download video", "video_id", videoID, "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to detect silence")
		return
	}

	segments, err := detectSilence(tmpPath, noiseDB, minDuration)
	if err != nil {
		slog.Error("detect-silence: ffmpeg failed", "video_id", videoID, "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to detect silence")
		return
	}

	resp := detectSilenceResponse{Segments: segments}
	if resp.Segments == nil {
		resp.Segments = []segmentRange{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

var (
	silenceStartRegex = regexp.MustCompile(`silence_start:\s*([\d.]+)`)
	silenceEndRegex   = regexp.MustCompile(`silence_end:\s*([\d.]+)`)
)

func parseSilenceDetectOutput(stderr string) []segmentRange {
	var segments []segmentRange
	var pendingStart float64
	hasPendingStart := false

	for _, line := range strings.Split(stderr, "\n") {
		if match := silenceStartRegex.FindStringSubmatch(line); match != nil {
			start, err := strconv.ParseFloat(match[1], 64)
			if err != nil {
				continue
			}
			pendingStart = start
			hasPendingStart = true
			continue
		}

		if match := silenceEndRegex.FindStringSubmatch(line); match != nil && hasPendingStart {
			end, err := strconv.ParseFloat(match[1], 64)
			if err != nil {
				continue
			}
			segments = append(segments, segmentRange{Start: pendingStart, End: end})
			hasPendingStart = false
		}
	}

	return segments
}

func detectSilence(inputPath string, noiseDB int, minDuration float64) ([]segmentRange, error) {
	filterValue := fmt.Sprintf("silencedetect=noise=%ddB:d=%.2f", noiseDB, minDuration)
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-af", filterValue,
		"-f", "null",
		"-",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg silencedetect: %w: %s", err, string(output))
	}

	return parseSilenceDetectOutput(string(output)), nil
}
