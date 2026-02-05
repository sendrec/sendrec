package video

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

var watchPageTemplate = template.Must(template.New("watch").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} — SendRec</title>
    <meta property="og:title" content="{{.Title}}">
    <meta property="og:type" content="video.other">
    <meta property="og:video" content="{{.VideoURL}}">
    <meta property="og:video:type" content="video/webm">
    <meta property="og:site_name" content="SendRec">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: #0a1628;
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
        }
        .container {
            max-width: 960px;
            width: 100%;
            padding: 2rem 1rem;
        }
        video {
            width: 100%;
            border-radius: 8px;
            background: #000;
        }
        h1 {
            margin-top: 1rem;
            font-size: 1.5rem;
            font-weight: 600;
        }
        .meta {
            margin-top: 0.5rem;
            color: #94a3b8;
            font-size: 0.875rem;
        }
        .branding {
            margin-top: 2rem;
            font-size: 0.75rem;
            color: #64748b;
        }
        .branding a {
            color: #00b67a;
            text-decoration: none;
        }
        .branding a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <video id="player" controls>
            <source src="{{.VideoURL}}" type="video/webm">
            Your browser does not support video playback.
        </video>
        <script>
            var v = document.getElementById('player');
            v.play().catch(function() { v.muted = true; v.play(); });
        </script>
        <h1>{{.Title}}</h1>
        <p class="meta">{{.Creator}} · {{.Date}}</p>
        <p class="branding">Shared via <a href="https://sendrec.eu">SendRec</a> — open-source video messaging</p>
    </div>
</body>
</html>`))

type watchPageData struct {
	Title    string
	VideoURL string
	Creator  string
	Date     string
}

func (h *Handler) WatchPage(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var title string
	var fileKey string
	var creator string
	var createdAt time.Time

	err := h.db.QueryRow(r.Context(),
		`SELECT v.title, v.file_key, u.name, v.created_at
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.share_token = $1 AND v.status = 'ready'`,
		shareToken,
	).Scan(&title, &fileKey, &creator, &createdAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	videoURL, err := h.storage.GenerateDownloadURL(r.Context(), fileKey, 1*time.Hour)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := watchPageTemplate.Execute(w, watchPageData{
		Title:    title,
		VideoURL: videoURL,
		Creator:  creator,
		Date:     createdAt.Format("Jan 2, 2006"),
	}); err != nil {
		log.Printf("failed to render watch page: %v", err)
	}
}
