package video

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

type transferRequest struct {
	OrganizationID *string `json:"organizationId"`
}

type transferResponse struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	OrganizationID *string `json:"organizationId"`
}

func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")
	callerOrgID := auth.OrgIDFromContext(r.Context())
	callerRole := auth.OrgRoleFromContext(r.Context())

	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Load video
	var videoUserID string
	var videoOrgID *string
	var status string
	var title string
	err := h.db.QueryRow(r.Context(),
		"SELECT user_id, organization_id, status, title FROM videos WHERE id = $1 AND status != 'deleted'",
		videoID,
	).Scan(&videoUserID, &videoOrgID, &status, &title)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load video")
		return
	}

	// Block transfer of processing videos
	if status != "ready" {
		httputil.WriteError(w, http.StatusBadRequest, "cannot transfer a video that is still processing")
		return
	}

	// Permission check: who can transfer this video?
	isCreator := videoUserID == userID
	isOrgAdmin := callerOrgID != "" && (callerRole == "owner" || callerRole == "admin")

	// Video must belong to the caller's current scope
	if callerOrgID != "" {
		// Caller is in an org context — video must be in that org
		if videoOrgID == nil || *videoOrgID != callerOrgID {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
		if !isCreator && !isOrgAdmin {
			httputil.WriteError(w, http.StatusForbidden, "only the creator or org admins can transfer videos")
			return
		}
	} else {
		// Caller is in personal context — video must be personal and owned by caller
		if videoOrgID != nil || !isCreator {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
	}

	// Validate target
	if req.OrganizationID != nil {
		// Moving to a workspace — caller must be a non-viewer member
		var targetRole string
		err := h.db.QueryRow(r.Context(),
			"SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2",
			*req.OrganizationID, userID,
		).Scan(&targetRole)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httputil.WriteError(w, http.StatusForbidden, "you are not a member of the target workspace")
				return
			}
			httputil.WriteError(w, http.StatusInternalServerError, "failed to verify target membership")
			return
		}
		if targetRole == "viewer" {
			httputil.WriteError(w, http.StatusForbidden, "viewers cannot receive transfers")
			return
		}
	}

	// Prevent no-op transfer
	if (req.OrganizationID == nil && videoOrgID == nil) ||
		(req.OrganizationID != nil && videoOrgID != nil && *req.OrganizationID == *videoOrgID) {
		httputil.WriteError(w, http.StatusBadRequest, "video is already in the target scope")
		return
	}

	// Execute transfer: update org, clear folder (scope-specific), clear tags
	_, err = h.db.Exec(r.Context(),
		"UPDATE videos SET organization_id = $1, folder_id = NULL, updated_at = now() WHERE id = $2",
		req.OrganizationID, videoID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to transfer video")
		return
	}

	// Clear tags (scope-specific)
	_, _ = h.db.Exec(r.Context(), "DELETE FROM video_tags WHERE video_id = $1", videoID)

	httputil.WriteJSON(w, http.StatusOK, transferResponse{
		ID:             videoID,
		Title:          title,
		OrganizationID: req.OrganizationID,
	})
}
