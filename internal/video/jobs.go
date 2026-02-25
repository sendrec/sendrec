package video

import (
	"context"
	"log/slog"
	"time"
)

type JobType string

const (
	JobTypeThumbnail   JobType = "thumbnail"
	JobTypeTranscode   JobType = "transcode"
	JobTypeTranscribe  JobType = "transcribe"
	JobTypeNormalize   JobType = "normalize"
	JobTypeProbe       JobType = "probe"
	JobTypeComposite   JobType = "composite"
)

type Job struct {
	ID        string
	Type      JobType
	VideoID   string
	Payload   map[string]any
	CreatedAt time.Time
}

func (h *Handler) EnqueueJob(ctx context.Context, jobType JobType, videoID string, payload map[string]any) {
	// For now, this is a wrapper around the existing async patterns.
	// In the future, this will write to a 'video_jobs' table.
	slog.Info("job: enqueued", "type", jobType, "video_id", videoID)

	switch jobType {
	case JobTypeThumbnail:
		thumbKey, _ := payload["thumbnailKey"].(string)
		fileKey, _ := payload["fileKey"].(string)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			GenerateThumbnail(ctx, h.db, h.storage, videoID, fileKey, thumbKey)
		}()
	case JobTypeTranscode:
		fileKey, _ := payload["fileKey"].(string)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			TranscodeWebMAsync(ctx, h.db, h.storage, videoID, fileKey)
		}()
	case JobTypeTranscribe:
		_ = EnqueueTranscription(ctx, h.db, videoID)
	case JobTypeNormalize:
		fileKey, _ := payload["fileKey"].(string)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			NormalizeVideoAsync(ctx, h.db, h.storage, videoID, fileKey)
		}()
	case JobTypeProbe:
		fileKey, _ := payload["fileKey"].(string)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			probeDuration(ctx, h.db, h.storage, videoID, fileKey)
		}()
	case JobTypeComposite:
		fileKey, _ := payload["fileKey"].(string)
		webcamKey, _ := payload["webcamKey"].(string)
		thumbKey, _ := payload["thumbnailKey"].(string)
		contentType, _ := payload["contentType"].(string)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			CompositeWithWebcam(ctx, h.db, h.storage, videoID, fileKey, webcamKey, thumbKey, contentType)
		}()
	}
}
