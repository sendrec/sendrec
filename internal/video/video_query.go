package video

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/validate"
)

const defaultPageSize = 50
const maxPageSize = 100

type listItem struct {
	ID                    string             `json:"id"`
	Title                 string             `json:"title"`
	Status                string             `json:"status"`
	Duration              int                `json:"duration"`
	ShareToken            string             `json:"shareToken"`
	ShareURL              string             `json:"shareUrl"`
	CreatedAt             string             `json:"createdAt"`
	ShareExpiresAt        *string            `json:"shareExpiresAt"`
	ViewCount             int64              `json:"viewCount"`
	UniqueViewCount       int64              `json:"uniqueViewCount"`
	ThumbnailURL          string             `json:"thumbnailUrl,omitempty"`
	HasPassword           bool               `json:"hasPassword"`
	CommentMode           string             `json:"commentMode"`
	CommentCount          int64              `json:"commentCount"`
	TranscriptStatus      string             `json:"transcriptStatus"`
	ViewNotification      *string            `json:"viewNotification"`
	DownloadEnabled       bool               `json:"downloadEnabled"`
	CtaText               *string            `json:"ctaText"`
	CtaUrl                *string            `json:"ctaUrl"`
	EmailGateEnabled      bool               `json:"emailGateEnabled"`
	SummaryStatus         string             `json:"summaryStatus"`
	DocumentStatus        string             `json:"documentStatus"`
	SuggestedTitle        *string            `json:"suggestedTitle"`
	FolderID              *string            `json:"folderId"`
	TranscriptionLanguage *string            `json:"transcriptionLanguage"`
	NoiseReduction        bool               `json:"noiseReduction"`
	Tags                  []listItemTag      `json:"tags"`
	Playlists             []listItemPlaylist `json:"playlists"`
}

type listItemTag struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Color *string `json:"color"`
}

type listItemPlaylist struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type watchResponse struct {
	Title            string              `json:"title"`
	VideoURL         string              `json:"videoUrl"`
	Duration         int                 `json:"duration"`
	Creator          string              `json:"creator"`
	CreatedAt        string              `json:"createdAt"`
	ContentType      string              `json:"contentType"`
	ThumbnailURL     string              `json:"thumbnailUrl,omitempty"`
	TranscriptStatus string              `json:"transcriptStatus"`
	TranscriptURL    string              `json:"transcriptUrl,omitempty"`
	Segments         []TranscriptSegment `json:"segments,omitempty"`
	Branding         brandingConfig      `json:"branding"`
	CtaText          *string             `json:"ctaText,omitempty"`
	CtaUrl           *string             `json:"ctaUrl,omitempty"`
	Summary          string              `json:"summary,omitempty"`
	Chapters         []Chapter           `json:"chapters,omitempty"`
	SummaryStatus    string              `json:"summaryStatus"`
	Document         string              `json:"document,omitempty"`
	DocumentStatus   string              `json:"documentStatus"`
}

func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type limitsResponse struct {
	MaxVideosPerMonth       int            `json:"maxVideosPerMonth"`
	MaxVideoDurationSeconds int            `json:"maxVideoDurationSeconds"`
	VideosUsedThisMonth     int            `json:"videosUsedThisMonth"`
	BrandingEnabled         bool           `json:"brandingEnabled"`
	AiEnabled               bool           `json:"aiEnabled"`
	TranscriptionEnabled    bool           `json:"transcriptionEnabled"`
	NoiseReductionEnabled   bool           `json:"noiseReductionEnabled"`
	MaxPlaylists            int            `json:"maxPlaylists"`
	PlaylistsUsed           int            `json:"playlistsUsed"`
	FieldLimits             map[string]int `json:"fieldLimits"`
}

func (h *Handler) Limits(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	orgID := auth.OrgIDFromContext(r.Context())
	var plan string
	if orgID != "" {
		plan, _ = h.getOrgPlan(r.Context(), orgID)
	} else {
		plan, _ = h.getUserPlan(r.Context(), userID)
	}

	maxVideos := h.maxVideosPerMonth
	maxDuration := h.maxVideoDurationSeconds
	if plan == "pro" || plan == "business" {
		maxVideos = 0
		maxDuration = 0
	}

	var videosUsed int
	if maxVideos > 0 {
		var err error
		if orgID != "" {
			videosUsed, err = h.countOrgVideosThisMonth(r.Context(), orgID)
		} else {
			videosUsed, err = h.countVideosThisMonth(r.Context(), userID)
		}
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to check video limit")
			return
		}
	}

	maxPlaylists := h.maxPlaylists
	var playlistsUsed int
	if plan == "pro" || plan == "business" {
		maxPlaylists = 0
	} else {
		_ = h.db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM playlists WHERE user_id = $1`,
			userID,
		).Scan(&playlistsUsed)
	}

	httputil.WriteJSON(w, http.StatusOK, limitsResponse{
		MaxVideosPerMonth:       maxVideos,
		MaxVideoDurationSeconds: maxDuration,
		VideosUsedThisMonth:     videosUsed,
		BrandingEnabled:         h.brandingEnabled,
		AiEnabled:               h.aiEnabled,
		TranscriptionEnabled:    h.transcriptionEnabled,
		NoiseReductionEnabled:   h.noiseReductionFilter != "",
		MaxPlaylists:            maxPlaylists,
		PlaylistsUsed:           playlistsUsed,
		FieldLimits:             validate.FieldLimits(),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	limit := defaultPageSize
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o > 0 {
		offset = o
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))

	baseQuery := `SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at,
		    (SELECT COUNT(*) FROM video_views vv WHERE vv.video_id = v.id) AS view_count,
		    (SELECT COUNT(DISTINCT vv.viewer_hash) FROM video_views vv WHERE vv.video_id = v.id) AS unique_view_count,
		    v.thumbnail_key, v.share_password, v.comment_mode,
		    (SELECT COUNT(*) FROM video_comments vc WHERE vc.video_id = v.id) AS comment_count,
		    v.transcript_status, v.view_notification, v.download_enabled, v.cta_text, v.cta_url, v.email_gate_enabled, v.summary_status, v.document_status,
		    v.suggested_title, v.folder_id, v.transcription_language, v.noise_reduction,
		    COALESCE((SELECT json_agg(json_build_object('id', t.id, 'name', t.name, 'color', t.color) ORDER BY t.name)
		      FROM video_tags vt JOIN tags t ON t.id = vt.tag_id
		      WHERE vt.video_id = v.id), '[]'::json) AS tags_json,
		    COALESCE((SELECT json_agg(json_build_object('id', p.id, 'title', p.title) ORDER BY p.title)
		      FROM playlist_videos pv JOIN playlists p ON p.id = pv.playlist_id
		      WHERE pv.video_id = v.id), '[]'::json) AS playlists_json
		 FROM videos v
		 WHERE v.status != 'deleted'`

	var args []any
	paramIdx := 1

	orgID := auth.OrgIDFromContext(r.Context())
	if orgID != "" {
		baseQuery += fmt.Sprintf(` AND v.organization_id = $%d`, paramIdx)
		args = append(args, orgID)
	} else {
		baseQuery += fmt.Sprintf(` AND v.user_id = $%d`, paramIdx)
		args = append(args, userID)
	}
	paramIdx++

	if query != "" {
		escaped := strings.NewReplacer(``, ``, `%`, `\%`, `_`, `\_`).Replace(query)
		args = append(args, "%"+escaped+"%")
		baseQuery += fmt.Sprintf(` AND (v.title ILIKE $%d OR EXISTS (
			SELECT 1 FROM jsonb_array_elements(v.transcript_json) seg
			WHERE seg->>'text' ILIKE $%d
		))`, paramIdx, paramIdx)
		paramIdx++
	}

	folderFilter := r.URL.Query().Get("folder_id")
	if folderFilter == "unfiled" {
		baseQuery += " AND v.folder_id IS NULL"
	} else if folderFilter != "" {
		baseQuery += fmt.Sprintf(` AND v.folder_id = $%d`, paramIdx)
		args = append(args, folderFilter)
		paramIdx++
	}

	tagFilter := r.URL.Query().Get("tag_id")
	if tagFilter != "" {
		baseQuery += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM video_tags vt WHERE vt.video_id = v.id AND vt.tag_id = $%d)`, paramIdx)
		args = append(args, tagFilter)
		paramIdx++
	}

	baseQuery += fmt.Sprintf(` ORDER BY v.created_at DESC LIMIT $%d OFFSET $%d`, paramIdx, paramIdx+1)
	args = append(args, limit, offset)

	rows, err := h.db.Query(r.Context(), baseQuery, args...)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list videos")
		return
	}
	defer rows.Close()

	items := []listItem{}
	for rows.Next() {
		var item listItem
		var createdAt time.Time
		var shareExpiresAt *time.Time
		var thumbnailKey *string
		var sharePassword *string
		var tagsJSON string
		var playlistsJSON string
		if err := rows.Scan(&item.ID, &item.Title, &item.Status, &item.Duration, &item.ShareToken, &createdAt, &shareExpiresAt, &item.ViewCount, &item.UniqueViewCount, &thumbnailKey, &sharePassword, &item.CommentMode, &item.CommentCount, &item.TranscriptStatus, &item.ViewNotification, &item.DownloadEnabled, &item.CtaText, &item.CtaUrl, &item.EmailGateEnabled, &item.SummaryStatus, &item.DocumentStatus, &item.SuggestedTitle, &item.FolderID, &item.TranscriptionLanguage, &item.NoiseReduction, &tagsJSON, &playlistsJSON); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan video")
			return
		}
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			item.Tags = make([]listItemTag, 0)
		}
		if item.Tags == nil {
			item.Tags = make([]listItemTag, 0)
		}
		if err := json.Unmarshal([]byte(playlistsJSON), &item.Playlists); err != nil {
			item.Playlists = make([]listItemPlaylist, 0)
		}
		if item.Playlists == nil {
			item.Playlists = make([]listItemPlaylist, 0)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		if shareExpiresAt != nil {
			formatted := shareExpiresAt.Format(time.RFC3339)
			item.ShareExpiresAt = &formatted
		}
		item.ShareURL = h.baseURL + "/watch/" + item.ShareToken
		item.HasPassword = sharePassword != nil
		if thumbnailKey != nil {
			thumbURL, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour)
			if err == nil {
				item.ThumbnailURL = thumbURL
			}
		}
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) Watch(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var videoID string
	var title string
	var duration int
	var fileKey string
	var creator string
	var createdAt time.Time
	var shareExpiresAt *time.Time
	var thumbnailKey *string
	var sharePassword *string
	var transcriptKey *string
	var transcriptJSON *string
	var transcriptStatus string
	var ownerID string
	var ownerEmail string
	var viewNotification *string
	var contentType string
	var ctaText, ctaUrl *string
	var summaryText *string
	var chaptersJSON *string
	var summaryStatus string
	var documentText *string
	var documentStatus string
	var ubCompanyName, ubLogoKey, ubColorBg, ubColorSurface, ubColorText, ubColorAccent, ubFooterText, ubCustomCSS *string
	var obCompanyName, obLogoKey, obColorBg, obColorSurface, obColorText, obColorAccent, obFooterText, obCustomCSS *string
	var vbCompanyName, vbLogoKey, vbColorBg, vbColorSurface, vbColorText, vbColorAccent, vbFooterText *string
	var videoOrgID *string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key, v.share_password,
		        v.transcript_key, v.transcript_json, v.transcript_status,
		        v.user_id, u.email, v.view_notification, v.content_type,
		        ub.company_name, ub.logo_key, ub.color_background, ub.color_surface, ub.color_text, ub.color_accent, ub.footer_text, ub.custom_css,
		        ob.company_name, ob.logo_key, ob.color_background, ob.color_surface, ob.color_text, ob.color_accent, ob.footer_text, ob.custom_css,
		        v.branding_company_name, v.branding_logo_key, v.branding_color_background, v.branding_color_surface, v.branding_color_text, v.branding_color_accent, v.branding_footer_text,
		        v.cta_text, v.cta_url,
		        v.summary, v.chapters, v.summary_status,
		        v.document, v.document_status,
		        v.organization_id
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 LEFT JOIN user_branding ub ON ub.user_id = v.user_id AND ub.organization_id IS NULL
		 LEFT JOIN user_branding ob ON ob.organization_id = v.organization_id
		 WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID, &title, &duration, &fileKey, &creator, &createdAt, &shareExpiresAt, &thumbnailKey, &sharePassword,
		&transcriptKey, &transcriptJSON, &transcriptStatus,
		&ownerID, &ownerEmail, &viewNotification, &contentType,
		&ubCompanyName, &ubLogoKey, &ubColorBg, &ubColorSurface, &ubColorText, &ubColorAccent, &ubFooterText, &ubCustomCSS,
		&obCompanyName, &obLogoKey, &obColorBg, &obColorSurface, &obColorText, &obColorAccent, &obFooterText, &obCustomCSS,
		&vbCompanyName, &vbLogoKey, &vbColorBg, &vbColorSurface, &vbColorText, &vbColorAccent, &vbFooterText,
		&ctaText, &ctaUrl,
		&summaryText, &chaptersJSON, &summaryStatus,
		&documentText, &documentStatus,
		&videoOrgID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if shareExpiresAt != nil && time.Now().After(*shareExpiresAt) {
		httputil.WriteError(w, http.StatusGone, "link expired")
		return
	}

	if sharePassword != nil {
		if !hasValidWatchCookie(r, h.hmacSecret, shareToken, *sharePassword) {
			httputil.WriteError(w, http.StatusForbidden, "password required")
			return
		}
	}

	baseBranding := brandingSettingsResponse{
		CompanyName: ubCompanyName, LogoKey: ubLogoKey,
		ColorBackground: ubColorBg, ColorSurface: ubColorSurface,
		ColorText: ubColorText, ColorAccent: ubColorAccent, FooterText: ubFooterText,
		CustomCSS: ubCustomCSS,
	}
	if videoOrgID != nil {
		baseBranding = brandingSettingsResponse{
			CompanyName: obCompanyName, LogoKey: obLogoKey,
			ColorBackground: obColorBg, ColorSurface: obColorSurface,
			ColorText: obColorText, ColorAccent: obColorAccent, FooterText: obFooterText,
			CustomCSS: obCustomCSS,
		}
	}
	branding := resolveBranding(r.Context(), h.storage,
		baseBranding,
		brandingSettingsResponse{
			CompanyName: vbCompanyName, LogoKey: vbLogoKey,
			ColorBackground: vbColorBg, ColorSurface: vbColorSurface,
			ColorText: vbColorText, ColorAccent: vbColorAccent, FooterText: vbFooterText,
		},
	)

	viewerUserID := h.viewerUserIDFromRequest(r)

	if r.URL.Query().Get("poll") != "transcript" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			ip := clientIP(r)
			hash := viewerHash(ip, r.UserAgent())
			ref := categorizeReferrer(r.Header.Get("Referer"))
			browser := parseBrowser(r.UserAgent())
			device := parseDevice(r.UserAgent())
			var country, city string
			if h.geoResolver != nil {
				country, city = h.geoResolver.Lookup(ip)
			}
			if _, err := h.db.Exec(ctx,
				`INSERT INTO video_views (video_id, viewer_hash, referrer, browser, device, country, city)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				videoID, hash, ref, browser, device, country, city,
			); err != nil {
				slog.Error("video: failed to record view", "video_id", videoID, "error", err)
			}
			h.resolveAndNotify(ctx, videoID, ownerID, ownerEmail, creator, title, shareToken, viewerUserID, viewNotification)
		}()
	}

	videoURL, err := h.storage.GenerateDownloadURL(r.Context(), fileKey, 1*time.Hour)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate video URL")
		return
	}

	var thumbnailURL string
	if thumbnailKey != nil {
		if u, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour); err == nil {
			thumbnailURL = u
		}
	}

	var transcriptURL string
	segments := make([]TranscriptSegment, 0)
	if transcriptKey != nil {
		if u, err := h.storage.GenerateDownloadURL(r.Context(), *transcriptKey, 1*time.Hour); err == nil {
			transcriptURL = u
		}
	}
	if transcriptJSON != nil {
		_ = json.Unmarshal([]byte(*transcriptJSON), &segments)
	}

	var summary string
	chapters := make([]Chapter, 0)
	if summaryText != nil {
		summary = *summaryText
	}
	if chaptersJSON != nil {
		_ = json.Unmarshal([]byte(*chaptersJSON), &chapters)
	}

	var document string
	if documentText != nil {
		document = *documentText
	}

	httputil.WriteJSON(w, http.StatusOK, watchResponse{
		Title:            title,
		VideoURL:         videoURL,
		Duration:         duration,
		Creator:          creator,
		CreatedAt:        createdAt.Format(time.RFC3339),
		ContentType:      contentType,
		ThumbnailURL:     thumbnailURL,
		TranscriptStatus: transcriptStatus,
		TranscriptURL:    transcriptURL,
		Segments:         segments,
		Branding:         branding,
		CtaText:          ctaText,
		CtaUrl:           ctaUrl,
		Summary:          summary,
		Chapters:         chapters,
		SummaryStatus:    summaryStatus,
		Document:         document,
		DocumentStatus:   documentStatus,
	})
}

func (h *Handler) getUserPlan(ctx context.Context, userID string) (string, error) {
	var plan string
	err := h.db.QueryRow(ctx, `SELECT subscription_plan FROM users WHERE id = $1`, userID).Scan(&plan)
	if err != nil {
		return "free", err
	}
	return plan, nil
}

func (h *Handler) countVideosThisMonth(ctx context.Context, userID string) (int, error) {
	var count int
	err := h.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM videos WHERE user_id = $1 AND created_at >= date_trunc('month', now()) AND status != 'deleted'`,
		userID,
	).Scan(&count)
	return count, err
}

func (h *Handler) getOrgPlan(ctx context.Context, orgID string) (string, error) {
	var plan string
	err := h.db.QueryRow(ctx, `SELECT subscription_plan FROM organizations WHERE id = $1`, orgID).Scan(&plan)
	if err != nil {
		return "free", err
	}
	return plan, nil
}

func (h *Handler) countOrgVideosThisMonth(ctx context.Context, orgID string) (int, error) {
	var count int
	err := h.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM videos WHERE organization_id = $1 AND status != 'deleted' AND created_at >= date_trunc('month', now())`,
		orgID,
	).Scan(&count)
	return count, err
}

func computeSummary(daily []dailyViews, todayStr string) analyticsSummary {
	var summary analyticsSummary
	for _, d := range daily {
		summary.TotalViews += d.Views
		summary.UniqueViews += d.UniqueViews
		if d.Date == todayStr {
			summary.ViewsToday = d.Views
		}
		if d.Views > summary.PeakDayViews {
			summary.PeakDayViews = d.Views
			summary.PeakDay = d.Date
		}
	}
	if len(daily) > 0 && summary.TotalViews > 0 {
		avg := float64(summary.TotalViews) / float64(len(daily))
		summary.AverageDailyViews = math.Round(avg*10) / 10
	}
	return summary
}

func sortDailyViews(daily []dailyViews) {
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].Date < daily[j].Date
	})
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var title string
	var fileKey string
	var contentType string
	err := h.db.QueryRow(r.Context(),
		`SELECT title, file_key, content_type FROM videos WHERE id = $1 AND user_id = $2 AND status = 'ready'`,
		videoID, userID,
	).Scan(&title, &fileKey, &contentType)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	filename := title + extensionForContentType(contentType)
	downloadURL, err := h.storage.GenerateDownloadURLWithDisposition(r.Context(), fileKey, filename, 1*time.Hour)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate download URL")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"downloadUrl": downloadURL})
}

func (h *Handler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var status string
	var segmentsJSON *string
	err := h.db.QueryRow(r.Context(),
		`SELECT transcript_status, transcript_json FROM videos WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	).Scan(&status, &segmentsJSON)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	segments := make([]TranscriptSegment, 0)
	if segmentsJSON != nil {
		_ = json.Unmarshal([]byte(*segmentsJSON), &segments)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"segments": segments,
	})
}

func (h *Handler) WatchDownload(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var title string
	var fileKey string
	var shareExpiresAt *time.Time
	var sharePassword *string
	var contentType string
	var downloadEnabled bool

	err := h.db.QueryRow(r.Context(),
		`SELECT title, file_key, share_expires_at, share_password, content_type, download_enabled FROM videos WHERE share_token = $1 AND status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&title, &fileKey, &shareExpiresAt, &sharePassword, &contentType, &downloadEnabled)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if !downloadEnabled {
		httputil.WriteError(w, http.StatusForbidden, "downloads are disabled for this video")
		return
	}

	if shareExpiresAt != nil && time.Now().After(*shareExpiresAt) {
		httputil.WriteError(w, http.StatusGone, "link expired")
		return
	}

	if sharePassword != nil {
		if !hasValidWatchCookie(r, h.hmacSecret, shareToken, *sharePassword) {
			httputil.WriteError(w, http.StatusForbidden, "password required")
			return
		}
	}

	filename := title + extensionForContentType(contentType)
	downloadURL, err := h.storage.GenerateDownloadURLWithDisposition(r.Context(), fileKey, filename, 1*time.Hour)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate download URL")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"downloadUrl": downloadURL})
}
