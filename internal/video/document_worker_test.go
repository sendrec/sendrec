package video

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestProcessNextDocument_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	segments := []TranscriptSegment{
		{Start: 0, End: 5, Text: "Hello everyone"},
		{Start: 5, End: 12, Text: "Today we discuss testing patterns."},
	}
	transcriptJSON, _ := json.Marshal(segments)

	documentMarkdown := "## Introduction\n\nA video about testing patterns.\n\n## Key Takeaways\n\n- Testing is important"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: documentMarkdown}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ai := NewAIClient(server.URL, "test-key", "test-model", 0)

	// Stuck job reset
	mock.ExpectExec(`UPDATE videos SET document_status = 'pending'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// Claim next pending job
	mock.ExpectQuery(`UPDATE videos SET document_status = 'processing'`).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "transcript_json"}).
				AddRow("vid-1", transcriptJSON),
		)

	// Save document result
	mock.ExpectExec(`UPDATE videos SET document = .+, document_status = 'ready'`).
		WithArgs(pgxmock.AnyArg(), "vid-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	processNextDocument(context.Background(), mock, ai)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessNextDocument_NoPendingJobs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Stuck job reset
	mock.ExpectExec(`UPDATE videos SET document_status = 'pending'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// Claim query returns no rows
	mock.ExpectQuery(`UPDATE videos SET document_status = 'processing'`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "transcript_json"}))

	processNextDocument(context.Background(), mock, nil)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
