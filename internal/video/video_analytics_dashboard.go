package video

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

type dashboardSummary struct {
	TotalViews           int64   `json:"totalViews"`
	UniqueViews          int64   `json:"uniqueViews"`
	AvgDailyViews        float64 `json:"avgDailyViews"`
	TotalVideos          int64   `json:"totalVideos"`
	TotalWatchTimeSeconds int64  `json:"totalWatchTimeSeconds"`
}

type dashboardDaily struct {
	Date        string `json:"date"`
	Views       int64  `json:"views"`
	UniqueViews int64  `json:"uniqueViews"`
}

type dashboardTopVideo struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Views        int64  `json:"views"`
	UniqueViews  int64  `json:"uniqueViews"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

type dashboardResponse struct {
	Summary   dashboardSummary    `json:"summary"`
	Daily     []dashboardDaily    `json:"daily"`
	TopVideos []dashboardTopVideo `json:"topVideos"`
}

func (h *Handler) AnalyticsDashboard(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

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
		since = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	var totalViews, uniqueViews int64
	err := h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) AS views, COUNT(DISTINCT viewer_hash) AS unique_views
		 FROM video_views vv
		 JOIN videos v ON v.id = vv.video_id
		 WHERE v.user_id = $1 AND vv.created_at >= $2`,
		userID, since,
	).Scan(&totalViews, &uniqueViews)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to query view summary")
		return
	}

	var totalVideos int64
	err = h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM videos WHERE user_id = $1`,
		userID,
	).Scan(&totalVideos)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to count videos")
		return
	}

	var totalWatchTimeSeconds int64
	err = h.db.QueryRow(r.Context(),
		`SELECT COALESCE(SUM(v.duration), 0)
		 FROM video_views vv
		 JOIN videos v ON v.id = vv.video_id
		 WHERE v.user_id = $1 AND vv.created_at >= $2`,
		userID, since,
	).Scan(&totalWatchTimeSeconds)
	if err != nil {
		totalWatchTimeSeconds = 0
	}

	daysInRange := days
	if daysInRange == 0 {
		daysInRange = int(now.Sub(since).Hours()/24) + 1
	}
	if daysInRange < 1 {
		daysInRange = 1
	}
	avgDailyViews := math.Round(float64(totalViews)/float64(daysInRange)*10) / 10

	rows, err := h.db.Query(r.Context(),
		`SELECT date_trunc('day', vv.created_at)::date AS day,
		        COUNT(*) AS views,
		        COUNT(DISTINCT vv.viewer_hash) AS unique_views
		 FROM video_views vv
		 JOIN videos v ON v.id = vv.video_id
		 WHERE v.user_id = $1 AND vv.created_at >= $2
		 GROUP BY day ORDER BY day`,
		userID, since,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to query daily views")
		return
	}
	defer rows.Close()

	dataByDate := make(map[string]dashboardDaily)
	for rows.Next() {
		var day time.Time
		var views, uv int64
		if err := rows.Scan(&day, &views, &uv); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan daily views")
			return
		}
		dateStr := day.Format("2006-01-02")
		dataByDate[dateStr] = dashboardDaily{
			Date:        dateStr,
			Views:       views,
			UniqueViews: uv,
		}
	}
	if err := rows.Err(); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to read daily views")
		return
	}

	daily := make([]dashboardDaily, 0)
	if days > 0 {
		for i := days - 1; i >= 0; i-- {
			d := now.AddDate(0, 0, -i)
			dateStr := d.Format("2006-01-02")
			if entry, ok := dataByDate[dateStr]; ok {
				daily = append(daily, entry)
			} else {
				daily = append(daily, dashboardDaily{Date: dateStr})
			}
		}
	} else {
		for _, entry := range dataByDate {
			daily = append(daily, entry)
		}
		sortDashboardDaily(daily)
	}

	topRows, err := h.db.Query(r.Context(),
		`SELECT v.id, v.title, COUNT(vv.id) AS views,
		        COUNT(DISTINCT vv.viewer_hash) AS unique_views,
		        v.share_token,
		        CASE WHEN v.thumbnail_key IS NOT NULL AND v.thumbnail_key != '' THEN true ELSE false END AS has_thumbnail
		 FROM videos v
		 LEFT JOIN video_views vv ON vv.video_id = v.id AND vv.created_at >= $2
		 WHERE v.user_id = $1 AND v.status != 'deleted'
		 GROUP BY v.id, v.title, v.share_token, v.thumbnail_key
		 ORDER BY views DESC
		 LIMIT 10`,
		userID, since,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to query top videos")
		return
	}
	defer topRows.Close()

	topVideos := make([]dashboardTopVideo, 0, 10)
	for topRows.Next() {
		var id, title, shareToken string
		var views, uv int64
		var hasThumbnail bool
		if err := topRows.Scan(&id, &title, &views, &uv, &shareToken, &hasThumbnail); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan top videos")
			return
		}
		thumbnailURL := ""
		if hasThumbnail {
			thumbnailURL = fmt.Sprintf("/api/watch/%s/thumbnail", shareToken)
		}
		topVideos = append(topVideos, dashboardTopVideo{
			ID:           id,
			Title:        title,
			Views:        views,
			UniqueViews:  uv,
			ThumbnailURL: thumbnailURL,
		})
	}
	if err := topRows.Err(); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to read top videos")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, dashboardResponse{
		Summary: dashboardSummary{
			TotalViews:            totalViews,
			UniqueViews:           uniqueViews,
			AvgDailyViews:         avgDailyViews,
			TotalVideos:           totalVideos,
			TotalWatchTimeSeconds: totalWatchTimeSeconds,
		},
		Daily:     daily,
		TopVideos: topVideos,
	})
}

func sortDashboardDaily(daily []dashboardDaily) {
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].Date < daily[j].Date
	})
}
