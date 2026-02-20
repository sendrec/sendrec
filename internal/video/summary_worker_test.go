package video

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestFormatTranscriptForLLM(t *testing.T) {
	segments := []TranscriptSegment{
		{Start: 0, End: 5, Text: "First."},
		{Start: 65, End: 70, Text: "Second."},
		{Start: 3661, End: 3670, Text: "Third."},
	}

	got := formatTranscriptForLLM(segments)
	want := "[00:00] First.\n[01:05] Second.\n[61:01] Third.\n"

	if got != want {
		t.Errorf("formatTranscriptForLLM =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatTranscriptForLLM_Truncation(t *testing.T) {
	// Build segments that would exceed maxTranscriptChars
	var segments []TranscriptSegment
	for i := 0; i < 5000; i++ {
		segments = append(segments, TranscriptSegment{
			Start: float64(i * 10),
			End:   float64(i*10 + 10),
			Text:  "This is a segment with enough text to fill up the buffer over time.",
		})
	}

	got := formatTranscriptForLLM(segments)
	if len(got) > maxTranscriptChars+200 { // small buffer for last line
		t.Errorf("formatTranscriptForLLM output too long: %d chars (max %d)", len(got), maxTranscriptChars)
	}
	if len(got) == 0 {
		t.Error("formatTranscriptForLLM returned empty string")
	}
}

func TestProcessNextSummary_SkipsTrivialTranscript(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	segments := []TranscriptSegment{
		{Start: 0, End: 5, Text: "Only one segment."},
	}
	transcriptJSON, _ := json.Marshal(segments)

	ai := NewAIClient("http://localhost", "key", "model")

	mock.ExpectExec(`UPDATE videos SET summary_status = 'pending'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	mock.ExpectQuery(`UPDATE videos SET summary_status = 'processing'`).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "transcript_json"}).
				AddRow("vid-1", transcriptJSON),
		)

	mock.ExpectExec(`UPDATE videos SET summary_status = 'failed'`).
		WithArgs("vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	processNextSummary(context.Background(), mock, ai)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessNextSummary_ClaimsAndProcesses(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	segments := []TranscriptSegment{
		{Start: 0, End: 10, Text: "Hello world."},
		{Start: 10, End: 20, Text: "Testing patterns."},
	}
	transcriptJSON, _ := json.Marshal(segments)

	summaryJSON := `{"summary":"A test video.","chapters":[{"title":"Introduction","start":0},{"title":"Testing","start":10}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: summaryJSON}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ai := NewAIClient(server.URL, "test-key", "test-model")

	// Stuck job reset
	mock.ExpectExec(`UPDATE videos SET summary_status = 'pending'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// Claim next pending job
	mock.ExpectQuery(`UPDATE videos SET summary_status = 'processing'`).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "transcript_json"}).
				AddRow("vid-1", transcriptJSON),
		)

	// Save summary result
	chaptersOut, _ := json.Marshal([]Chapter{
		{Title: "Introduction", Start: 0},
		{Title: "Testing", Start: 10},
	})
	mock.ExpectExec(`UPDATE videos SET summary = .+, chapters = .+, summary_status = 'ready'`).
		WithArgs("A test video.", chaptersOut, "vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	processNextSummary(context.Background(), mock, ai)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessNextSummary_NoPendingJobs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Stuck job reset
	mock.ExpectExec(`UPDATE videos SET summary_status = 'pending'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// Claim query returns no rows
	mock.ExpectQuery(`UPDATE videos SET summary_status = 'processing'`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "transcript_json"}))

	processNextSummary(context.Background(), mock, nil)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessNextSummary_AIFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	segments := []TranscriptSegment{
		{Start: 0, End: 10, Text: "Hello world."},
		{Start: 10, End: 20, Text: "More content here."},
	}
	transcriptJSON, _ := json.Marshal(segments)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	ai := NewAIClient(server.URL, "test-key", "test-model")

	// Stuck job reset
	mock.ExpectExec(`UPDATE videos SET summary_status = 'pending'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// Claim next pending job
	mock.ExpectQuery(`UPDATE videos SET summary_status = 'processing'`).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "transcript_json"}).
				AddRow("vid-1", transcriptJSON),
		)

	// Mark as failed
	mock.ExpectExec(`UPDATE videos SET summary_status = 'failed'`).
		WithArgs("vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	processNextSummary(context.Background(), mock, ai)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
