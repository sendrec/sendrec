package video

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

type embedPageData struct {
	Title        string
	VideoURL     string
	ThumbnailURL string
	ShareToken   string
	Nonce        string
	BaseURL      string
	ContentType  string
	CtaText      string
	CtaUrl       string
}

type embedPasswordPageData struct {
	Title      string
	ShareToken string
	Nonce      string
}

var embedPageTemplate = template.Must(template.New("embed").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="icon" type="image/png" sizes="32x32" href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAABGdBTUEAALGPC/xhBQAAACBjSFJNAAB6JgAAgIQAAPoAAACA6AAAdTAAAOpgAAA6mAAAF3CculE8AAAAeGVYSWZNTQAqAAAACAAEARoABQAAAAEAAAA+ARsABQAAAAEAAABGASgAAwAAAAEAAgAAh2kABAAAAAEAAABOAAAAAAAAAEgAAAABAAAASAAAAAEAA6ABAAMAAAABAAEAAKACAAQAAAABAAAAIKADAAQAAAABAAAAIAAAAACfCVbEAAAACXBIWXMAAAsTAAALEwEAmpwYAAAEa0lEQVRYCe1VW2icRRQ+Z+af/xK1kdQlvlhatWC7tamKoIilVrFSjdhoIqZtQBERn7RKXwQJvigUqy8iIQ9KU4Msrq0+eHlJUChiIWLTpFHrgxAkIiE2t+a/zMzxzCYb/t3ECyj4spNsZnbmzPm+850zJwCN0VCgocD/rAD+V/i3fFoqxHHc6ilPo68nR/f1LP4T3/+aQLF88nbjyaNkabew1AKIBqSYJKKSDOUb4w90zfwVkb8lsP2TwSKCuMuQ3miF6P+xvXu66nBb+cRhUOodkN4VNssA7cqJFICBAkrTc5jajonHDl6s3qmf/5TAztKJLZmvXrOCHhZSRSIIwC7MHZt45NBR52Rn+eQdqSeHgCACY4AjB4684p/sMhPh+0BJ8m1zCPd8s//QXD24+758o+6k+NFAWxqqIQjU42ApsmkKZn7BmT+7/cP3is48ldALSkbE4OSA3ceaKbL6EipZ8WiTFDCKbp2N4bnKxjp/1hDYderdqw2I91nWzXYpYaccnJCAygPZvOEqQnFgx8eDrUS4G1LNgXOCUGhp9ItKYdHzqA20KaNSFTjKKjade4Z7vXXwYc1mTN5TGAXFCjirip4HmGVjwsCItXZOePgBoS4KGUTgnDMQ6fTr8Y7Dx1cAfucgXoo17kOJV7Iqbvv6+ZkbCjxPrdisTjUEOkslOYZpFxkLSMTOfUCTDXKJPZN/VjeVB/bKQHD62UYwS8KJVY+8iEnMA1EKwglMTkUv9mlZkrwhr2tS8IMXb7QEN6JmAk52qxcFyVfy4O6+QNziao5/XX7cPOmW1SGF9zSnrAWI/VR+aAaWwnWfY40CRkOIEgJXxS63/LZnm8Xib1XH1dkibHJr6/LP4bHl9zvKAw9SGB7kgi0Ywr3uZTiC7jlCbL4af7TLVfGaUaOAyOQ8a7/AQbFyjoQozKb+zflbe4aHPY5ql0uTG6S5Dsj+zGQewih8Ajz/PkZmXiw9p4CyLGYt38r7yK8rKq5ucEq3nR74HP3wfuInxE0GUOtzYLInizYavQBzzaSil8nzjjjgivxkLjUtia1JqAtGqrPcAbnwHDizkDLFLDky3tHz9ipG3aJGAaelsNjvCrCSYa5yfo5tVnhnRlU6YlX4HTA4OHCHEfggLHwx0t09PdbRM4HW9EluPq4qBHFsxsSC9Gd1mDVfawnwUeH8T6etTk9hU1DhQDpjJIgkE0Epr3PgyPJLfp6QJNOCst6qRyHEMZskU64pkUuhUhsI1KvV8/Xm2hSsWBRLpRbrJwOciv3cVMDyx4W80nQYXPFe9gvZrIflHco75n9Oz0MYvkkpNzH2zqQMarr3fEf3l3m76nqNAu5gvKtrRqTBAUySF7gYL7Ajjd5ye2VlfzU67Rdpdnc9uLsrF22/TZZGXed0BMCTksC+fltfX5M7rx/rKpA3urN0PJoPrt3K+WwliZelsRdHO3rWPM38He6Em43ftEnHS8BpI5FpGYTXnB1pb7+ct2usGwo0FGgo4BT4A0kx06ZKzSjiAAAAAElFTkSuQmCC">
    <title>{{.Title}}</title>
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        html, body { width: 100%; height: 100%; overflow: hidden; background: #0f172a; }
        .container {
            display: flex;
            flex-direction: column;
            width: 100%;
            height: 100%;
            position: relative;
        }
        .video-wrapper {
            flex: 1;
            min-height: 0;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #000;
        }
        video {
            width: 100%;
            height: 100%;
            object-fit: contain;
        }
        .footer {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 8px 12px;
            background: #1e293b;
            color: #e2e8f0;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            font-size: 13px;
        }
        .footer-title {
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            margin-right: 12px;
        }
        .footer a {
            color: #94a3b8;
            text-decoration: none;
            white-space: nowrap;
            font-size: 12px;
        }
        .footer a:hover { color: #e2e8f0; }
        .cta-overlay { display: none; position: absolute; bottom: 48px; left: 0; right: 0; padding: 12px; text-align: center; background: rgba(15, 23, 42, 0.9); }
        .cta-overlay.visible { display: block; }
        .cta-overlay a { display: inline-block; padding: 8px 24px; background: #22c55e; color: #fff; border: none; border-radius: 6px; font-size: 14px; font-weight: 600; text-decoration: none; }
        .cta-overlay a:hover { opacity: 0.9; color: #fff; }
    </style>
</head>
<body>
    <div class="container">
        <div class="video-wrapper">
            <video controls playsinline webkit-playsinline crossorigin="anonymous" controlsList="nodownload" src="{{.VideoURL}}"{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}></video>
        </div>
        {{if and .CtaText .CtaUrl}}
        <div class="cta-overlay" id="cta-overlay">
            <a href="{{.CtaUrl}}" target="_blank" rel="noopener noreferrer" id="cta-btn">{{.CtaText}}</a>
        </div>
        {{end}}
        <div class="footer">
            <span class="footer-title">{{.Title}}</span>
            <a href="{{.BaseURL}}/watch/{{.ShareToken}}" target="_blank" rel="noopener">Watch on SendRec</a>
        </div>
    </div>
    <script nonce="{{.Nonce}}">
        (function() {
            var v = document.querySelector('video');
            if (v) {
                v.muted = true;
                v.play().catch(function() {});
            }
        })();
        {{if and .CtaText .CtaUrl}}
        (function() {
            var v = document.querySelector('video');
            var overlay = document.getElementById('cta-overlay');
            var btn = document.getElementById('cta-btn');
            if (v && overlay) {
                v.addEventListener('ended', function() {
                    overlay.classList.add('visible');
                });
                v.addEventListener('play', function() {
                    overlay.classList.remove('visible');
                });
            }
            if (btn) {
                btn.addEventListener('click', function() {
                    fetch('/api/watch/{{.ShareToken}}/cta-click', { method: 'POST' }).catch(function() {});
                });
            }
        })();
        {{end}}
        (function() {
            var v = document.querySelector('video');
            if (!v) return;
            var milestones = [25, 50, 75, 100];
            var reached = {};
            v.addEventListener('timeupdate', function() {
                if (!v.duration) return;
                var pct = (v.currentTime / v.duration) * 100;
                for (var i = 0; i < milestones.length; i++) {
                    var m = milestones[i];
                    if (pct >= m && !reached[m]) {
                        reached[m] = true;
                        fetch('/api/watch/{{.ShareToken}}/milestone', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ milestone: m })
                        }).catch(function() {});
                    }
                }
            });
        })();
    </script>
</body>
</html>`))

var embedPasswordPageTemplate = template.Must(template.New("embed-password").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}}</title>
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        html, body { width: 100%; height: 100%; background: #0f172a; }
        body {
            color: #e2e8f0;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container { text-align: center; padding: 2rem; max-width: 360px; width: 100%; }
        h1 { font-size: 1.25rem; margin-bottom: 0.5rem; }
        p { color: #94a3b8; margin-bottom: 1rem; font-size: 0.875rem; }
        .error { color: #ef4444; font-size: 0.8rem; margin-bottom: 0.75rem; display: none; }
        input[type="password"] {
            width: 100%;
            padding: 0.625rem 0.75rem;
            border-radius: 6px;
            border: 1px solid #334155;
            background: #1e293b;
            color: #fff;
            font-size: 0.875rem;
            margin-bottom: 0.75rem;
            outline: none;
        }
        input[type="password"]:focus { border-color: #22c55e; }
        button {
            width: 100%;
            background: #22c55e;
            color: #fff;
            padding: 0.625rem 1rem;
            border: none;
            border-radius: 6px;
            font-size: 0.875rem;
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

func (h *Handler) EmbedPage(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var videoID string
	var title string
	var fileKey string
	var creator string
	var createdAt time.Time
	var shareExpiresAt *time.Time
	var thumbnailKey *string
	var sharePassword *string
	var contentType string
	var ownerID string
	var ownerEmail string
	var viewNotification *string
	var ctaText, ctaUrl *string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.title, v.file_key, u.name, v.created_at, v.share_expires_at,
		        v.thumbnail_key, v.share_password, v.content_type,
		        v.user_id, u.email, v.view_notification,
		        v.cta_text, v.cta_url
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID, &title, &fileKey, &creator, &createdAt, &shareExpiresAt,
		&thumbnailKey, &sharePassword, &contentType,
		&ownerID, &ownerEmail, &viewNotification,
		&ctaText, &ctaUrl)
	if err != nil {
		nonce := httputil.NonceFromContext(r.Context())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		if err := notFoundPageTemplate.Execute(w, notFoundPageData{Nonce: nonce}); err != nil {
			log.Printf("failed to render not found page: %v", err)
		}
		return
	}

	nonce := httputil.NonceFromContext(r.Context())

	if shareExpiresAt != nil && time.Now().After(*shareExpiresAt) {
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
			if err := embedPasswordPageTemplate.Execute(w, embedPasswordPageData{
				Title:      title,
				ShareToken: shareToken,
				Nonce:      nonce,
			}); err != nil {
				log.Printf("failed to render embed password page: %v", err)
			}
			return
		}
	}

	viewerUserID := h.viewerUserIDFromRequest(r)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ip := clientIP(r)
		hash := viewerHash(ip, r.UserAgent())
		if _, err := h.db.Exec(ctx,
			`INSERT INTO video_views (video_id, viewer_hash) VALUES ($1, $2)`,
			videoID, hash,
		); err != nil {
			log.Printf("failed to record embed view for %s: %v", videoID, err)
		}
		h.resolveAndNotify(ctx, videoID, ownerID, ownerEmail, creator, title, shareToken, viewerUserID, viewNotification)
	}()

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
	if err := embedPageTemplate.Execute(w, embedPageData{
		Title:        title,
		VideoURL:     videoURL,
		ThumbnailURL: thumbnailURL,
		ShareToken:   shareToken,
		Nonce:        nonce,
		BaseURL:      h.baseURL,
		ContentType:  contentType,
		CtaText:      derefString(ctaText),
		CtaUrl:       derefString(ctaUrl),
	}); err != nil {
		log.Printf("failed to render embed page: %v", err)
	}
}
