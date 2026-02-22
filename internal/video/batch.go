package video

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/webhook"
)

const maxBatchSize = 100

type batchRequest struct {
	VideoIDs []string `json:"videoIds"`
}

type batchDeleteResponse struct {
	Deleted int `json:"deleted"`
}

func (h *Handler) BatchDelete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.VideoIDs) == 0 || len(req.VideoIDs) > maxBatchSize {
		httputil.WriteError(w, http.StatusBadRequest, "videoIds must contain 1-100 items")
		return
	}

	rows, err := h.db.Query(r.Context(),
		`UPDATE videos SET status = 'deleted', updated_at = now()
		 WHERE id = ANY($1) AND user_id = $2 AND status != 'deleted'
		 RETURNING id, file_key, thumbnail_key, webcam_key, transcript_key, title`,
		req.VideoIDs, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete videos")
		return
	}
	defer rows.Close()

	type deletedVideo struct {
		id, fileKey, title                     string
		thumbnailKey, webcamKey, transcriptKey *string
	}
	var deleted []deletedVideo
	for rows.Next() {
		var d deletedVideo
		if err := rows.Scan(&d.id, &d.fileKey, &d.thumbnailKey, &d.webcamKey, &d.transcriptKey, &d.title); err != nil {
			continue
		}
		deleted = append(deleted, d)
	}
	if err := rows.Err(); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to process deletions")
		return
	}

	for _, d := range deleted {
		go func(dv deletedVideo) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := deleteWithRetry(ctx, h.storage, dv.fileKey, 3); err != nil {
				log.Printf("batch delete: all retries failed for %s: %v", dv.fileKey, err)
				return
			}
			if dv.thumbnailKey != nil {
				if err := deleteWithRetry(ctx, h.storage, *dv.thumbnailKey, 3); err != nil {
					log.Printf("batch delete: thumbnail delete failed for %s: %v", *dv.thumbnailKey, err)
				}
			}
			if dv.webcamKey != nil {
				if err := deleteWithRetry(ctx, h.storage, *dv.webcamKey, 3); err != nil {
					log.Printf("batch delete: webcam delete failed for %s: %v", *dv.webcamKey, err)
				}
			}
			if dv.transcriptKey != nil {
				if err := deleteWithRetry(ctx, h.storage, *dv.transcriptKey, 3); err != nil {
					log.Printf("batch delete: transcript delete failed for %s: %v", *dv.transcriptKey, err)
				}
			}
			if _, err := h.db.Exec(ctx,
				`UPDATE videos SET file_purged_at = now() WHERE id = $1`,
				dv.id,
			); err != nil {
				log.Printf("batch delete: failed to mark file_purged_at for %s: %v", dv.id, err)
			}
		}(d)

		h.dispatchWebhook(userID, webhook.Event{
			Name:      "video.deleted",
			Timestamp: time.Now().UTC(),
			Data: map[string]any{
				"videoId": d.id,
				"title":   d.title,
			},
		})
	}

	httputil.WriteJSON(w, http.StatusOK, batchDeleteResponse{Deleted: len(deleted)})
}

type batchFolderRequest struct {
	VideoIDs []string `json:"videoIds"`
	FolderID *string  `json:"folderId"`
}

func (h *Handler) BatchSetFolder(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req batchFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.VideoIDs) == 0 || len(req.VideoIDs) > maxBatchSize {
		httputil.WriteError(w, http.StatusBadRequest, "videoIds must contain 1-100 items")
		return
	}

	var folderID *string
	if req.FolderID != nil && *req.FolderID != "" {
		var id string
		err := h.db.QueryRow(r.Context(),
			`SELECT id FROM folders WHERE id = $1 AND user_id = $2`,
			*req.FolderID, userID,
		).Scan(&id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httputil.WriteError(w, http.StatusNotFound, "folder not found")
				return
			}
			httputil.WriteError(w, http.StatusInternalServerError, "failed to verify folder")
			return
		}
		folderID = req.FolderID
	}

	_, err := h.db.Exec(r.Context(),
		`UPDATE videos SET folder_id = $1, updated_at = now()
		 WHERE id = ANY($2) AND user_id = $3 AND status != 'deleted'`,
		folderID, req.VideoIDs, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update video folders")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type batchTagsRequest struct {
	VideoIDs []string `json:"videoIds"`
	TagIDs   []string `json:"tagIds"`
}

func (h *Handler) BatchSetTags(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req batchTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TagIDs == nil {
		req.TagIDs = []string{}
	}

	if len(req.VideoIDs) == 0 || len(req.VideoIDs) > maxBatchSize {
		httputil.WriteError(w, http.StatusBadRequest, "videoIds must contain 1-100 items")
		return
	}
	if len(req.TagIDs) > maxTagsPerVideo {
		httputil.WriteError(w, http.StatusBadRequest, "maximum 10 tags per video")
		return
	}

	if len(req.TagIDs) > 0 {
		var count int
		err := h.db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM tags WHERE id = ANY($1) AND user_id = $2`,
			req.TagIDs, userID,
		).Scan(&count)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to verify tags")
			return
		}
		if count != len(req.TagIDs) {
			httputil.WriteError(w, http.StatusBadRequest, "one or more tags not found")
			return
		}
	}

	var videoCount int
	err := h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM videos WHERE id = ANY($1) AND user_id = $2 AND status != 'deleted'`,
		req.VideoIDs, userID,
	).Scan(&videoCount)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify videos")
		return
	}

	for _, videoID := range req.VideoIDs {
		if _, err := h.db.Exec(r.Context(),
			`DELETE FROM video_tags WHERE video_id = $1`,
			videoID,
		); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update video tags")
			return
		}

		for _, tagID := range req.TagIDs {
			if _, err := h.db.Exec(r.Context(),
				`INSERT INTO video_tags (video_id, tag_id) VALUES ($1, $2)`,
				videoID, tagID,
			); err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "failed to update video tags")
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
