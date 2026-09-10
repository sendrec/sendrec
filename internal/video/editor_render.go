package video

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

const editTimelineVersion = 1

type editClip struct {
	ID          string  `json:"id"`
	SourceID    string  `json:"sourceId"`
	SourceStart float64 `json:"sourceStart"`
	SourceEnd   float64 `json:"sourceEnd"`
	Duration    float64 `json:"duration"`
}

type editorCoverOverlay struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
}

type editTimeline struct {
	Version  int                  `json:"version"`
	Clips    []editClip           `json:"clips"`
	Overlays []editorCoverOverlay `json:"overlays,omitempty"`
}

type editorStateResponse struct {
	Timeline      editTimeline `json:"timeline"`
	RenderStatus  string       `json:"renderStatus"`
	RenderError   *string      `json:"renderError"`
	RenderedVideo *string      `json:"renderedVideoId"`
}

type sourceVideo struct {
	ID          string
	FileKey     string
	ContentType string
	Duration    int
	HasAudio    bool
}

type renderJob struct {
	SourceVideoID  string
	OwnerID        string
	OrganizationID *string
	Title          string
	Timeline       editTimeline
	Sources        map[string]sourceVideo
}

var enqueueTimelineRender = func(h *Handler, job renderJob) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		h.renderTimelineAsync(ctx, job)
	}()
}

func validateEditTimeline(timeline *editTimeline) error {
	if timeline.Version == 0 {
		timeline.Version = editTimelineVersion
	}
	if timeline.Version != editTimelineVersion {
		return fmt.Errorf("unsupported timeline version")
	}
	if len(timeline.Clips) == 0 {
		return fmt.Errorf("timeline must contain at least one clip")
	}
	if len(timeline.Clips) > 500 {
		return fmt.Errorf("timeline contains too many clips")
	}
	seen := make(map[string]struct{}, len(timeline.Clips))
	totalDuration := 0.0
	for i := range timeline.Clips {
		clip := &timeline.Clips[i]
		if strings.TrimSpace(clip.ID) == "" || strings.TrimSpace(clip.SourceID) == "" {
			return fmt.Errorf("clip id and sourceId are required")
		}
		if _, exists := seen[clip.ID]; exists {
			return fmt.Errorf("clip ids must be unique")
		}
		seen[clip.ID] = struct{}{}
		if clip.SourceStart < 0 || clip.SourceEnd <= clip.SourceStart {
			return fmt.Errorf("clip sourceEnd must be greater than sourceStart")
		}
		clip.Duration = clip.SourceEnd - clip.SourceStart
		totalDuration += clip.Duration
	}
	if totalDuration < 1 {
		return fmt.Errorf("timeline must be at least one second long")
	}

	if len(timeline.Overlays) > 500 {
		return fmt.Errorf("timeline contains too many overlays")
	}

	overlayIDs := make(map[string]struct{}, len(timeline.Overlays))
	for i := range timeline.Overlays {
		overlay := &timeline.Overlays[i]

		if strings.TrimSpace(overlay.ID) == "" {
			return fmt.Errorf("overlay id is required")
		}
		if _, exists := overlayIDs[overlay.ID]; exists {
			return fmt.Errorf("overlay ids must be unique")
		}
		overlayIDs[overlay.ID] = struct{}{}

		if overlay.X < 0 || overlay.Y < 0 ||
			overlay.Width <= 0 || overlay.Height <= 0 {
			return fmt.Errorf("overlay position and size are invalid")
		}

		if overlay.X+overlay.Width > 100.001 ||
			overlay.Y+overlay.Height > 100.001 {
			return fmt.Errorf("overlay must stay inside video bounds")
		}

		if overlay.Start < 0 || overlay.End <= overlay.Start {
			return fmt.Errorf("overlay end must be greater than start")
		}

		if overlay.End > totalDuration+0.001 {
			return fmt.Errorf("overlay exceeds timeline duration")
		}
	}

	return nil
}

func (h *Handler) GetEditorState(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "id")
	where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status != 'deleted'")
	var raw []byte
	var status string
	var renderError, renderedVideoID *string
	err := h.db.QueryRow(r.Context(), `SELECT COALESCE(edit_timeline, 'null'::jsonb), edit_render_status,
		edit_render_error, edit_render_video_id::text FROM videos WHERE `+where, args...).
		Scan(&raw, &status, &renderError, &renderedVideoID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	var timeline editTimeline
	if string(raw) != "null" {
		if err := json.Unmarshal(raw, &timeline); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "stored timeline is invalid")
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, editorStateResponse{
		Timeline: timeline, RenderStatus: status, RenderError: renderError, RenderedVideo: renderedVideoID,
	})
}

func (h *Handler) RenderEditorTimeline(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "id")
	var timeline editTimeline
	if err := json.NewDecoder(r.Body).Decode(&timeline); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateEditTimeline(&timeline); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	where, args := orgVideoFilter(r.Context(), videoID, nil, "AND status = 'ready'")
	var job renderJob
	job.SourceVideoID = videoID
	err := h.db.QueryRow(r.Context(), `SELECT user_id, organization_id, title FROM videos WHERE `+where, args...).
		Scan(&job.OwnerID, &job.OrganizationID, &job.Title)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found or not ready")
		return
	}

	job.Timeline = timeline
	job.Sources = make(map[string]sourceVideo)
	for _, clip := range timeline.Clips {
		if _, ok := job.Sources[clip.SourceID]; ok {
			continue
		}
		sourceWhere, sourceArgs := orgVideoFilter(r.Context(), clip.SourceID, nil, "AND status = 'ready'")
		var source sourceVideo
		source.ID = clip.SourceID
		if err := h.db.QueryRow(r.Context(), `SELECT file_key, content_type, duration FROM videos WHERE `+sourceWhere, sourceArgs...).
			Scan(&source.FileKey, &source.ContentType, &source.Duration); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "timeline contains an unavailable source")
			return
		}
		job.Sources[source.ID] = source
	}
	for _, clip := range timeline.Clips {
		if clip.SourceEnd > float64(job.Sources[clip.SourceID].Duration)+0.001 {
			httputil.WriteError(w, http.StatusBadRequest, "clip exceeds source duration")
			return
		}
	}

	raw, err := json.Marshal(timeline)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save timeline")
		return
	}
	updateWhere, updateArgs := orgVideoFilter(r.Context(), videoID, []any{json.RawMessage(raw)}, "AND edit_render_status != 'processing'")
	tag, err := h.db.Exec(r.Context(), `UPDATE videos SET edit_timeline = $1, edit_render_status = 'processing',
		edit_render_error = NULL, edit_render_video_id = NULL, updated_at = now() WHERE `+updateWhere, updateArgs...)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save timeline")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusConflict, "timeline is already being rendered")
		return
	}

	enqueueTimelineRender(h, job)
	w.WriteHeader(http.StatusAccepted)
}

func buildTimelineRenderArgs(inputs []string, clips []editClip, overlays []editorCoverOverlay, sourceIndexes map[string]int, sources map[string]sourceVideo, output string) []string {
	args := make([]string, 0, len(inputs)*2+len(clips)*2+16)
	for _, input := range inputs {
		args = append(args, "-i", input)
	}
	var filters []string
	var concatInputs strings.Builder
	for i, clip := range clips {
		inputIndex := sourceIndexes[clip.SourceID]
		filters = append(filters, fmt.Sprintf(
			"[%d:v]trim=start=%.3f:end=%.3f,setpts=PTS-STARTPTS,scale=1920:1080:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,fps=30,format=yuv420p[v%d]",
			inputIndex, clip.SourceStart, clip.SourceEnd, i))
		if sources[clip.SourceID].HasAudio {
			filters = append(filters, fmt.Sprintf("[%d:a]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS,aresample=48000[a%d]", inputIndex, clip.SourceStart, clip.SourceEnd, i))
		} else {
			filters = append(filters, fmt.Sprintf("anullsrc=r=48000:cl=stereo,atrim=duration=%.3f,asetpts=PTS-STARTPTS[a%d]", clip.Duration, i))
		}
		fmt.Fprintf(&concatInputs, "[v%d][a%d]", i, i)
	}
	if len(overlays) == 0 {
		filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=1:a=1[vout][aout]", concatInputs.String(), len(clips)))
	} else {
		filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=1:a=1[vbase][aout]", concatInputs.String(), len(clips)))

		previousLabel := "vbase"
		for i, overlay := range overlays {
			nextLabel := fmt.Sprintf("voverlay%d", i)
			if i == len(overlays)-1 {
				nextLabel = "vout"
			}

			filters = append(filters, fmt.Sprintf(
				"[%s]drawbox=x=iw*%.6f:y=ih*%.6f:w=iw*%.6f:h=ih*%.6f:color=black:t=fill:enable='between(t,%.3f,%.3f)'[%s]",
				previousLabel,
				overlay.X/100,
				overlay.Y/100,
				overlay.Width/100,
				overlay.Height/100,
				overlay.Start,
				overlay.End,
				nextLabel,
			))

			previousLabel = nextLabel
		}
	}
	args = append(args, "-filter_complex", strings.Join(filters, ";"), "-map", "[vout]", "-map", "[aout]",
		"-c:v", "libx264", "-profile:v", "high", "-level:v", "5.1", "-preset", "fast", "-crf", "23",
		"-c:a", "aac", "-movflags", "+faststart", "-y", output)
	return args
}

func probeHasAudio(path string) bool {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=index", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func (h *Handler) renderTimelineAsync(ctx context.Context, job renderJob) {
	fail := func(err error) {
		msg := err.Error()
		if len(msg) > 1000 {
			msg = msg[:1000]
		}
		if _, dbErr := h.db.Exec(ctx, `UPDATE videos SET edit_render_status = 'failed', edit_render_error = $1,
			updated_at = now() WHERE id = $2`, msg, job.SourceVideoID); dbErr != nil {
			slog.Error("editor-render: failed to record error", "video_id", job.SourceVideoID, "error", dbErr)
		}
	}

	tmpDir, err := os.MkdirTemp("", "sendrec-editor-render-*")
	if err != nil {
		fail(err)
		return
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	inputs := make([]string, 0, len(job.Sources))
	indexes := make(map[string]int, len(job.Sources))
	for _, clip := range job.Timeline.Clips {
		source := job.Sources[clip.SourceID]
		if _, exists := indexes[source.ID]; exists {
			continue
		}
		path := filepath.Join(tmpDir, fmt.Sprintf("source-%d%s", len(inputs), extensionForContentType(source.ContentType)))
		if err := h.storage.DownloadToFile(ctx, source.FileKey, path); err != nil {
			fail(err)
			return
		}
		source.HasAudio = probeHasAudio(path)
		job.Sources[source.ID] = source
		indexes[source.ID] = len(inputs)
		inputs = append(inputs, path)
	}

	output := filepath.Join(tmpDir, "rendered.mp4")
	cmd := exec.CommandContext(ctx, "ffmpeg", buildTimelineRenderArgs(inputs, job.Timeline.Clips, job.Timeline.Overlays, indexes, job.Sources, output)...)
	if combined, err := cmd.CombinedOutput(); err != nil {
		fail(fmt.Errorf("ffmpeg render: %w: %s", err, string(combined)))
		return
	}
	info, err := os.Stat(output)
	if err != nil {
		fail(err)
		return
	}

	shareToken, err := generateShareToken()
	if err != nil {
		fail(err)
		return
	}
	resultKey := videoFileKey(job.OwnerID, shareToken, "video/mp4")
	if err := h.storage.UploadFile(ctx, resultKey, output, "video/mp4"); err != nil {
		fail(err)
		return
	}
	uploaded := true
	defer func() {
		if uploaded {
			_ = h.storage.DeleteObject(context.Background(), resultKey)
		}
	}()

	var resultID string
	totalDuration := 0.0
	for _, clip := range job.Timeline.Clips {
		totalDuration += clip.Duration
	}
	err = h.db.QueryRow(ctx, `INSERT INTO videos
		(user_id, organization_id, title, status, duration, file_size, file_key, share_token, content_type, ios_normalized)
		VALUES ($1, $2, $3, 'ready', $4, $5, $6, $7, 'video/mp4', true) RETURNING id`,
		job.OwnerID, job.OrganizationID, job.Title+" (bearbeitet)", int(totalDuration), info.Size(), resultKey, shareToken).Scan(&resultID)
	if err != nil {
		fail(err)
		return
	}
	uploaded = false

	if _, err := h.db.Exec(ctx, `UPDATE videos SET edit_render_status = 'ready', edit_render_error = NULL,
		edit_render_video_id = $1, updated_at = now() WHERE id = $2`, resultID, job.SourceVideoID); err != nil {
		fail(err)
		return
	}
	GenerateThumbnail(ctx, h.db, h.storage, resultID, resultKey, thumbnailFileKey(job.OwnerID, shareToken))
	if err := EnqueueTranscription(ctx, h.db, resultID); err != nil {
		slog.Error("editor-render: failed to enqueue transcription", "video_id", resultID, "error", err)
	}
}
