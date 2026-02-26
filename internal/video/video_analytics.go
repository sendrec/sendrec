package video

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/webhook"
)

type analyticsSummary struct {
	TotalViews        int64   `json:"totalViews"`
	UniqueViews       int64   `json:"uniqueViews"`
	ViewsToday        int64   `json:"viewsToday"`
	AverageDailyViews float64 `json:"averageDailyViews"`
	PeakDay           string  `json:"peakDay"`
	PeakDayViews      int64   `json:"peakDayViews"`
	TotalCtaClicks    int64   `json:"totalCtaClicks"`
	CtaClickRate      float64 `json:"ctaClickRate"`
}

type dailyViews struct {
	Date        string `json:"date"`
	Views       int64  `json:"views"`
	UniqueViews int64  `json:"uniqueViews"`
}

type milestoneCounts struct {
	Reached25  int64 `json:"reached25"`
	Reached50  int64 `json:"reached50"`
	Reached75  int64 `json:"reached75"`
	Reached100 int64 `json:"reached100"`
}

type viewerInfo struct {
	Email            string `json:"email"`
	FirstViewedAt    string `json:"firstViewedAt"`
	ViewCount        int64  `json:"viewCount"`
	Completion       int    `json:"completion"`
	WatchTimeSeconds int64  `json:"watchTimeSeconds"`
	Country          string `json:"country"`
	City             string `json:"city"`
}

type segmentData struct {
	Segment    int     `json:"segment"`
	WatchCount int64   `json:"watchCount"`
	Intensity  float64 `json:"intensity"`
}

type trendData struct {
	Views          *float64 `json:"views"`
	UniqueViews    *float64 `json:"uniqueViews"`
	AvgWatchTime   *float64 `json:"avgWatchTime"`
	CompletionRate *float64 `json:"completionRate"`
}

type referrerData struct {
	Source     string  `json:"source"`
	Count      int64  `json:"count"`
	Percentage float64 `json:"percentage"`
}

type breakdownItem struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
}

type analyticsResponse struct {
	Summary    analyticsSummary `json:"summary"`
	Daily      []dailyViews     `json:"daily"`
	Milestones milestoneCounts  `json:"milestones"`
	Viewers    []viewerInfo     `json:"viewers"`
	Heatmap    []segmentData    `json:"heatmap"`
	Trends     *trendData       `json:"trends,omitempty"`
	Referrers  []referrerData   `json:"referrers"`
	Browsers   []breakdownItem  `json:"browsers"`
	Devices    []breakdownItem  `json:"devices"`
}

type milestoneRequest struct {
	Milestone int `json:"milestone"`
}

type segmentsRequest struct {
	Segments []int `json:"segments"`
}

func (h *Handler) RecordCTAClick(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var videoID string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM videos WHERE share_token = $1 AND status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ip := clientIP(r)
		hash := viewerHash(ip, r.UserAgent())
		if _, err := h.db.Exec(ctx,
			`INSERT INTO cta_clicks (video_id, viewer_hash) VALUES ($1, $2)`,
			videoID, hash,
		); err != nil {
			slog.Error("video: failed to record CTA click", "video_id", videoID, "error", err)
		}
		if h.webhookClient != nil {
			var ownerID, ctaTitle string
			if err := h.db.QueryRow(ctx,
				`SELECT user_id, title FROM videos WHERE id = $1`, videoID,
			).Scan(&ownerID, &ctaTitle); err == nil {
				wURL, wSecret, wErr := h.webhookClient.LookupConfigByUserID(ctx, ownerID)
				if wErr == nil {
					if err := h.webhookClient.Dispatch(ctx, ownerID, wURL, wSecret, webhook.Event{
						Name:      "video.cta_click",
						Timestamp: time.Now().UTC(),
						Data: map[string]any{
							"videoId":    videoID,
							"title":      ctaTitle,
							"viewerHash": hash,
						},
					}); err != nil {
						slog.Error("webhook: dispatch failed for video.cta_click", "video_id", videoID, "error", err)
					}
				}
			}
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RecordMilestone(w http.ResponseWriter, r *http.Request) {
	var req milestoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Milestone != 25 && req.Milestone != 50 && req.Milestone != 75 && req.Milestone != 100 {
		httputil.WriteError(w, http.StatusBadRequest, "milestone must be 25, 50, 75, or 100")
		return
	}

	shareToken := chi.URLParam(r, "shareToken")

	var videoID string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM videos WHERE share_token = $1 AND status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ip := clientIP(r)
		hash := viewerHash(ip, r.UserAgent())
		if _, err := h.db.Exec(ctx,
			`INSERT INTO view_milestones (video_id, viewer_hash, milestone) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			videoID, hash, req.Milestone,
		); err != nil {
			slog.Error("video: failed to record milestone", "video_id", videoID, "error", err)
		}
		if h.webhookClient != nil {
			var ownerID, milestoneTitle string
			if err := h.db.QueryRow(ctx,
				`SELECT user_id, title FROM videos WHERE id = $1`, videoID,
			).Scan(&ownerID, &milestoneTitle); err == nil {
				wURL, wSecret, wErr := h.webhookClient.LookupConfigByUserID(ctx, ownerID)
				if wErr == nil {
					if err := h.webhookClient.Dispatch(ctx, ownerID, wURL, wSecret, webhook.Event{
						Name:      "video.milestone",
						Timestamp: time.Now().UTC(),
						Data: map[string]any{
							"videoId":    videoID,
							"title":      milestoneTitle,
							"milestone":  req.Milestone,
							"viewerHash": hash,
						},
					}); err != nil {
						slog.Error("webhook: dispatch failed for video.milestone", "video_id", videoID, "error", err)
					}
				}
			}
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RecordSegments(w http.ResponseWriter, r *http.Request) {
	var req segmentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Segments) == 0 || len(req.Segments) > 50 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	shareToken := chi.URLParam(r, "shareToken")

	var videoID string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM videos WHERE share_token = $1 AND status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, seg := range req.Segments {
			if seg < 0 || seg >= 50 {
				continue
			}
			if _, err := h.db.Exec(ctx,
				`INSERT INTO segment_engagement (video_id, segment_index, watch_count)
				 VALUES ($1, $2, 1)
				 ON CONFLICT (video_id, segment_index)
				 DO UPDATE SET watch_count = segment_engagement.watch_count + 1`,
				videoID, seg,
			); err != nil {
				slog.Error("video: failed to record segment", "video_id", videoID, "segment", seg, "error", err)
			}
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Analytics(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var id string
	var emailGateEnabled bool
	err := h.db.QueryRow(r.Context(),
		`SELECT id, email_gate_enabled FROM videos WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	).Scan(&id, &emailGateEnabled)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "7d"
	}

	var days int
	switch rangeParam {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "all":
		days = 0
	default:
		httputil.WriteError(w, http.StatusBadRequest, "invalid range: must be 7d, 30d, 90d, or all")
		return
	}

	now := time.Now().UTC().Truncate(24 * time.Hour)
	var since time.Time
	if days > 0 {
		since = now.AddDate(0, 0, -(days - 1))
	} else {
		since = time.Time{}
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT date_trunc('day', created_at) AS day, COUNT(*) AS views, COUNT(DISTINCT viewer_hash) AS unique_views
		 FROM video_views WHERE video_id = $1 AND created_at >= $2
		 GROUP BY day ORDER BY day`,
		videoID, since,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to query analytics")
		return
	}
	defer rows.Close()

	dataByDate := make(map[string]dailyViews)
	for rows.Next() {
		var day time.Time
		var views, uniqueViews int64
		if err := rows.Scan(&day, &views, &uniqueViews); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan analytics")
			return
		}
		dateStr := day.Format("2006-01-02")
		dataByDate[dateStr] = dailyViews{
			Date:        dateStr,
			Views:       views,
			UniqueViews: uniqueViews,
		}
	}
	if err := rows.Err(); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to read analytics")
		return
	}

	daily := make([]dailyViews, 0)
	if days > 0 {
		for i := days - 1; i >= 0; i-- {
			d := now.AddDate(0, 0, -i)
			dateStr := d.Format("2006-01-02")
			if entry, ok := dataByDate[dateStr]; ok {
				daily = append(daily, entry)
			} else {
				daily = append(daily, dailyViews{Date: dateStr})
			}
		}
	} else {
		for _, entry := range dataByDate {
			daily = append(daily, entry)
		}
		sortDailyViews(daily)
	}

	var totalCtaClicks int64
	err = h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM cta_clicks WHERE video_id = $1 AND created_at >= $2`,
		videoID, since,
	).Scan(&totalCtaClicks)
	if err != nil {
		totalCtaClicks = 0
	}

	var milestones milestoneCounts
	milestoneRows, err := h.db.Query(r.Context(),
		`SELECT milestone, COUNT(DISTINCT viewer_hash) FROM view_milestones WHERE video_id = $1 AND created_at >= $2 GROUP BY milestone`,
		videoID, since,
	)
	if err == nil {
		defer milestoneRows.Close()
		for milestoneRows.Next() {
			var m int
			var count int64
			if err := milestoneRows.Scan(&m, &count); err == nil {
				switch m {
				case 25:
					milestones.Reached25 = count
				case 50:
					milestones.Reached50 = count
				case 75:
					milestones.Reached75 = count
				case 100:
					milestones.Reached100 = count
				}
			}
		}
	}

	viewers := make([]viewerInfo, 0)
	if emailGateEnabled {
		viewerRows, err := h.db.Query(r.Context(),
			`SELECT vv.email, vv.created_at,
			        COUNT(views.id) AS view_count,
			        COALESCE(MAX(vm.milestone), 0) AS completion,
			        COALESCE(MAX(views.country), '') AS country,
			        COALESCE(MAX(views.city), '') AS city
			 FROM video_viewers vv
			 LEFT JOIN video_views views ON views.video_id = vv.video_id AND views.viewer_hash = vv.viewer_hash
			 LEFT JOIN view_milestones vm ON vm.video_id = vv.video_id AND vm.viewer_hash = vv.viewer_hash
			 WHERE vv.video_id = $1
			 GROUP BY vv.email, vv.created_at
			 ORDER BY vv.created_at DESC`,
			videoID,
		)
		if err == nil {
			defer viewerRows.Close()
			for viewerRows.Next() {
				var vi viewerInfo
				var createdAt time.Time
				if err := viewerRows.Scan(&vi.Email, &createdAt, &vi.ViewCount, &vi.Completion, &vi.Country, &vi.City); err == nil {
					vi.FirstViewedAt = createdAt.Format(time.RFC3339)
					viewers = append(viewers, vi)
				}
			}
		}
	}

	summary := computeSummary(daily, now.Format("2006-01-02"))
	summary.TotalCtaClicks = totalCtaClicks
	if summary.TotalViews > 0 {
		summary.CtaClickRate = float64(totalCtaClicks) / float64(summary.TotalViews)
	}

	heatmap := make([]segmentData, 0, 50)
	segRows, err := h.db.Query(r.Context(),
		`SELECT segment_index, watch_count FROM segment_engagement
		 WHERE video_id = $1 ORDER BY segment_index`,
		videoID,
	)
	if err == nil {
		defer segRows.Close()
		var maxCount int64
		var segments []segmentData
		for segRows.Next() {
			var sd segmentData
			if err := segRows.Scan(&sd.Segment, &sd.WatchCount); err == nil {
				segments = append(segments, sd)
				if sd.WatchCount > maxCount {
					maxCount = sd.WatchCount
				}
			}
		}
		for i := range segments {
			if maxCount > 0 {
				segments[i].Intensity = float64(segments[i].WatchCount) / float64(maxCount)
			}
		}
		heatmap = segments
	}

	referrers := make([]referrerData, 0)
	refRows, err := h.db.Query(r.Context(),
		`SELECT referrer, COUNT(*) AS cnt
		 FROM video_views WHERE video_id = $1 AND created_at >= $2
		 GROUP BY referrer ORDER BY cnt DESC`,
		videoID, since,
	)
	if err == nil {
		defer refRows.Close()
		for refRows.Next() {
			var rd referrerData
			if err := refRows.Scan(&rd.Source, &rd.Count); err == nil {
				referrers = append(referrers, rd)
			}
		}
		var total int64
		for _, rd := range referrers {
			total += rd.Count
		}
		if total > 0 {
			for i := range referrers {
				referrers[i].Percentage = float64(referrers[i].Count) / float64(total) * 100
			}
		}
	}

	browsers := make([]breakdownItem, 0)
	browserRows, err := h.db.Query(r.Context(),
		`SELECT browser, COUNT(*) AS cnt
		 FROM video_views WHERE video_id = $1 AND created_at >= $2
		 GROUP BY browser ORDER BY cnt DESC`,
		videoID, since,
	)
	if err == nil {
		defer browserRows.Close()
		var browserTotal int64
		var rawBrowsers []breakdownItem
		type countedItem struct {
			name  string
			count int64
		}
		var browserCounts []countedItem
		for browserRows.Next() {
			var name string
			var count int64
			if err := browserRows.Scan(&name, &count); err == nil {
				browserCounts = append(browserCounts, countedItem{name, count})
				browserTotal += count
			}
		}
		if browserTotal > 0 {
			for _, bc := range browserCounts {
				rawBrowsers = append(rawBrowsers, breakdownItem{
					Name:       bc.name,
					Percentage: math.Round(float64(bc.count)/float64(browserTotal)*1000) / 10,
				})
			}
		}
		browsers = rawBrowsers
	}
	if browsers == nil {
		browsers = make([]breakdownItem, 0)
	}

	devices := make([]breakdownItem, 0)
	deviceRows, err := h.db.Query(r.Context(),
		`SELECT device, COUNT(*) AS cnt
		 FROM video_views WHERE video_id = $1 AND created_at >= $2
		 GROUP BY device ORDER BY cnt DESC`,
		videoID, since,
	)
	if err == nil {
		defer deviceRows.Close()
		var deviceTotal int64
		var rawDevices []breakdownItem
		type countedItem struct {
			name  string
			count int64
		}
		var deviceCounts []countedItem
		for deviceRows.Next() {
			var name string
			var count int64
			if err := deviceRows.Scan(&name, &count); err == nil {
				deviceCounts = append(deviceCounts, countedItem{name, count})
				deviceTotal += count
			}
		}
		if deviceTotal > 0 {
			for _, dc := range deviceCounts {
				rawDevices = append(rawDevices, breakdownItem{
					Name:       dc.name,
					Percentage: math.Round(float64(dc.count)/float64(deviceTotal)*1000) / 10,
				})
			}
		}
		devices = rawDevices
	}
	if devices == nil {
		devices = make([]breakdownItem, 0)
	}

	var trends *trendData
	if days > 0 && summary.TotalViews > 0 {
		prevSince := since.AddDate(0, 0, -days)
		var prevViews, prevUnique int64
		_ = h.db.QueryRow(r.Context(),
			`SELECT COUNT(*), COUNT(DISTINCT viewer_hash)
			 FROM video_views WHERE video_id = $1 AND created_at >= $2 AND created_at < $3`,
			videoID, prevSince, since,
		).Scan(&prevViews, &prevUnique)

		trends = &trendData{}
		if prevViews > 0 {
			v := (float64(summary.TotalViews) - float64(prevViews)) / float64(prevViews) * 100
			trends.Views = &v
		}
		if prevUnique > 0 {
			v := (float64(summary.UniqueViews) - float64(prevUnique)) / float64(prevUnique) * 100
			trends.UniqueViews = &v
		}
	}

	httputil.WriteJSON(w, http.StatusOK, analyticsResponse{
		Summary:    summary,
		Daily:      daily,
		Milestones: milestones,
		Viewers:    viewers,
		Heatmap:    heatmap,
		Trends:     trends,
		Referrers:  referrers,
		Browsers:   browsers,
		Devices:    devices,
	})
}

func (h *Handler) AnalyticsExport(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var id string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM videos WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	).Scan(&id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "7d"
	}
	var days int
	switch rangeParam {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "all":
		days = 0
	default:
		httputil.WriteError(w, http.StatusBadRequest, "invalid range")
		return
	}

	now := time.Now().UTC().Truncate(24 * time.Hour)
	var since time.Time
	if days > 0 {
		since = now.AddDate(0, 0, -(days - 1))
	} else {
		since = time.Time{}
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT date_trunc('day', created_at)::date AS day, COUNT(*) AS views, COUNT(DISTINCT viewer_hash) AS unique_views
		 FROM video_views WHERE video_id = $1 AND created_at >= $2
		 GROUP BY day ORDER BY day`,
		videoID, since,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to query")
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=analytics.csv")
	fmt.Fprintln(w, "Date,Views,Unique Views")
	for rows.Next() {
		var day time.Time
		var views, uv int64
		if err := rows.Scan(&day, &views, &uv); err == nil {
			fmt.Fprintf(w, "%s,%d,%d\n", day.Format("2006-01-02"), views, uv)
		}
	}
}
