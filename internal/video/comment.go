package video

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/validate"
	"github.com/sendrec/sendrec/internal/webhook"
)

var validCommentModes = map[string]bool{
	"disabled":            true,
	"anonymous":           true,
	"name_required":       true,
	"name_email_required": true,
}

var quickReactionEmojis = []string{"👍", "👎", "❤️", "😂", "😮", "🎉"}
var quickReactionBodies = buildQuickReactionSet()

func buildQuickReactionSet() map[string]bool {
	set := make(map[string]bool, len(quickReactionEmojis))
	for _, emoji := range quickReactionEmojis {
		set[emoji] = true
	}
	return set
}

type setCommentModeRequest struct {
	CommentMode string `json:"commentMode"`
}

type postCommentRequest struct {
	AuthorName     string   `json:"authorName"`
	AuthorEmail    string   `json:"authorEmail"`
	Body           string   `json:"body"`
	IsPrivate      bool     `json:"isPrivate"`
	VideoTimestamp *float64 `json:"videoTimestamp"`
}

type commentResponse struct {
	ID             string   `json:"id"`
	AuthorName     string   `json:"authorName"`
	Body           string   `json:"body"`
	IsPrivate      bool     `json:"isPrivate"`
	IsOwner        bool     `json:"isOwner"`
	CreatedAt      string   `json:"createdAt"`
	VideoTimestamp *float64 `json:"videoTimestamp,omitempty"`
}

func isQuickReactionBody(body string) bool {
	return quickReactionBodies[body]
}

func (h *Handler) SetCommentMode(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setCommentModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validCommentModes[req.CommentMode] {
		httputil.WriteError(w, http.StatusBadRequest, "invalid comment mode")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET comment_mode = $1 WHERE id = $2 AND user_id = $3`,
		req.CommentMode, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not update comment mode")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) optionalUserID(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	tokenStr, found := strings.CutPrefix(authHeader, "Bearer ")
	if !found {
		return ""
	}
	claims, err := auth.ValidateToken(h.hmacSecret, tokenStr)
	if err != nil || claims.TokenType != "access" {
		return ""
	}
	return claims.UserID
}

func (h *Handler) PostWatchComment(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var videoID, ownerID, commentMode string
	var shareExpiresAt time.Time
	var sharePassword *string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.user_id, v.comment_mode, v.share_expires_at, v.share_password
		 FROM videos v WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID, &ownerID, &commentMode, &shareExpiresAt, &sharePassword)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if time.Now().After(shareExpiresAt) {
		httputil.WriteError(w, http.StatusGone, "link expired")
		return
	}

	if sharePassword != nil {
		if !hasValidWatchCookie(r, h.hmacSecret, shareToken, *sharePassword) {
			httputil.WriteError(w, http.StatusForbidden, "password required")
			return
		}
	}

	if commentMode == "disabled" {
		httputil.WriteError(w, http.StatusForbidden, "comments are disabled")
		return
	}

	var req postCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Body = strings.TrimSpace(req.Body)
	req.AuthorName = strings.TrimSpace(req.AuthorName)
	req.AuthorEmail = strings.TrimSpace(req.AuthorEmail)

	if len(req.AuthorName) > 200 {
		httputil.WriteError(w, http.StatusBadRequest, "name is too long")
		return
	}

	if len(req.AuthorEmail) > 320 {
		httputil.WriteError(w, http.StatusBadRequest, "email is too long")
		return
	}

	if req.VideoTimestamp != nil && *req.VideoTimestamp < 0 {
		httputil.WriteError(w, http.StatusBadRequest, "invalid timestamp")
		return
	}

	if req.Body == "" {
		httputil.WriteError(w, http.StatusBadRequest, "comment body is required")
		return
	}

	if msg := validate.CommentBody(req.Body); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	quickReaction := isQuickReactionBody(req.Body)

	switch commentMode {
	case "name_required":
		if req.AuthorName == "" && !quickReaction {
			httputil.WriteError(w, http.StatusBadRequest, "name is required")
			return
		}
	case "name_email_required":
		if req.AuthorName == "" && !quickReaction {
			httputil.WriteError(w, http.StatusBadRequest, "name is required")
			return
		}
		if req.AuthorEmail == "" && !quickReaction {
			httputil.WriteError(w, http.StatusBadRequest, "email is required")
			return
		}
	}

	if quickReaction {
		req.AuthorName = ""
		req.AuthorEmail = ""
	}

	callerUserID := h.optionalUserID(r)

	if req.IsPrivate && callerUserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required for private comments")
		return
	}

	var userIDArg *string
	if callerUserID != "" {
		userIDArg = &callerUserID
	}

	var commentID string
	var createdAt time.Time
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO video_comments (video_id, user_id, author_name, author_email, body, is_private, video_timestamp_seconds)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		videoID, userIDArg, req.AuthorName, req.AuthorEmail, req.Body, req.IsPrivate, req.VideoTimestamp,
	).Scan(&commentID, &createdAt)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not save comment")
		return
	}

	shouldEmailComment := h.commentNotifier != nil && h.shouldSendImmediateCommentNotification(r.Context(), ownerID)
	shouldSlackComment := h.slackNotifier != nil

	if !req.IsPrivate && !quickReaction && callerUserID != ownerID && (shouldEmailComment || shouldSlackComment) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var ownerEmail, ownerName, videoTitle string
			err := h.db.QueryRow(ctx,
				`SELECT u.email, u.name, v.title FROM users u JOIN videos v ON v.user_id = u.id WHERE v.id = $1`,
				videoID,
			).Scan(&ownerEmail, &ownerName, &videoTitle)
			if err != nil {
				slog.Error("comment: failed to fetch owner info for notification", "video_id", videoID, "error", err)
				return
			}
			watchURL := h.baseURL + "/watch/" + shareToken
			authorName := req.AuthorName
			if authorName == "" {
				authorName = "Anonymous"
			}
			if h.webhookClient != nil {
				wURL, wSecret, wErr := h.webhookClient.LookupConfigByUserID(ctx, ownerID)
				if wErr == nil {
					if err := h.webhookClient.Dispatch(ctx, ownerID, wURL, wSecret, webhook.Event{
						Name:      "video.comment",
						Timestamp: time.Now().UTC(),
						Data: map[string]any{
							"videoId":  videoID,
							"title":    videoTitle,
							"watchUrl": watchURL,
							"author":   authorName,
							"body":     req.Body,
						},
					}); err != nil {
						slog.Error("webhook: dispatch failed for video.comment", "video_id", videoID, "error", err)
					}
				}
			}
			if shouldSlackComment {
				if err := h.slackNotifier.SendCommentNotification(ctx, ownerEmail, ownerName, videoTitle, authorName, req.Body, watchURL); err != nil {
					slog.Error("comment: failed to send Slack notification", "video_id", videoID, "error", err)
				}
			}
			if shouldEmailComment {
				if err := h.commentNotifier.SendCommentNotification(ctx, ownerEmail, ownerName, videoTitle, authorName, req.Body, watchURL); err != nil {
					slog.Error("comment: failed to send email notification", "video_id", videoID, "error", err)
				}
			}
		}()
	}

	httputil.WriteJSON(w, http.StatusCreated, commentResponse{
		ID:             commentID,
		AuthorName:     req.AuthorName,
		Body:           req.Body,
		IsPrivate:      req.IsPrivate,
		IsOwner:        callerUserID == ownerID && callerUserID != "",
		CreatedAt:      createdAt.Format(time.RFC3339),
		VideoTimestamp: req.VideoTimestamp,
	})
}

func (h *Handler) lookupWatchVideo(w http.ResponseWriter, r *http.Request) (videoID, ownerID, commentMode string, ok bool) {
	shareToken := chi.URLParam(r, "shareToken")

	var shareExpiresAt time.Time
	var sharePassword *string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.user_id, v.comment_mode, v.share_expires_at, v.share_password
		 FROM videos v WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID, &ownerID, &commentMode, &shareExpiresAt, &sharePassword)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return "", "", "", false
	}

	if time.Now().After(shareExpiresAt) {
		httputil.WriteError(w, http.StatusGone, "link expired")
		return "", "", "", false
	}

	if sharePassword != nil {
		if !hasValidWatchCookie(r, h.hmacSecret, shareToken, *sharePassword) {
			httputil.WriteError(w, http.StatusForbidden, "password required")
			return "", "", "", false
		}
	}

	return videoID, ownerID, commentMode, true
}

func (h *Handler) queryComments(ctx context.Context, videoID, ownerID string, includePrivate bool) ([]commentResponse, error) {
	query := `SELECT c.id, c.user_id, c.author_name, c.body, c.is_private, c.created_at, c.video_timestamp_seconds
		 FROM video_comments c WHERE c.video_id = $1`
	if !includePrivate {
		query += ` AND c.is_private = false`
	}
	query += ` ORDER BY c.created_at ASC`

	rows, err := h.db.Query(ctx, query, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []commentResponse
	for rows.Next() {
		var id, authorName, body string
		var userID *string
		var isPrivate bool
		var createdAt time.Time
		var videoTimestamp *float64

		if err := rows.Scan(&id, &userID, &authorName, &body, &isPrivate, &createdAt, &videoTimestamp); err != nil {
			return nil, err
		}

		isOwner := userID != nil && *userID == ownerID
		comments = append(comments, commentResponse{
			ID:             id,
			AuthorName:     authorName,
			Body:           body,
			IsPrivate:      isPrivate,
			IsOwner:        isOwner,
			CreatedAt:      createdAt.Format(time.RFC3339),
			VideoTimestamp: videoTimestamp,
		})
	}

	if comments == nil {
		comments = []commentResponse{}
	}

	return comments, nil
}

type listCommentsResponseBody struct {
	Comments    []commentResponse `json:"comments"`
	CommentMode string            `json:"commentMode"`
}

func (h *Handler) ListWatchComments(w http.ResponseWriter, r *http.Request) {
	videoID, ownerID, commentMode, ok := h.lookupWatchVideo(w, r)
	if !ok {
		return
	}

	if commentMode == "disabled" {
		httputil.WriteJSON(w, http.StatusOK, listCommentsResponseBody{
			Comments:    []commentResponse{},
			CommentMode: commentMode,
		})
		return
	}

	callerUserID := h.optionalUserID(r)
	includePrivate := callerUserID != "" && callerUserID == ownerID

	comments, err := h.queryComments(r.Context(), videoID, ownerID, includePrivate)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not fetch comments")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, listCommentsResponseBody{
		Comments:    comments,
		CommentMode: commentMode,
	})
}

func (h *Handler) ListOwnerComments(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var ownerID, commentMode string
	err := h.db.QueryRow(r.Context(),
		`SELECT v.user_id, v.comment_mode FROM videos WHERE id = $1 AND user_id = $2`,
		videoID, userID,
	).Scan(&ownerID, &commentMode)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	comments, err := h.queryComments(r.Context(), videoID, ownerID, true)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not fetch comments")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, listCommentsResponseBody{
		Comments:    comments,
		CommentMode: commentMode,
	})
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")
	commentID := chi.URLParam(r, "commentId")

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM video_comments c USING videos v
		 WHERE c.id = $1 AND c.video_id = $2 AND v.id = c.video_id AND v.user_id = $3`,
		commentID, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not delete comment")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "comment not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
