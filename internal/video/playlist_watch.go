package video

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

type playlistWatchData struct {
	Title         string
	Description   string
	Nonce         string
	BaseURL       string
	ShareToken    string
	Videos        []playlistWatchVideoItem
	VideosJSON    template.JS
	NeedsPassword bool
	NeedsEmail    bool
}

type playlistWatchVideoItem struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Duration     int    `json:"duration"`
	ShareToken   string `json:"shareToken"`
	VideoURL     string `json:"videoUrl"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	ContentType  string `json:"contentType"`
}

var playlistWatchTemplate = template.Must(template.New("playlist-watch").Funcs(template.FuncMap{
	"formatDuration": func(seconds int) string {
		return formatDuration(seconds)
	},
	"inc": func(i int) int { return i + 1 },
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} — SendRec</title>
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: #ffffff;
            color: #1e293b;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            min-height: 100vh;
        }
        {{if .NeedsPassword}}
        body {
            background: #0a1628;
            color: #ffffff;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .gate-container { text-align: center; padding: 2rem; max-width: 400px; width: 100%; }
        .gate-container h1 { font-size: 1.5rem; margin-bottom: 0.75rem; }
        .gate-container p { color: #94a3b8; margin-bottom: 1.5rem; }
        .gate-error { color: #ef4444; font-size: 0.875rem; margin-bottom: 1rem; display: none; }
        .gate-error.visible { display: block; }
        .gate-container input[type="password"] {
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
        .gate-container input[type="password"]:focus { border-color: #00b67a; }
        .gate-container button {
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
        .gate-container button:hover { opacity: 0.9; }
        .gate-container button:disabled { opacity: 0.5; cursor: not-allowed; }
        {{else if .NeedsEmail}}
        body {
            background: #0a1628;
            color: #ffffff;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .gate-container { text-align: center; padding: 2rem; max-width: 400px; width: 100%; }
        .gate-container h1 { font-size: 1.5rem; margin-bottom: 0.75rem; }
        .gate-container p { color: #94a3b8; margin-bottom: 1.5rem; }
        .gate-error { color: #ef4444; font-size: 0.875rem; margin-bottom: 1rem; display: none; }
        .gate-error.visible { display: block; }
        .gate-container input[type="email"] {
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
        .gate-container input[type="email"]:focus { border-color: #00b67a; }
        .gate-container button {
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
        .gate-container button:hover { opacity: 0.9; }
        .gate-container button:disabled { opacity: 0.5; cursor: not-allowed; }
        {{else}}
        .playlist-layout {
            display: flex;
            min-height: 100vh;
        }
        .playlist-sidebar {
            width: 280px;
            min-width: 280px;
            background: #0f172a;
            color: #e2e8f0;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }
        .sidebar-header {
            padding: 1.25rem 1rem;
            border-bottom: 1px solid #1e293b;
        }
        .sidebar-header h2 {
            font-size: 1rem;
            font-weight: 600;
            color: #f1f5f9;
            margin-bottom: 0.25rem;
            word-break: break-word;
        }
        .sidebar-header .video-count {
            font-size: 0.75rem;
            color: #64748b;
        }
        .video-list {
            flex: 1;
            overflow-y: auto;
            list-style: none;
        }
        .video-list-item {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            padding: 0.625rem 1rem;
            cursor: pointer;
            transition: background 0.15s;
            border-bottom: 1px solid #1e293b;
        }
        .video-list-item:hover {
            background: #1e293b;
        }
        .video-list-item.active {
            background: #1e3a5f;
        }
        .video-list-item .position {
            font-size: 0.75rem;
            color: #64748b;
            min-width: 1.25rem;
            text-align: center;
        }
        .video-list-item.active .position {
            color: #00b67a;
        }
        .video-thumb {
            width: 80px;
            height: 45px;
            border-radius: 4px;
            object-fit: cover;
            background: #334155;
            flex-shrink: 0;
        }
        .video-thumb-placeholder {
            width: 80px;
            height: 45px;
            border-radius: 4px;
            background: #334155;
            flex-shrink: 0;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .video-thumb-placeholder svg {
            width: 24px;
            height: 24px;
            fill: #64748b;
        }
        .video-info {
            flex: 1;
            min-width: 0;
        }
        .video-info .video-title {
            font-size: 0.8125rem;
            font-weight: 500;
            color: #e2e8f0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .video-list-item.active .video-title {
            color: #ffffff;
        }
        .video-info .video-duration {
            font-size: 0.6875rem;
            color: #64748b;
            margin-top: 0.125rem;
        }
        .video-list-item .watched-badge {
            color: #00b67a;
            font-size: 0.75rem;
            flex-shrink: 0;
        }
        .playlist-player {
            flex: 1;
            display: flex;
            flex-direction: column;
            min-width: 0;
        }
        .player-header {
            padding: 1rem 1.5rem;
            border-bottom: 1px solid #e2e8f0;
        }
        .player-header h1 {
            font-size: 1.25rem;
            font-weight: 600;
            color: #0f172a;
        }
        .player-header .player-counter {
            font-size: 0.8125rem;
            color: #64748b;
            margin-top: 0.25rem;
        }
        .player-container {
            flex: 1;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 1.5rem;
            background: #000;
            position: relative;
        }
        .player-container video {
            width: 100%;
            max-height: 70vh;
            border-radius: 4px;
            background: #000;
        }
        .next-overlay {
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.85);
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            color: #ffffff;
            z-index: 10;
        }
        .hidden { display: none; }
        .next-overlay.hidden { display: none; }
        .next-overlay .next-label {
            font-size: 0.875rem;
            color: #94a3b8;
            margin-bottom: 0.5rem;
        }
        .next-overlay .next-title {
            font-size: 1.25rem;
            font-weight: 600;
            margin-bottom: 1.5rem;
            text-align: center;
            padding: 0 2rem;
        }
        .next-progress-bar {
            width: 200px;
            height: 4px;
            background: #334155;
            border-radius: 2px;
            margin-bottom: 1.5rem;
            overflow: hidden;
        }
        .next-progress-fill {
            height: 100%;
            background: #00b67a;
            border-radius: 2px;
            transition: width 0.1s linear;
        }
        .next-actions {
            display: flex;
            gap: 0.75rem;
        }
        .next-actions button {
            padding: 0.5rem 1.25rem;
            border-radius: 6px;
            font-size: 0.875rem;
            font-weight: 600;
            cursor: pointer;
            border: none;
        }
        .btn-play-now {
            background: #00b67a;
            color: #fff;
        }
        .btn-play-now:hover { opacity: 0.9; }
        .btn-cancel {
            background: transparent;
            color: #94a3b8;
            border: 1px solid #475569 !important;
        }
        .btn-cancel:hover { color: #e2e8f0; border-color: #94a3b8 !important; }
        .branding-footer {
            padding: 0.75rem 1.5rem;
            text-align: center;
            font-size: 0.75rem;
            color: #94a3b8;
            border-top: 1px solid #e2e8f0;
        }
        .branding-footer a {
            color: #00b67a;
            text-decoration: none;
        }
        .branding-footer a:hover { text-decoration: underline; }
        @media (max-width: 768px) {
            .playlist-layout {
                flex-direction: column;
            }
            .playlist-sidebar {
                width: 100%;
                min-width: 100%;
                max-height: 40vh;
                order: 2;
            }
            .playlist-player {
                order: 1;
            }
            .player-container video {
                max-height: 40vh;
            }
        }
        {{end}}
    </style>
</head>
<body>
    {{if .NeedsPassword}}
    <div class="gate-container">
        <h1>{{.Title}}</h1>
        <p>This playlist is password protected</p>
        <p class="gate-error" id="error-msg"></p>
        <form id="password-form">
            <input type="password" id="password-input" placeholder="Enter password" required maxlength="128" autofocus>
            <button type="submit" id="submit-btn">Continue</button>
        </form>
    </div>
    <script nonce="{{.Nonce}}">
        document.getElementById('password-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var btn = document.getElementById('submit-btn');
            var errEl = document.getElementById('error-msg');
            var pw = document.getElementById('password-input').value;
            btn.disabled = true;
            errEl.classList.remove('visible');
            fetch('/api/watch/playlist/{{.ShareToken}}/verify', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({password: pw})
            }).then(function(r) {
                if (r.ok) { window.location.reload(); }
                else { return r.json().then(function(d) { errEl.textContent = d.error || 'Incorrect password'; errEl.classList.add('visible'); btn.disabled = false; }); }
            }).catch(function() {
                errEl.textContent = 'Something went wrong'; errEl.classList.add('visible'); btn.disabled = false;
            });
        });
    </script>
    {{else if .NeedsEmail}}
    <div class="gate-container">
        <h1>{{.Title}}</h1>
        <p>Enter your email to watch this playlist</p>
        <p class="gate-error" id="error-msg"></p>
        <form id="email-gate-form">
            <input type="email" id="email-input" placeholder="you@example.com" required maxlength="320" autofocus>
            <button type="submit" id="submit-btn">Watch Playlist</button>
        </form>
    </div>
    <script nonce="{{.Nonce}}">
        document.getElementById('email-gate-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var btn = document.getElementById('submit-btn');
            var errEl = document.getElementById('error-msg');
            var email = document.getElementById('email-input').value;
            btn.disabled = true;
            errEl.classList.remove('visible');
            fetch('/api/watch/playlist/{{.ShareToken}}/identify', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({email: email})
            }).then(function(r) {
                if (r.ok) { window.location.reload(); }
                else { return r.json().then(function(d) { errEl.textContent = d.error || 'Something went wrong'; errEl.classList.add('visible'); btn.disabled = false; }); }
            }).catch(function() {
                errEl.textContent = 'Something went wrong'; errEl.classList.add('visible'); btn.disabled = false;
            });
        });
    </script>
    {{else}}
    <div class="playlist-layout">
        <aside class="playlist-sidebar">
            <div class="sidebar-header">
                <h2>{{.Title}}</h2>
                <div class="video-count">{{len .Videos}} videos</div>
            </div>
            <ul class="video-list" id="video-list">
                {{range $i, $v := .Videos}}
                <li class="video-list-item{{if eq $i 0}} active{{end}}" data-index="{{$i}}">
                    <span class="position">{{$i | inc}}</span>
                    {{if $v.ThumbnailURL}}
                    <img class="video-thumb" src="{{$v.ThumbnailURL}}" alt="">
                    {{else}}
                    <div class="video-thumb-placeholder">
                        <svg viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                    </div>
                    {{end}}
                    <div class="video-info">
                        <div class="video-title" title="{{$v.Title}}">{{$v.Title}}</div>
                        <div class="video-duration">{{formatDuration $v.Duration}}</div>
                    </div>
                    <span class="watched-badge hidden">&#10003;</span>
                </li>
                {{end}}
            </ul>
        </aside>
        <main class="playlist-player">
            <div class="player-header">
                <h1 id="current-title">{{if .Videos}}{{(index .Videos 0).Title}}{{end}}</h1>
                <div class="player-counter" id="player-counter">{{if .Videos}}1 of {{len .Videos}}{{end}}</div>
            </div>
            <div class="player-container">
                <video id="player" controls playsinline{{if .Videos}} src="{{(index .Videos 0).VideoURL}}"{{end}}></video>
                <div class="next-overlay hidden" id="next-overlay">
                    <div class="next-label">Up next</div>
                    <div class="next-title" id="next-title"></div>
                    <div class="next-progress-bar"><div class="next-progress-fill" id="next-progress"></div></div>
                    <div class="next-actions">
                        <button class="btn-play-now" id="btn-play-now">Play Now</button>
                        <button class="btn-cancel" id="btn-cancel">Cancel</button>
                    </div>
                </div>
            </div>
            <div class="branding-footer">
                Shared via <a href="https://sendrec.eu" target="_blank" rel="noopener">SendRec</a>
            </div>
        </main>
    </div>
    <script nonce="{{.Nonce}}">
    (function() {
        var videos = {{.VideosJSON}};
        var currentIndex = 0;
        var player = document.getElementById('player');
        var titleEl = document.getElementById('current-title');
        var counterEl = document.getElementById('player-counter');
        var overlay = document.getElementById('next-overlay');
        var nextTitleEl = document.getElementById('next-title');
        var progressEl = document.getElementById('next-progress');
        var listItems = document.querySelectorAll('.video-list-item');
        var countdownTimer = null;
        var storageKey = 'playlist_progress_{{.ShareToken}}';

        function loadWatchedSet() {
            try {
                var raw = localStorage.getItem(storageKey);
                if (raw) { return new Set(JSON.parse(raw)); }
            } catch(e) {}
            return new Set();
        }

        function saveWatchedSet(s) {
            try { localStorage.setItem(storageKey, JSON.stringify(Array.from(s))); } catch(e) {}
        }

        var watched = loadWatchedSet();

        function updateWatchedBadges() {
            listItems.forEach(function(li) {
                var idx = parseInt(li.getAttribute('data-index'), 10);
                var badge = li.querySelector('.watched-badge');
                if (badge && videos[idx]) {
                    if (watched.has(videos[idx].id)) { badge.classList.remove('hidden'); } else { badge.classList.add('hidden'); }
                }
            });
        }
        updateWatchedBadges();

        function markWatched(videoId) {
            if (!watched.has(videoId)) {
                watched.add(videoId);
                saveWatchedSet(watched);
                updateWatchedBadges();
            }
        }

        function switchVideo(index) {
            if (index < 0 || index >= videos.length) return;
            cancelCountdown();
            currentIndex = index;
            var v = videos[index];
            player.src = v.videoUrl;
            player.load();
            player.play().catch(function() {});
            titleEl.textContent = v.title;
            counterEl.textContent = (index + 1) + ' of ' + videos.length;
            listItems.forEach(function(li) {
                li.classList.toggle('active', parseInt(li.getAttribute('data-index'), 10) === index);
            });
            li = listItems[index];
            if (li) { li.scrollIntoView({ block: 'nearest', behavior: 'smooth' }); }
        }

        listItems.forEach(function(li) {
            li.addEventListener('click', function() {
                switchVideo(parseInt(this.getAttribute('data-index'), 10));
            });
        });

        player.addEventListener('timeupdate', function() {
            if (player.duration > 0 && player.currentTime / player.duration > 0.8) {
                markWatched(videos[currentIndex].id);
            }
        });

        player.addEventListener('ended', function() {
            markWatched(videos[currentIndex].id);
            if (currentIndex < videos.length - 1) {
                startCountdown(currentIndex + 1);
            }
        });

        function startCountdown(nextIndex) {
            var nextVideo = videos[nextIndex];
            nextTitleEl.textContent = nextVideo.title;
            overlay.classList.remove('hidden');
            var remaining = 5000;
            var interval = 50;
            progressEl.style.width = '100%';
            countdownTimer = setInterval(function() {
                remaining -= interval;
                progressEl.style.width = Math.max(0, (remaining / 5000) * 100) + '%';
                if (remaining <= 0) {
                    cancelCountdown();
                    switchVideo(nextIndex);
                }
            }, interval);
        }

        function cancelCountdown() {
            if (countdownTimer) {
                clearInterval(countdownTimer);
                countdownTimer = null;
            }
            overlay.classList.add('hidden');
        }

        document.getElementById('btn-play-now').addEventListener('click', function() {
            cancelCountdown();
            switchVideo(currentIndex + 1);
        });

        document.getElementById('btn-cancel').addEventListener('click', function() {
            cancelCountdown();
        });
    })();
    </script>
    {{end}}
</body>
</html>`))


func (h *Handler) PlaylistWatchPage(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")
	nonce := httputil.NonceFromContext(r.Context())

	var playlistID, title string
	var description *string
	var sharePassword *string
	var requireEmail bool

	err := h.db.QueryRow(r.Context(),
		`SELECT p.id, p.title, p.description, p.share_password, p.require_email
		 FROM playlists p
		 WHERE p.share_token = $1 AND p.is_shared = true`,
		shareToken,
	).Scan(&playlistID, &title, &description, &sharePassword, &requireEmail)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if sharePassword != nil {
		if !hasValidWatchCookie(r, h.hmacSecret, shareToken, *sharePassword) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := playlistWatchTemplate.Execute(w, playlistWatchData{
				Title:         title,
				Nonce:         nonce,
				BaseURL:       h.baseURL,
				ShareToken:    shareToken,
				NeedsPassword: true,
			}); err != nil {
				slog.Error("playlist-watch: failed to render password page", "error", err)
			}
			return
		}
	}

	if requireEmail {
		if _, ok := hasValidEmailGateCookie(r, h.hmacSecret, shareToken); !ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := playlistWatchTemplate.Execute(w, playlistWatchData{
				Title:      title,
				Nonce:      nonce,
				BaseURL:    h.baseURL,
				ShareToken: shareToken,
				NeedsEmail: true,
			}); err != nil {
				slog.Error("playlist-watch: failed to render email gate page", "error", err)
			}
			return
		}
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id,
		        v.thumbnail_key
		 FROM playlist_videos pv
		 JOIN videos v ON v.id = pv.video_id AND v.status IN ('ready', 'processing')
		 WHERE pv.playlist_id = $1
		 ORDER BY pv.position, v.created_at`,
		playlistID,
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	videoItems := make([]playlistWatchVideoItem, 0)
	for rows.Next() {
		var id, videoTitle, videoShareToken, contentType, userID string
		var duration int
		var thumbnailKey *string

		if err := rows.Scan(&id, &videoTitle, &duration, &videoShareToken, &contentType, &userID, &thumbnailKey); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		videoURL, err := h.storage.GenerateDownloadURL(r.Context(), videoFileKey(userID, videoShareToken, contentType), 1*time.Hour)
		if err != nil {
			slog.Error("playlist-watch: failed to generate video URL", "video_id", id, "error", err)
			continue
		}

		item := playlistWatchVideoItem{
			ID:          id,
			Title:       videoTitle,
			Duration:    duration,
			ShareToken:  videoShareToken,
			VideoURL:    videoURL,
			ContentType: contentType,
		}

		if thumbnailKey != nil {
			thumbURL, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour)
			if err == nil {
				item.ThumbnailURL = thumbURL
			}
		}

		videoItems = append(videoItems, item)
	}

	videosJSONBytes, _ := json.Marshal(videoItems)

	descriptionText := derefString(description)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := playlistWatchTemplate.Execute(w, playlistWatchData{
		Title:       title,
		Description: descriptionText,
		Nonce:       nonce,
		BaseURL:     h.baseURL,
		ShareToken:  shareToken,
		Videos:      videoItems,
		VideosJSON:  template.JS(videosJSONBytes),
	}); err != nil {
		slog.Error("playlist-watch: failed to render playlist watch page", "error", err)
	}
}

func (h *Handler) VerifyPlaylistWatchPassword(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var req verifyPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Password) > 128 {
		httputil.WriteError(w, http.StatusBadRequest, "password is too long")
		return
	}

	var sharePassword *string
	err := h.db.QueryRow(r.Context(),
		`SELECT share_password FROM playlists WHERE share_token = $1 AND is_shared = true`,
		shareToken,
	).Scan(&sharePassword)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "playlist not found")
		return
	}

	if sharePassword == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !checkSharePassword(*sharePassword, req.Password) {
		httputil.WriteError(w, http.StatusForbidden, "incorrect password")
		return
	}

	sig := signWatchCookie(h.hmacSecret, shareToken, *sharePassword)
	setWatchCookie(w, shareToken, sig, h.secureCookies)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) IdentifyPlaylistViewer(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var req identifyViewerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || len(req.Email) > 320 || !strings.Contains(req.Email, "@") {
		httputil.WriteError(w, http.StatusBadRequest, "invalid email")
		return
	}

	var playlistID string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM playlists WHERE share_token = $1 AND is_shared = true`,
		shareToken,
	).Scan(&playlistID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "playlist not found")
		return
	}

	sig := signEmailGateCookie(h.hmacSecret, shareToken, req.Email)
	setEmailGateCookie(w, shareToken, sig, h.secureCookies)
	w.WriteHeader(http.StatusOK)
}
