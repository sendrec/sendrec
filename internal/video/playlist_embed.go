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
        :focus-visible { outline: 2px solid #00b67a; outline-offset: 2px; }
        html, body { width: 100%; height: 100%; overflow: hidden; }
        body {
            background: #0a1628;
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            -webkit-font-smoothing: antialiased;
        }
        .hidden { display: none !important; }
` + playerCSS + safariWarningCSS + `
        .playlist-layout {
            display: flex;
            height: 100vh;
        }
        .playlist-sidebar {
            width: 240px;
            min-width: 240px;
            background: #111d32;
            border-right: 1px solid #1e2d45;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }
        .sidebar-header {
            padding: 10px 12px;
            border-bottom: 1px solid #1e2d45;
            flex-shrink: 0;
        }
        .sidebar-header h2 {
            font-size: 14px;
            font-weight: 600;
            color: #ffffff;
            margin-bottom: 2px;
            word-break: break-word;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            line-height: 1.3;
        }
        .sidebar-meta {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 6px;
        }
        .sidebar-header .video-count {
            font-size: 11px;
            color: #8892a4;
        }
        .auto-advance-toggle {
            display: flex;
            align-items: center;
            gap: 4px;
            font-size: 10px;
            color: #8892a4;
            cursor: pointer;
            user-select: none;
        }
        .aa-toggle-track {
            width: 24px;
            height: 14px;
            border-radius: 7px;
            background: #334155;
            position: relative;
            transition: background 0.2s;
        }
        .aa-toggle-track.active {
            background: #00b67a;
        }
        .aa-toggle-knob {
            width: 10px;
            height: 10px;
            border-radius: 50%;
            background: #ffffff;
            position: absolute;
            top: 2px;
            left: 2px;
            transition: left 0.2s;
        }
        .aa-toggle-track.active .aa-toggle-knob {
            left: 12px;
        }
        .video-list {
            flex: 1;
            overflow-y: auto;
            list-style: none;
        }
        .video-list::-webkit-scrollbar { width: 4px; }
        .video-list::-webkit-scrollbar-track { background: transparent; }
        .video-list::-webkit-scrollbar-thumb { background: #1e2d45; border-radius: 2px; }
        .video-list-item {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 8px 12px;
            cursor: pointer;
            transition: background 0.15s;
            border-bottom: 1px solid #1e2d45;
            position: relative;
        }
        .video-list-item:hover {
            background: #1e293b;
        }
        .video-list-item.active {
            background: #1e3a5f;
            border-left: 3px solid #00b67a;
            padding-left: 9px;
        }
        .video-list-item .position {
            font-size: 11px;
            color: #8892a4;
            min-width: 14px;
            text-align: center;
            flex-shrink: 0;
            font-family: monospace;
        }
        .video-list-item.active .position {
            color: #00b67a;
            font-weight: 600;
        }
        .now-playing-tag {
            font-size: 9px;
            font-weight: 600;
            color: #00b67a;
            text-transform: uppercase;
            letter-spacing: 0.3px;
            display: none;
        }
        .video-list-item.active .now-playing-tag { display: block; }
        .video-thumb {
            width: 64px;
            height: 36px;
            border-radius: 3px;
            object-fit: cover;
            background: #334155;
            flex-shrink: 0;
            overflow: hidden;
            position: relative;
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
            overflow: hidden;
            position: relative;
        }
        .video-thumb-placeholder svg {
            width: 16px;
            height: 16px;
            fill: #64748b;
        }
        .item-thumb-duration {
            position: absolute;
            bottom: 1px;
            right: 2px;
            font-size: 9px;
            font-family: monospace;
            background: rgba(0,0,0,0.75);
            color: #fff;
            padding: 0px 3px;
            border-radius: 2px;
            line-height: 1.3;
        }
        .video-info {
            flex: 1;
            min-width: 0;
        }
        .video-info .video-title {
            font-size: 12px;
            font-weight: 500;
            color: #ffffff;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            line-height: 1.3;
        }
        .video-list-item.active .video-title {
            font-weight: 600;
        }
        .video-info .video-duration {
            font-size: 10px;
            color: #8892a4;
            margin-top: 1px;
        }
        .video-list-item .watched-badge {
            color: #00b67a;
            font-size: 12px;
            flex-shrink: 0;
        }
        .playlist-player {
            flex: 1;
            display: flex;
            flex-direction: column;
            min-width: 0;
            background: #0a1628;
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
            top: 0; left: 0; right: 0; bottom: 0;
            background: rgba(0, 0, 0, 0.85);
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            color: #ffffff;
            z-index: 10;
        }
        .next-overlay .next-label {
            font-size: 12px;
            color: #94a3b8;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            font-weight: 500;
            margin-bottom: 6px;
        }
        .next-overlay .next-title {
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 6px;
            text-align: center;
            padding: 0 1.5rem;
            line-height: 1.3;
        }
        .next-countdown {
            font-size: 11px;
            color: #94a3b8;
            margin-bottom: 10px;
        }
        .next-progress-bar {
            width: 160px;
            height: 3px;
            background: #334155;
            border-radius: 2px;
            margin-bottom: 16px;
            overflow: hidden;
        }
        .next-progress-fill {
            height: 100%;
            background: #00b67a;
            border-radius: 2px;
            width: 100%;
            transition: width 0.1s linear;
        }
        .next-actions {
            display: flex;
            gap: 8px;
        }
        .next-actions button {
            padding: 6px 16px;
            border-radius: 5px;
            font-size: 13px;
            font-weight: 600;
            cursor: pointer;
            border: none;
            transition: all 0.15s;
        }
        .btn-play-now {
            background: #00b67a;
            color: #fff;
        }
        .btn-play-now:hover { background: #00a06b; }
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
            background: #111d32;
            border-top: 1px solid #1e2d45;
            font-size: 12px;
            color: #8892a4;
            flex-shrink: 0;
        }
        .footer-title {
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            margin-right: 8px;
        }
        .footer a {
            color: #8892a4;
            text-decoration: none;
            white-space: nowrap;
            font-size: 11px;
            transition: color 0.15s;
        }
        .footer a:hover { color: #ffffff; }
        @media (prefers-reduced-motion: reduce) {
            *, *::before, *::after {
                animation-duration: 0.01ms !important;
                transition-duration: 0.01ms !important;
            }
        }
        @media (max-width: 480px) {
            .playlist-layout {
                flex-direction: column;
            }
            .playlist-sidebar {
                width: 100%;
                min-width: 100%;
                max-height: none;
                order: 2;
                border-right: none;
                border-top: 1px solid #1e2d45;
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
                <div class="sidebar-meta">
                    <span class="video-count">{{len .Videos}} videos</span>
                    <div class="auto-advance-toggle" id="auto-advance-toggle" role="switch" aria-checked="true" tabindex="0">
                        <span>Auto</span>
                        <div class="aa-toggle-track active" id="aa-toggle-track">
                            <div class="aa-toggle-knob"></div>
                        </div>
                    </div>
                </div>
            </div>
            <ul class="video-list" id="video-list">
                {{range $i, $v := .Videos}}
                <li class="video-list-item{{if eq $i 0}} active{{end}}" data-index="{{$i}}" tabindex="0" role="listitem">
                    <span class="position">{{$i | inc}}</span>
                    {{if $v.ThumbnailURL}}
                    <img class="video-thumb" src="{{$v.ThumbnailURL}}" alt="">
                    {{else}}
                    <div class="video-thumb-placeholder">
                        <svg viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                        <span class="item-thumb-duration">{{formatDuration $v.Duration}}</span>
                    </div>
                    {{end}}
                    <div class="video-info">
                        <div class="video-title" title="{{$v.Title}}">{{$v.Title}}</div>
                        <div class="video-duration">{{formatDuration $v.Duration}}</div>
                        <div class="now-playing-tag">Now Playing</div>
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
                    <div class="next-countdown" id="next-countdown"></div>
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
        var nextCountdownEl = document.getElementById('next-countdown');
        var listItems = document.querySelectorAll('.video-list-item');
        var countdownTimer = null;
        var autoAdvance = true;
        var storageKey = 'playlist_progress_{{.ShareToken}}';
        var aaToggle = document.getElementById('auto-advance-toggle');
        var aaTrack = document.getElementById('aa-toggle-track');
        if (aaToggle) {
            aaToggle.addEventListener('click', function() {
                autoAdvance = !autoAdvance;
                aaTrack.classList.toggle('active', autoAdvance);
                aaToggle.setAttribute('aria-checked', autoAdvance ? 'true' : 'false');
            });
            aaToggle.addEventListener('keydown', function(e) {
                if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); aaToggle.click(); }
            });
        }

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
            li.addEventListener('keydown', function(e) {
                if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    switchVideo(parseInt(this.getAttribute('data-index'), 10));
                }
            });
        });

        // --- Next-video countdown ---
        player.addEventListener('ended', function() {
            markWatched(videos[currentIndex].id);
            updatePlayBtn();
            if (autoAdvance && currentIndex < videos.length - 1) {
                startCountdown(currentIndex + 1);
            }
        });

        function startCountdown(nextIndex) {
            if (!autoAdvance) return;
            var nextVideo = videos[nextIndex];
            nextTitleEl.textContent = nextVideo.title;
            nextOverlay.classList.remove('hidden');
            var remaining = 5000;
            var interval = 50;
            progressEl.style.width = '100%';
            if (nextCountdownEl) nextCountdownEl.textContent = 'Playing in 5...';
            countdownTimer = setInterval(function() {
                remaining -= interval;
                progressEl.style.width = Math.max(0, (remaining / 5000) * 100) + '%';
                var seconds = Math.ceil(remaining / 1000);
                if (nextCountdownEl) nextCountdownEl.textContent = 'Playing in ' + seconds + '...';
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
        :focus-visible { outline: 2px solid #00b67a; outline-offset: 2px; }
        html, body { width: 100%; height: 100%; background: #0a1628; }
        body {
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            align-items: center;
            justify-content: center;
            -webkit-font-smoothing: antialiased;
        }
        .container { text-align: center; padding: 2rem; max-width: 360px; width: 100%; }
        .gate-icon { width: 48px; height: 48px; border-radius: 10px; background: rgba(0,182,122,0.1); display: flex; align-items: center; justify-content: center; margin: 0 auto 16px; font-size: 20px; }
        h1 { font-size: 1.25rem; margin-bottom: 0.5rem; font-weight: 700; }
        p { color: #94a3b8; margin-bottom: 1rem; font-size: 0.875rem; }
        .error { color: #ef4444; font-size: 0.8rem; margin-bottom: 0.75rem; display: none; }
        input[type="password"] {
            width: 100%; padding: 0.625rem 0.75rem; border-radius: 6px;
            border: 1px solid #334155; background: #1e293b; color: #fff;
            font-size: 0.875rem; margin-bottom: 0.75rem; outline: none;
        }
        input[type="password"]:focus { border-color: #00b67a; box-shadow: 0 0 0 3px rgba(0,182,122,0.1); }
        input[type="password"]::placeholder { color: #94a3b8; opacity: 0.5; }
        button {
            width: 100%; background: #00b67a; color: #fff; padding: 0.625rem 1rem;
            border: none; border-radius: 6px; font-size: 0.875rem; font-weight: 600;
            cursor: pointer; transition: background 0.15s;
        }
        button:hover { background: #00a06b; }
        button:disabled { opacity: 0.5; cursor: not-allowed; }
    </style>
</head>
<body>
    <div class="container">
        <div class="gate-icon">&#128274;</div>
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
        :focus-visible { outline: 2px solid #00b67a; outline-offset: 2px; }
        html, body { width: 100%; height: 100%; background: #0a1628; }
        body {
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            align-items: center;
            justify-content: center;
            -webkit-font-smoothing: antialiased;
        }
        .container { text-align: center; padding: 2rem; max-width: 360px; width: 100%; }
        .gate-icon { width: 48px; height: 48px; border-radius: 10px; background: rgba(0,182,122,0.1); display: flex; align-items: center; justify-content: center; margin: 0 auto 16px; font-size: 20px; }
        h1 { font-size: 1.25rem; margin-bottom: 0.5rem; font-weight: 700; }
        p { color: #94a3b8; margin-bottom: 1rem; font-size: 0.875rem; }
        .error { color: #ef4444; font-size: 0.8rem; margin-bottom: 0.75rem; display: none; }
        input[type="email"] {
            width: 100%; padding: 0.625rem 0.75rem; border-radius: 6px;
            border: 1px solid #334155; background: #1e293b; color: #fff;
            font-size: 0.875rem; margin-bottom: 0.75rem; outline: none;
        }
        input[type="email"]:focus { border-color: #00b67a; box-shadow: 0 0 0 3px rgba(0,182,122,0.1); }
        input[type="email"]::placeholder { color: #94a3b8; opacity: 0.5; }
        button {
            width: 100%; background: #00b67a; color: #fff; padding: 0.625rem 1rem;
            border: none; border-radius: 6px; font-size: 0.875rem; font-weight: 600;
            cursor: pointer; transition: background 0.15s;
        }
        button:hover { background: #00a06b; }
        button:disabled { opacity: 0.5; cursor: not-allowed; }
    </style>
</head>
<body>
    <div class="container">
        <div class="gate-icon">&#9993;</div>
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
