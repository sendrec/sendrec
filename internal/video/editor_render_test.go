package video

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func validTimeline(clips ...editClip) editTimeline {
	return editTimeline{Version: 1, Clips: clips}
}

func TestValidateEditTimeline(t *testing.T) {
	tests := []struct {
		name     string
		timeline editTimeline
		wantErr  bool
	}{
		{"one clip", validTimeline(editClip{ID: "one", SourceID: "source-a", SourceStart: 2, SourceEnd: 5}), false},
		{"same source", validTimeline(
			editClip{ID: "one", SourceID: "source-a", SourceStart: 0, SourceEnd: 2},
			editClip{ID: "two", SourceID: "source-a", SourceStart: 7, SourceEnd: 9}), false},
		{"two sources", validTimeline(
			editClip{ID: "one", SourceID: "source-a", SourceStart: 0, SourceEnd: 2},
			editClip{ID: "two", SourceID: "source-b", SourceStart: 1, SourceEnd: 4}), false},
		{"empty", validTimeline(), true},
		{"invalid range", validTimeline(editClip{ID: "one", SourceID: "source-a", SourceStart: 4, SourceEnd: 4}), true},
		{"duplicate id", validTimeline(
			editClip{ID: "same", SourceID: "source-a", SourceStart: 0, SourceEnd: 2},
			editClip{ID: "same", SourceID: "source-b", SourceStart: 0, SourceEnd: 2}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEditTimeline(&tt.timeline)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateEditTimeline() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				for _, clip := range tt.timeline.Clips {
					if clip.Duration != clip.SourceEnd-clip.SourceStart {
						t.Fatalf("duration was not derived from source range: %+v", clip)
					}
				}
			}
		})
	}
}

func TestBuildTimelineRenderArgsPreservesClipOrderAndRanges(t *testing.T) {
	clips := []editClip{
		{ID: "first", SourceID: "original", SourceStart: 4.2, SourceEnd: 11.8, Duration: 7.6},
		{ID: "second", SourceID: "inserted", SourceStart: 18.1, SourceEnd: 27.4, Duration: 9.3},
		{ID: "third", SourceID: "original", SourceStart: 30, SourceEnd: 35, Duration: 5},
	}
	sources := map[string]sourceVideo{
		"original": {ID: "original", HasAudio: true},
		"inserted": {ID: "inserted", HasAudio: true},
	}
	args := buildTimelineRenderArgs(
		[]string{"original.mp4", "inserted.webm"}, clips,
		map[string]int{"original": 0, "inserted": 1}, sources, "output.mp4",
	)
	joined := strings.Join(args, " ")
	ordered := []string{
		"[0:v]trim=start=4.200:end=11.800",
		"[1:v]trim=start=18.100:end=27.400",
		"[0:v]trim=start=30.000:end=35.000",
		"[v0][a0][v1][a1][v2][a2]concat=n=3:v=1:a=1",
	}
	position := -1
	for _, fragment := range ordered {
		next := strings.Index(joined, fragment)
		if next <= position {
			t.Fatalf("fragment %q missing or out of order in %s", fragment, joined)
		}
		position = next
	}
	if !strings.Contains(joined, "force_original_aspect_ratio=decrease") ||
		!strings.Contains(joined, "pad=1920:1080") {
		t.Fatalf("render args must preserve aspect ratio and pad to the target frame: %s", joined)
	}

	silentSources := map[string]sourceVideo{"silent": {ID: "silent", HasAudio: false}}
	silentArgs := strings.Join(buildTimelineRenderArgs(
		[]string{"silent.mp4"},
		[]editClip{{ID: "silent", SourceID: "silent", SourceStart: 1, SourceEnd: 3.5, Duration: 2.5}},
		map[string]int{"silent": 0}, silentSources, "silent-output.mp4",
	), " ")
	if !strings.Contains(silentArgs, "anullsrc=r=48000:cl=stereo,atrim=duration=2.500") {
		t.Fatalf("silent audio must exactly match the clip duration: %s", silentArgs)
	}
}

func TestBuildTimelineRenderArgsIncludesTimedCoverOverlay(t *testing.T) {
	clips := []editClip{
		{ID: "clip-1", SourceID: "original", SourceStart: 0, SourceEnd: 12, Duration: 12},
	}
	sources := map[string]sourceVideo{
		"original": {ID: "original", HasAudio: true},
	}
	overlays := []editorCoverOverlay{
		{
			ID:     "cover-1",
			X:      25,
			Y:      10,
			Width:  40,
			Height: 20,
			Start:  3.5,
			End:    8.25,
		},
	}

	args := buildTimelineRenderArgs(
		[]string{"original.mp4"},
		clips,
		map[string]int{"original": 0},
		sources,
		"output.mp4",
		overlays,
	)

	joined := strings.Join(args, " ")

	expected := "drawbox=x=iw*0.250000:y=ih*0.100000:w=iw*0.400000:h=ih*0.200000:color=black:t=fill:enable='between(t,3.500,8.250)'"
	if !strings.Contains(joined, expected) {
		t.Fatalf("render args missing timed cover overlay; expected %q in %s", expected, joined)
	}
}

func TestRenderEditorTimelineSavesTimelineAndQueuesResolvedSources(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, true)

	mock.ExpectQuery(`SELECT user_id, organization_id, title FROM videos`).
		WithArgs("video-main", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "title"}).
			AddRow(testUserID, nil, "Original"))
	mock.ExpectQuery(`SELECT file_key, content_type, duration FROM videos`).
		WithArgs("video-main", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "content_type", "duration"}).
			AddRow("recordings/main.mp4", "video/mp4", 20))
	mock.ExpectQuery(`SELECT file_key, content_type, duration FROM videos`).
		WithArgs("video-inserted", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "content_type", "duration"}).
			AddRow("recordings/inserted.webm", "video/webm", 30))
	mock.ExpectExec(`UPDATE videos SET edit_timeline = \$1, edit_render_status = 'processing'`).
		WithArgs(pgxmock.AnyArg(), "video-main", testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	originalEnqueue := enqueueTimelineRender
	defer func() { enqueueTimelineRender = originalEnqueue }()
	var queued renderJob
	enqueueTimelineRender = func(_ *Handler, job renderJob) { queued = job }

	body := `{"version":1,"clips":[` +
		`{"id":"a","sourceId":"video-main","sourceStart":2,"sourceEnd":5,"duration":999},` +
		`{"id":"b","sourceId":"video-inserted","sourceStart":7,"sourceEnd":11,"duration":999},` +
		`{"id":"c","sourceId":"video-main","sourceStart":12,"sourceEnd":14,"duration":999}]}`
	router := chi.NewRouter()
	router.With(newAuthMiddleware()).Post("/api/videos/{id}/editor/render", handler.RenderEditorTimeline)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(t, http.MethodPost, "/api/videos/video-main/editor/render", []byte(body)))

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(queued.Timeline.Clips) != 3 || queued.Timeline.Clips[1].SourceID != "video-inserted" {
		t.Fatalf("queued timeline order changed: %+v", queued.Timeline.Clips)
	}
	if queued.Timeline.Clips[0].Duration != 3 || queued.Timeline.Clips[1].Duration != 4 {
		t.Fatalf("server did not derive clip durations: %+v", queued.Timeline.Clips)
	}
	if len(queued.Sources) != 2 {
		t.Fatalf("expected two unique resolved sources, got %d", len(queued.Sources))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRenderEditorTimelineRejectsUnavailableSource(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, true)

	mock.ExpectQuery(`SELECT user_id, organization_id, title FROM videos`).
		WithArgs("video-main", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "title"}).
			AddRow(testUserID, nil, "Original"))
	mock.ExpectQuery(`SELECT file_key, content_type, duration FROM videos`).
		WithArgs("missing-source", testUserID).
		WillReturnError(fmt.Errorf("not found"))

	originalEnqueue := enqueueTimelineRender
	defer func() { enqueueTimelineRender = originalEnqueue }()
	enqueued := false
	enqueueTimelineRender = func(_ *Handler, _ renderJob) { enqueued = true }

	body := `{"version":1,"clips":[{"id":"a","sourceId":"missing-source","sourceStart":0,"sourceEnd":2}]}`
	router := chi.NewRouter()
	router.With(newAuthMiddleware()).Post("/api/videos/{id}/editor/render", handler.RenderEditorTimeline)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(t, http.MethodPost, "/api/videos/video-main/editor/render", []byte(body)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if enqueued {
		t.Fatal("render must not be queued for an unavailable source")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRenderEditorTimelineRejectsConcurrentRender(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, true)

	mock.ExpectQuery(`SELECT user_id, organization_id, title FROM videos`).
		WithArgs("video-main", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "organization_id", "title"}).
			AddRow(testUserID, nil, "Original"))
	mock.ExpectQuery(`SELECT file_key, content_type, duration FROM videos`).
		WithArgs("video-main", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "content_type", "duration"}).
			AddRow("recordings/main.mp4", "video/mp4", 20))
	mock.ExpectExec(`UPDATE videos SET edit_timeline = \$1, edit_render_status = 'processing'`).
		WithArgs(pgxmock.AnyArg(), "video-main", testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	originalEnqueue := enqueueTimelineRender
	defer func() { enqueueTimelineRender = originalEnqueue }()
	enqueued := false
	enqueueTimelineRender = func(_ *Handler, _ renderJob) { enqueued = true }

	body := `{"version":1,"clips":[{"id":"a","sourceId":"video-main","sourceStart":0,"sourceEnd":2}]}`
	router := chi.NewRouter()
	router.With(newAuthMiddleware()).Post("/api/videos/{id}/editor/render", handler.RenderEditorTimeline)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(t, http.MethodPost, "/api/videos/video-main/editor/render", []byte(body)))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if enqueued {
		t.Fatal("a concurrent render must not enqueue another job")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetEditorStateReturnsStoredTimeline(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, true)
	raw := `{"version":1,"clips":[{"id":"saved","sourceId":"source-a","sourceStart":3,"sourceEnd":8,"duration":5}]}`
	renderedID := "rendered-id"
	mock.ExpectQuery(`SELECT COALESCE\(edit_timeline`).WithArgs("video-main", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"edit_timeline", "edit_render_status", "edit_render_error", "edit_render_video_id"}).
			AddRow([]byte(raw), "ready", (*string)(nil), &renderedID))

	router := chi.NewRouter()
	router.With(newAuthMiddleware()).Get("/api/videos/{id}/editor", handler.GetEditorState)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(t, http.MethodGet, "/api/videos/video-main/editor", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; expectations: %v", recorder.Code, mock.ExpectationsWereMet())
	}
	var state editorStateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Timeline.Clips) != 1 || state.Timeline.Clips[0].SourceStart != 3 || state.RenderedVideo == nil {
		t.Fatalf("unexpected restored state: %+v", state)
	}
}

func TestRenderTimelineFailureOnlyMarksEditFailed(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	storage := &mockStorage{downloadToFileErr: fmt.Errorf("source unavailable")}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, true)
	mock.ExpectExec(`UPDATE videos SET edit_render_status = 'failed'`).
		WithArgs("source unavailable", "original-id").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	handler.renderTimelineAsync(context.Background(), renderJob{
		SourceVideoID: "original-id",
		Timeline:      validTimeline(editClip{ID: "one", SourceID: "original-id", SourceStart: 0, SourceEnd: 2, Duration: 2}),
		Sources:       map[string]sourceVideo{"original-id": {ID: "original-id", FileKey: "original.mp4", ContentType: "video/mp4", Duration: 10}},
	})
	if storage.uploadFileCallCount != 0 || storage.deleteCallCount != 0 {
		t.Fatal("failed render must not upload or delete an original object")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTimelineRenderFFmpegIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").CombinedOutput(); err != nil || !strings.Contains(string(out), "libx264") {
		t.Skip("ffmpeg libx264 encoder not available")
	}
	dir := t.TempDir()
	inputs := []string{filepath.Join(dir, "original-a.mp4"), filepath.Join(dir, "video-b.mp4")}
	colors := []string{"red", "blue"}
	frequencies := []string{"440", "880"}
	sampleRates := []string{"44100", "48000"}
	channels := []string{"1", "2"}
	for i := range inputs {
		cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "color=c="+colors[i]+":s=320x180:d=2",
			"-f", "lavfi", "-i", "sine=frequency="+frequencies[i]+":sample_rate="+sampleRates[i]+":duration=2",
			"-ac", channels[i], "-c:v", "libx264", "-c:a", "aac", "-shortest", "-y", inputs[i])
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("create fixture: %v: %s", err, output)
		}
	}
	output := filepath.Join(dir, "result.mp4")
	clips := []editClip{
		{ID: "a-first", SourceID: "original-a", SourceStart: .25, SourceEnd: .75, Duration: .5},
		{ID: "b", SourceID: "video-b", SourceStart: .5, SourceEnd: 1, Duration: .5},
		{ID: "a-last", SourceID: "original-a", SourceStart: 1.25, SourceEnd: 1.75, Duration: .5},
	}
	sources := map[string]sourceVideo{
		"original-a": {ID: "original-a", HasAudio: true},
		"video-b":    {ID: "video-b", HasAudio: true},
	}
	cmd := exec.Command("ffmpeg", buildTimelineRenderArgs(inputs, clips, map[string]int{"original-a": 0, "video-b": 1}, sources, output)...)
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v: %s", err, combined)
	}
	if info, err := os.Stat(output); err != nil || info.Size() == 0 {
		t.Fatalf("render output missing or empty: %v", err)
	}

	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Duration  string `json:"duration"`
		} `json:"streams"`
	}
	probeCmd := exec.Command("ffprobe", "-v", "error", "-show_entries",
		"stream=codec_type,width,height,duration", "-of", "json", output)
	probeOutput, err := probeCmd.Output()
	if err != nil {
		t.Fatalf("ffprobe failed: %v", err)
	}
	if err := json.Unmarshal(probeOutput, &probe); err != nil {
		t.Fatalf("decode ffprobe output: %v: %s", err, probeOutput)
	}
	if len(probe.Streams) != 2 {
		t.Fatalf("expected video and audio streams, got %+v", probe.Streams)
	}
	var videoDuration, audioDuration float64
	for _, stream := range probe.Streams {
		d, _ := strconv.ParseFloat(stream.Duration, 64)
		switch stream.CodecType {
		case "video":
			if stream.Width != 1920 || stream.Height != 1080 {
				t.Fatalf("unexpected output dimensions: %dx%d", stream.Width, stream.Height)
			}
			videoDuration = d
		case "audio":
			audioDuration = d
		}
	}
	if math.Abs(videoDuration-1.5) > .08 || math.Abs(audioDuration-1.5) > .08 {
		t.Fatalf("unexpected stream durations: video=%f audio=%f", videoDuration, audioDuration)
	}
	if math.Abs(videoDuration-audioDuration) > .05 {
		t.Fatalf("audio/video drift detected: video=%f audio=%f", videoDuration, audioDuration)
	}

	assertDominantColor := func(at string, channel int) {
		t.Helper()
		pixelCmd := exec.Command("ffmpeg", "-v", "error", "-ss", at, "-i", output,
			"-vf", "scale=1:1", "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
		pixel, err := pixelCmd.Output()
		if err != nil || len(pixel) < 3 {
			t.Fatalf("sample frame at %s: %v (%v)", at, err, pixel)
		}
		other := 2
		if channel == 2 {
			other = 0
		}
		if int(pixel[channel])-int(pixel[other]) < 80 {
			t.Fatalf("unexpected color at %s: rgb=%v", at, pixel[:3])
		}
	}
	assertDominantColor("0.25", 0)
	assertDominantColor("0.75", 2)
	assertDominantColor("1.25", 0)
}
