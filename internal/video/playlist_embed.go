package video

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

type playlistEmbedData struct {
	Title      string
	Nonce      string
	BaseURL    string
	ShareToken string
	Videos     []playlistWatchVideoItem
	VideosJSON template.JS
}

type playlistEmbedGateData struct {
	Title      string
	ShareToken string
	Nonce      string
}

var playlistEmbedTemplate = template.Must(template.New("playlist-embed").Funcs(template.FuncMap{
	"formatDuration": func(seconds int) string {
		return formatDuration(seconds)
	},
	"inc": func(i int) int { return i + 1 },
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}}</title>
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        html, body { width: 100%; height: 100%; overflow: hidden; }
        body {
            background: #0f172a;
            color: #e2e8f0;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
        }
` + playerCSS + safariWarningCSS + `
        .playlist-layout {
            display: flex;
            height: 100vh;
        }
        .playlist-sidebar {
            width: 240px;
            min-width: 240px;
            background: #0f172a;
            color: #e2e8f0;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }
        .sidebar-header {
            padding: 0.75rem 0.75rem;
            border-bottom: 1px solid #1e293b;
        }
        .sidebar-header h2 {
            font-size: 0.875rem;
            font-weight: 600;
            color: #f1f5f9;
            margin-bottom: 0.125rem;
            word-break: break-word;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .sidebar-header .video-count {
            font-size: 0.6875rem;
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
            gap: 0.5rem;
            padding: 0.5rem 0.75rem;
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
            font-size: 0.6875rem;
            color: #64748b;
            min-width: 1rem;
            text-align: center;
        }
        .video-list-item.active .position {
            color: #00b67a;
        }
        .video-thumb {
            width: 64px;
            height: 36px;
            border-radius: 3px;
            object-fit: cover;
            background: #334155;
            flex-shrink: 0;
        }
        .video-thumb-placeholder {
            width: 64px;
            height: 36px;
            border-radius: 3px;
            background: #334155;
            flex-shrink: 0;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .video-thumb-placeholder svg {
            width: 20px;
            height: 20px;
            fill: #64748b;
        }
        .video-info {
            flex: 1;
            min-width: 0;
        }
        .video-info .video-title {
            font-size: 0.75rem;
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
            font-size: 0.625rem;
            color: #64748b;
            margin-top: 0.0625rem;
        }
        .video-list-item .watched-badge {
            color: #00b67a;
            font-size: 0.6875rem;
            flex-shrink: 0;
        }
        .hidden { display: none; }
        .playlist-player {
            flex: 1;
            display: flex;
            flex-direction: column;
            min-width: 0;
        }
        .player-container {
            flex: 1;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #000;
            position: relative;
            overflow: hidden;
        }
        .player-container video {
            width: 100%;
            height: 100%;
            object-fit: contain;
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
        .next-overlay.hidden { display: none; }
        .next-overlay .next-label {
            font-size: 0.75rem;
            color: #94a3b8;
            margin-bottom: 0.375rem;
        }
        .next-overlay .next-title {
            font-size: 1rem;
            font-weight: 600;
            margin-bottom: 1rem;
            text-align: center;
            padding: 0 1.5rem;
        }
        .next-progress-bar {
            width: 160px;
            height: 3px;
            background: #334155;
            border-radius: 2px;
            margin-bottom: 1rem;
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
            gap: 0.5rem;
        }
        .next-actions button {
            padding: 0.375rem 1rem;
            border-radius: 5px;
            font-size: 0.8125rem;
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
        .footer {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 6px 10px;
            background: #1e293b;
            font-size: 12px;
        }
        .footer-title {
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            margin-right: 8px;
        }
        .footer a {
            color: #94a3b8;
            text-decoration: none;
            white-space: nowrap;
            font-size: 11px;
        }
        .footer a:hover { color: #e2e8f0; }
        @media (max-width: 480px) {
            .playlist-layout {
                flex-direction: column;
            }
            .playlist-sidebar {
                width: 100%;
                min-width: 100%;
                max-height: 35vh;
                order: 2;
            }
            .playlist-player {
                order: 1;
            }
            .volume-slider { display: none; }
        }
    </style>
</head>
<body>
    <div class="playlist-layout">
        <aside class="playlist-sidebar">
            <div class="sidebar-header">
                <h2 title="{{.Title}}">{{.Title}}</h2>
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
            <div class="player-container" id="player-container">
                <video id="player" playsinline{{if .Videos}} src="{{(index .Videos 0).VideoURL}}"{{end}}></video>
` + playerControlsHTML + `
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
            <div class="footer">
                <span class="footer-title">{{.Title}}</span>
                <a href="{{.BaseURL}}/watch/playlist/{{.ShareToken}}" target="_blank" rel="noopener">Watch on SendRec</a>
            </div>
` + safariWarningHTML + `
        </main>
    </div>
    <script nonce="{{.Nonce}}">
    (function() {
` + safariWarningJS + `
    })();
    (function() {
        var videos = {{.VideosJSON}};
        var currentIndex = 0;
        var player = document.getElementById('player');
        var container = document.getElementById('player-container');
        var nextOverlay = document.getElementById('next-overlay');
        var nextTitleEl = document.getElementById('next-title');
        var progressEl = document.getElementById('next-progress');
        var listItems = document.querySelectorAll('.video-list-item');
        var countdownTimer = null;
        var storageKey = 'playlist_progress_{{.ShareToken}}';

        var controls = document.getElementById('player-controls');
        var overlay = document.getElementById('player-overlay');
        var playBtn = document.getElementById('play-btn');
        var overlayBtn = document.getElementById('play-overlay-btn');
        var seekBar = document.getElementById('seek-bar');
        var seekProgress = document.getElementById('seek-progress');
        var seekBuffered = document.getElementById('seek-buffered');
        var seekThumb = document.getElementById('seek-thumb');
        var timeCurrent = document.getElementById('time-current');
        var timeDuration = document.getElementById('time-duration');
        var muteBtn = document.getElementById('mute-btn');
        var volumeSlider = document.getElementById('volume-slider');
        var speedBtn = document.getElementById('speed-btn');
        var speedMenu = document.getElementById('speed-menu');
        var pipBtn = document.getElementById('pip-btn');
        var fullscreenBtn = document.getElementById('fullscreen-btn');
        var spinner = document.getElementById('player-spinner');
        var errorOverlay = document.getElementById('player-error');
        var seekTooltip = document.getElementById('seek-time-tooltip');
        var shortcutsBtn = document.getElementById('shortcuts-btn');
        var shortcutsPanel = document.getElementById('shortcuts-panel');
        var hideTimer = null;

        // --- Watched tracking ---
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

` + playerJS + `

        // Mark watched at 80%
        player.addEventListener('timeupdate', function() {
            if (player.duration > 0 && player.currentTime / player.duration > 0.8) {
                markWatched(videos[currentIndex].id);
            }
        });

        // Playlist N/P key override
        onPlayerKeyOverride = function(e) {
            switch (e.key) {
                case 'n':
                case 'N':
                    if (currentIndex < videos.length - 1) switchVideo(currentIndex + 1);
                    e.preventDefault();
                    return true;
                case 'p':
                case 'P':
                    if (currentIndex > 0) switchVideo(currentIndex - 1);
                    e.preventDefault();
                    return true;
            }
            return false;
        };

        // Append N/P rows to shortcuts table
        var shortcutsTable = document.getElementById('shortcuts-table');
        if (shortcutsTable) {
            var nextRow = document.createElement('tr');
            nextRow.innerHTML = '<td>Next video</td><td><kbd>N</kbd></td>';
            shortcutsTable.appendChild(nextRow);
            var prevRow = document.createElement('tr');
            prevRow.innerHTML = '<td>Previous video</td><td><kbd>P</kbd></td>';
            shortcutsTable.appendChild(prevRow);
        }

        // --- Switch video ---
        function switchVideo(index) {
            if (index < 0 || index >= videos.length) return;
            cancelCountdown();
            currentIndex = index;
            var v = videos[index];

            seekProgress.style.width = '0%';
            seekBuffered.style.width = '0%';
            seekThumb.style.left = '0%';
            timeCurrent.textContent = '0:00';
            timeDuration.textContent = '0:00';
            errorOverlay.classList.remove('visible');
            spinner.classList.remove('visible');

            player.src = v.videoUrl;
            player.load();
            player.play().catch(function() {});
            listItems.forEach(function(li) {
                li.classList.toggle('active', parseInt(li.getAttribute('data-index'), 10) === index);
            });
            var li = listItems[index];
            if (li) { li.scrollIntoView({ block: 'nearest', behavior: 'smooth' }); }
        }

        listItems.forEach(function(li) {
            li.addEventListener('click', function() {
                switchVideo(parseInt(this.getAttribute('data-index'), 10));
            });
        });

        // --- Next-video countdown ---
        player.addEventListener('ended', function() {
            markWatched(videos[currentIndex].id);
            updatePlayBtn();
            if (currentIndex < videos.length - 1) {
                startCountdown(currentIndex + 1);
            }
        });

        function startCountdown(nextIndex) {
            var nextVideo = videos[nextIndex];
            nextTitleEl.textContent = nextVideo.title;
            nextOverlay.classList.remove('hidden');
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
            nextOverlay.classList.add('hidden');
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
</body>
</html>`))

var playlistEmbedPasswordTemplate = template.Must(template.New("playlist-embed-password").Parse(`<!DOCTYPE html>
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
        input[type="password"]:focus { border-color: #00b67a; }
        button {
            width: 100%;
            background: #00b67a;
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
        <h1>This playlist is password protected</h1>
        <p>Enter the password to watch.</p>
        <p class="error" id="error-msg"></p>
        <form id="password-form">
            <input type="password" id="password-input" placeholder="Password" required maxlength="128" autofocus>
            <button type="submit" id="submit-btn">Watch Playlist</button>
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
            fetch('/api/watch/playlist/{{.ShareToken}}/verify', {
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

var playlistEmbedEmailGateTemplate = template.Must(template.New("playlist-embed-emailgate").Parse(`<!DOCTYPE html>
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
        input[type="email"] {
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
        input[type="email"]:focus { border-color: #00b67a; }
        button {
            width: 100%;
            background: #00b67a;
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
        <h1>Enter your email to watch</h1>
        <p>{{.Title}}</p>
        <p class="error" id="error-msg"></p>
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
            errEl.style.display = 'none';
            fetch('/api/watch/playlist/{{.ShareToken}}/identify', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({email: email})
            }).then(function(r) {
                if (r.ok) { window.location.reload(); }
                else { return r.json().then(function(d) { errEl.textContent = d.error || 'Something went wrong'; errEl.style.display = 'block'; btn.disabled = false; }); }
            }).catch(function() {
                errEl.textContent = 'Something went wrong'; errEl.style.display = 'block'; btn.disabled = false;
            });
        });
    </script>
</body>
</html>`))

func (h *Handler) PlaylistEmbedPage(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")
	nonce := httputil.NonceFromContext(r.Context())

	var playlistID, title string
	var sharePassword *string
	var requireEmail bool

	err := h.db.QueryRow(r.Context(),
		`SELECT p.id, p.title, p.share_password, p.require_email
		 FROM playlists p
		 WHERE p.share_token = $1 AND p.is_shared = true`,
		shareToken,
	).Scan(&playlistID, &title, &sharePassword, &requireEmail)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if sharePassword != nil {
		if !hasValidWatchCookie(r, h.hmacSecret, shareToken, *sharePassword) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := playlistEmbedPasswordTemplate.Execute(w, playlistEmbedGateData{
				Title:      title,
				ShareToken: shareToken,
				Nonce:      nonce,
			}); err != nil {
				slog.Error("playlist-embed: failed to render password page", "error", err)
			}
			return
		}
	}

	if requireEmail {
		if _, ok := hasValidEmailGateCookie(r, h.hmacSecret, shareToken); !ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := playlistEmbedEmailGateTemplate.Execute(w, playlistEmbedGateData{
				Title:      title,
				ShareToken: shareToken,
				Nonce:      nonce,
			}); err != nil {
				slog.Error("playlist-embed: failed to render email gate page", "error", err)
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
			slog.Error("playlist-embed: failed to generate video URL", "video_id", id, "error", err)
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := playlistEmbedTemplate.Execute(w, playlistEmbedData{
		Title:      title,
		Nonce:      nonce,
		BaseURL:    h.baseURL,
		ShareToken: shareToken,
		Videos:     videoItems,
		VideosJSON: template.JS(videosJSONBytes),
	}); err != nil {
		slog.Error("playlist-embed: failed to render playlist embed page", "error", err)
	}
}
