package video

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
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
    {{if .ThumbnailURL}}<meta property="og:image" content="{{.ThumbnailURL}}">{{end}}
    <meta property="og:site_name" content="SendRec">
    <style nonce="{{.Nonce}}">
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
        .actions {
            margin-top: 1rem;
        }
        .download-btn {
            display: inline-block;
            background: transparent;
            color: #00b67a;
            border: 1px solid #00b67a;
            padding: 0.5rem 1.25rem;
            border-radius: 6px;
            font-size: 0.875rem;
            font-weight: 600;
            cursor: pointer;
            text-decoration: none;
        }
        .download-btn:hover {
            background: rgba(0, 182, 122, 0.1);
        }
    </style>
</head>
<body>
    <div class="container">
        <video id="player" controls{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}>
            <source src="{{.VideoURL}}" type="video/webm">
            Your browser does not support video playback.
        </video>
        <script nonce="{{.Nonce}}">
            var v = document.getElementById('player');
            v.play().catch(function() { v.muted = true; v.play(); });
        </script>
        <h1>{{.Title}}</h1>
        <p class="meta">{{.Creator}} · {{.Date}}</p>
        <div class="actions">
            <button class="download-btn" id="download-btn">Download</button>
        </div>
        <script nonce="{{.Nonce}}">
            document.getElementById('download-btn').addEventListener('click', function() {
                fetch('/api/watch/{{.ShareToken}}/download')
                    .then(function(r) { return r.json(); })
                    .then(function(data) { if (data.downloadUrl) window.location.href = data.downloadUrl; });
            });
        </script>
        <p class="branding">Shared via <a href="https://sendrec.eu">SendRec</a> — open-source video messaging</p>
    </div>
</body>
</html>`))

var expiredPageTemplate = template.Must(template.New("expired").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Link Expired — SendRec</title>
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: #0a1628;
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container { text-align: center; padding: 2rem; }
        h1 { font-size: 1.5rem; margin-bottom: 0.75rem; }
        p { color: #94a3b8; margin-bottom: 1.5rem; }
        a {
            display: inline-block;
            background: #00b67a;
            color: #fff;
            padding: 0.625rem 1.5rem;
            border-radius: 8px;
            text-decoration: none;
            font-weight: 600;
        }
        a:hover { opacity: 0.9; }
    </style>
</head>
<body>
    <div class="container">
        <h1>This link has expired</h1>
        <p>The video owner can extend the link to make it available again.</p>
        <a href="https://sendrec.eu">Go to SendRec</a>
    </div>
</body>
</html>`))

type watchPageData struct {
	Title        string
	VideoURL     string
	Creator      string
	Date         string
	Nonce        string
	ThumbnailURL string
	ShareToken   string
}

type expiredPageData struct {
	Nonce string
}

type passwordPageData struct {
	Title      string
	ShareToken string
	Nonce      string
}

var passwordPageTemplate = template.Must(template.New("password").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} — SendRec</title>
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: #0a1628;
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container { text-align: center; padding: 2rem; max-width: 400px; width: 100%; }
        h1 { font-size: 1.5rem; margin-bottom: 0.75rem; }
        p { color: #94a3b8; margin-bottom: 1.5rem; }
        .error { color: #ef4444; font-size: 0.875rem; margin-bottom: 1rem; display: none; }
        input[type="password"] {
            width: 100%;
            padding: 0.75rem 1rem;
            border-radius: 8px;
            border: 1px solid #334155;
            background: #1e293b;
            color: #fff;
            font-size: 1rem;
            margin-bottom: 1rem;
            outline: none;
        }
        input[type="password"]:focus { border-color: #00b67a; }
        button {
            width: 100%;
            background: #00b67a;
            color: #fff;
            padding: 0.75rem 1.5rem;
            border: none;
            border-radius: 8px;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
        }
        button:hover { opacity: 0.9; }
        button:disabled { opacity: 0.5; cursor: not-allowed; }
    </style>
</head>
<body>
    <div class="container">
        <h1>This video is password protected</h1>
        <p>Enter the password to watch this video.</p>
        <p class="error" id="error-msg"></p>
        <form id="password-form">
            <input type="password" id="password-input" placeholder="Password" required autofocus>
            <button type="submit" id="submit-btn">Watch Video</button>
        </form>
    </div>
    <script nonce="{{.Nonce}}">
        document.getElementById('password-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var btn = document.getElementById('submit-btn');
            var errEl = document.getElementById('error-msg');
            var pw = document.getElementById('password-input').value;
            btn.disabled = true;
            errEl.style.display = 'none';
            fetch('/api/watch/{{.ShareToken}}/verify', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({password: pw})
            }).then(function(r) {
                if (r.ok) { window.location.reload(); }
                else { errEl.textContent = 'Incorrect password'; errEl.style.display = 'block'; btn.disabled = false; }
            }).catch(function() {
                errEl.textContent = 'Something went wrong'; errEl.style.display = 'block'; btn.disabled = false;
            });
        });
    </script>
</body>
</html>`))

func (h *Handler) WatchPage(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var title string
	var fileKey string
	var creator string
	var createdAt time.Time
	var shareExpiresAt time.Time
	var thumbnailKey *string
	var sharePassword *string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key, v.share_password
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.share_token = $1 AND v.status = 'ready'`,
		shareToken,
	).Scan(&title, &fileKey, &creator, &createdAt, &shareExpiresAt, &thumbnailKey, &sharePassword)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	nonce := httputil.NonceFromContext(r.Context())

	if time.Now().After(shareExpiresAt) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		if err := expiredPageTemplate.Execute(w, expiredPageData{Nonce: nonce}); err != nil {
			log.Printf("failed to render expired page: %v", err)
		}
		return
	}

	if sharePassword != nil {
		if !hasValidWatchCookie(r, h.hmacSecret, shareToken, *sharePassword) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := passwordPageTemplate.Execute(w, passwordPageData{
				Title:      title,
				ShareToken: shareToken,
				Nonce:      nonce,
			}); err != nil {
				log.Printf("failed to render password page: %v", err)
			}
			return
		}
	}

	videoURL, err := h.storage.GenerateDownloadURL(r.Context(), fileKey, 1*time.Hour)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var thumbnailURL string
	if thumbnailKey != nil {
		if u, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour); err == nil {
			thumbnailURL = u
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := watchPageTemplate.Execute(w, watchPageData{
		Title:        title,
		VideoURL:     videoURL,
		Creator:      creator,
		Date:         createdAt.Format("Jan 2, 2006"),
		Nonce:        nonce,
		ThumbnailURL: thumbnailURL,
		ShareToken:   shareToken,
	}); err != nil {
		log.Printf("failed to render watch page: %v", err)
	}
}
