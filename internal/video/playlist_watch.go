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
        :focus-visible { outline: 2px solid #00b67a; outline-offset: 2px; }
        body {
            background: #0a1628;
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            min-height: 100vh;
            -webkit-font-smoothing: antialiased;
        }
        .hidden { display: none !important; }
        {{if .NeedsPassword}}
        body { display: flex; align-items: center; justify-content: center; }
        .gate-container { text-align: center; padding: 2rem; max-width: 400px; width: 100%; }
        .gate-icon { width: 56px; height: 56px; border-radius: 12px; background: rgba(0,182,122,0.1); display: flex; align-items: center; justify-content: center; margin: 0 auto 20px; font-size: 24px; }
        .gate-container h1 { font-size: 24px; font-weight: 700; margin-bottom: 0.75rem; }
        .gate-container p { color: #94a3b8; margin-bottom: 1.5rem; }
        .gate-error { color: #ef4444; font-size: 0.875rem; margin-bottom: 1rem; display: none; }
        .gate-error.visible { display: block; }
        .gate-container input[type="password"] {
            width: 100%; padding: 0.75rem 1rem; border-radius: 8px;
            border: 1px solid #334155; background: #1e293b; color: #fff;
            font-size: 1rem; margin-bottom: 1rem; outline: none;
        }
        .gate-container input[type="password"]:focus { border-color: #00b67a; box-shadow: 0 0 0 3px rgba(0,182,122,0.1); }
        .gate-container input[type="password"]::placeholder { color: #94a3b8; opacity: 0.5; }
        .gate-container button {
            width: 100%; background: #00b67a; color: #fff; padding: 0.75rem 1.5rem;
            border: none; border-radius: 8px; font-size: 1rem; font-weight: 600; cursor: pointer;
            transition: background 0.15s;
        }
        .gate-container button:hover { background: #00a06b; }
        .gate-container button:disabled { opacity: 0.5; cursor: not-allowed; }
        .gate-branding { margin-top: 24px; font-size: 12px; color: #8892a4; }
        .gate-branding a { color: #00b67a; text-decoration: none; }
        .gate-branding a:hover { text-decoration: underline; }
        {{else if .NeedsEmail}}
        body { display: flex; align-items: center; justify-content: center; }
        .gate-container { text-align: center; padding: 2rem; max-width: 400px; width: 100%; }
        .gate-icon { width: 56px; height: 56px; border-radius: 12px; background: rgba(0,182,122,0.1); display: flex; align-items: center; justify-content: center; margin: 0 auto 20px; font-size: 24px; }
        .gate-container h1 { font-size: 24px; font-weight: 700; margin-bottom: 0.75rem; }
        .gate-container p { color: #94a3b8; margin-bottom: 1.5rem; }
        .gate-error { color: #ef4444; font-size: 0.875rem; margin-bottom: 1rem; display: none; }
        .gate-error.visible { display: block; }
        .gate-container input[type="email"] {
            width: 100%; padding: 0.75rem 1rem; border-radius: 8px;
            border: 1px solid #334155; background: #1e293b; color: #fff;
            font-size: 1rem; margin-bottom: 1rem; outline: none;
        }
        .gate-container input[type="email"]:focus { border-color: #00b67a; box-shadow: 0 0 0 3px rgba(0,182,122,0.1); }
        .gate-container input[type="email"]::placeholder { color: #94a3b8; opacity: 0.5; }
        .gate-container button {
            width: 100%; background: #00b67a; color: #fff; padding: 0.75rem 1.5rem;
            border: none; border-radius: 8px; font-size: 1rem; font-weight: 600; cursor: pointer;
            transition: background 0.15s;
        }
        .gate-container button:hover { background: #00a06b; }
        .gate-container button:disabled { opacity: 0.5; cursor: not-allowed; }
        .gate-branding { margin-top: 24px; font-size: 12px; color: #8892a4; }
        .gate-branding a { color: #00b67a; text-decoration: none; }
        .gate-branding a:hover { text-decoration: underline; }
        {{else}}
` + playerCSS + safariWarningCSS + `
        .playlist-layout {
            display: flex;
            min-height: 100vh;
        }
        .playlist-sidebar {
            width: 300px;
            min-width: 300px;
            background: #111d32;
            border-right: 1px solid #1e2d45;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }
        .sidebar-header {
            padding: 20px 16px 16px;
            border-bottom: 1px solid #1e2d45;
            flex-shrink: 0;
        }
        .sidebar-header h2 {
            font-size: 18px;
            font-weight: 600;
            color: #ffffff;
            margin-bottom: 8px;
            word-break: break-word;
            line-height: 1.3;
        }
        .sidebar-meta {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
        }
        .now-playing-label {
            font-size: 13px;
            color: #8892a4;
        }
        .now-playing-label strong {
            color: #00b67a;
            font-weight: 600;
        }
        .auto-advance-toggle {
            display: flex;
            align-items: center;
            gap: 6px;
            font-size: 12px;
            color: #8892a4;
            cursor: pointer;
            user-select: none;
        }
        .aa-toggle-track {
            width: 32px;
            height: 18px;
            background: #1a2740;
            border-radius: 9px;
            position: relative;
            transition: background 0.2s;
            border: 1px solid #1e2d45;
        }
        .aa-toggle-track.active {
            background: #00b67a;
            border-color: #00b67a;
        }
        .aa-toggle-knob {
            position: absolute;
            top: 2px;
            left: 2px;
            width: 12px;
            height: 12px;
            background: #fff;
            border-radius: 50%;
            transition: left 0.2s;
        }
        .aa-toggle-track.active .aa-toggle-knob {
            left: 16px;
        }
        .sidebar-collapse-btn {
            display: none;
            width: 100%;
            padding: 10px;
            background: #1a2740;
            border: none;
            border-bottom: 1px solid #1e2d45;
            color: #8892a4;
            font-size: 13px;
            font-weight: 500;
            cursor: pointer;
            text-align: center;
        }
        .sidebar-collapse-btn:hover { background: #1e2d45; }
        .sidebar-collapse-btn svg {
            width: 14px; height: 14px; vertical-align: middle; margin-left: 4px;
            transition: transform 0.2s;
        }
        .sidebar-collapse-btn.collapsed svg { transform: rotate(180deg); }
        .video-list {
            flex: 1;
            overflow-y: auto;
            list-style: none;
        }
        .video-list::-webkit-scrollbar { width: 6px; }
        .video-list::-webkit-scrollbar-track { background: transparent; }
        .video-list::-webkit-scrollbar-thumb { background: #1e2d45; border-radius: 3px; }
        .video-list.mobile-collapsed { display: none; }
        .video-list-item {
            display: flex;
            align-items: center;
            gap: 10px;
            padding: 10px 16px;
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
            padding-left: 13px;
        }
        .video-list-item .position {
            font-size: 12px;
            color: #8892a4;
            min-width: 18px;
            text-align: center;
            flex-shrink: 0;
            font-family: monospace;
        }
        .video-list-item.active .position {
            color: #00b67a;
            font-weight: 600;
        }
        .now-playing-tag {
            font-size: 10px;
            font-weight: 600;
            color: #00b67a;
            text-transform: uppercase;
            letter-spacing: 0.3px;
            display: none;
        }
        .video-list-item.active .now-playing-tag { display: block; }
        .video-thumb {
            width: 80px;
            height: 45px;
            border-radius: 4px;
            object-fit: cover;
            background: #334155;
            flex-shrink: 0;
            overflow: hidden;
            position: relative;
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
            overflow: hidden;
            position: relative;
        }
        .video-thumb-placeholder svg {
            width: 20px;
            height: 20px;
            fill: #64748b;
        }
        .item-thumb-duration {
            position: absolute;
            bottom: 2px;
            right: 3px;
            font-size: 10px;
            font-family: monospace;
            background: rgba(0,0,0,0.75);
            color: #fff;
            padding: 1px 4px;
            border-radius: 2px;
            line-height: 1.2;
        }
        .video-info {
            flex: 1;
            min-width: 0;
        }
        .video-info .video-title {
            font-size: 13px;
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
            font-size: 11px;
            color: #8892a4;
            margin-top: 2px;
        }
        .video-list-item .watched-badge {
            color: #00b67a;
            font-size: 14px;
            flex-shrink: 0;
        }
        .playlist-player {
            flex: 1;
            display: flex;
            flex-direction: column;
            min-width: 0;
            background: #0a1628;
        }
        .player-header {
            padding: 16px 24px;
            border-bottom: 1px solid #1e2d45;
            background: #111d32;
            flex-shrink: 0;
        }
        .player-header h1 {
            font-size: 24px;
            font-weight: 700;
            color: #ffffff;
            line-height: 1.3;
        }
        .player-meta {
            display: flex;
            align-items: center;
            gap: 16px;
            margin-top: 6px;
        }
        .player-meta-item {
            font-size: 13px;
            color: #8892a4;
        }
        .player-counter {
            font-size: 13px;
            color: #8892a4;
        }
        .player-container {
            flex: 1;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #000;
            position: relative;
            overflow: hidden;
            min-height: 300px;
        }
        .player-container video {
            width: 100%;
            max-height: calc(100vh - 220px);
            background: #000;
            display: block;
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
            font-size: 14px;
            color: #94a3b8;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            font-weight: 500;
            margin-bottom: 8px;
        }
        .next-overlay .next-title {
            font-size: 20px;
            font-weight: 600;
            margin-bottom: 8px;
            text-align: center;
            padding: 0 2rem;
            line-height: 1.3;
        }
        .next-countdown {
            font-size: 13px;
            color: #94a3b8;
            margin-bottom: 12px;
        }
        .next-progress-bar {
            width: 200px;
            height: 4px;
            background: #334155;
            border-radius: 2px;
            margin-bottom: 20px;
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
            gap: 12px;
        }
        .next-actions button {
            padding: 8px 20px;
            border-radius: 6px;
            font-size: 14px;
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
        .branding-footer {
            padding: 12px 24px;
            text-align: center;
            font-size: 12px;
            color: #8892a4;
            border-top: 1px solid #1e2d45;
            background: #111d32;
            flex-shrink: 0;
        }
        .branding-footer a {
            color: #8892a4;
            text-decoration: none;
            transition: color 0.15s;
        }
        .branding-footer a:hover { color: #ffffff; }
        @media (prefers-reduced-motion: reduce) {
            *, *::before, *::after {
                animation-duration: 0.01ms !important;
                transition-duration: 0.01ms !important;
            }
        }
        @media (max-width: 640px) {
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
            .sidebar-collapse-btn { display: block; }
            .playlist-player {
                order: 1;
            }
            .player-container { min-height: 200px; }
            .player-container video {
                max-height: 40vh;
            }
            .player-header { padding: 12px 16px; }
            .player-header h1 { font-size: 18px; }
            .volume-slider { display: none; }
        }
        {{end}}
    </style>
</head>
<body>
    {{if .NeedsPassword}}
    <div class="gate-container">
        <div class="gate-icon">&#128274;</div>
        <h1>{{.Title}}</h1>
        <p>This playlist is password protected</p>
        <p class="gate-error" id="error-msg"></p>
        <form id="password-form">
            <input type="password" id="password-input" placeholder="Enter password" required maxlength="128" autofocus>
            <button type="submit" id="submit-btn">Continue</button>
        </form>
        <div class="gate-branding">Powered by <a href="https://sendrec.eu" target="_blank" rel="noopener">SendRec</a></div>
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
        <div class="gate-icon">&#9993;</div>
        <h1>{{.Title}}</h1>
        <p>Enter your email to watch this playlist</p>
        <p class="gate-error" id="error-msg"></p>
        <form id="email-gate-form">
            <input type="email" id="email-input" placeholder="you@example.com" required maxlength="320" autofocus>
            <button type="submit" id="submit-btn">Watch Playlist</button>
        </form>
        <div class="gate-branding">Powered by <a href="https://sendrec.eu" target="_blank" rel="noopener">SendRec</a></div>
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
                <div class="sidebar-meta">
                    <span class="now-playing-label">Now playing <strong id="now-playing-num">1</strong> of {{len .Videos}}</span>
                    <div class="auto-advance-toggle" id="auto-advance-toggle" role="switch" aria-checked="true" aria-label="Auto-advance to next video" tabindex="0">
                        <span>Autoplay</span>
                        <div class="aa-toggle-track active" id="aa-toggle-track">
                            <div class="aa-toggle-knob"></div>
                        </div>
                    </div>
                </div>
            </div>
            <button class="sidebar-collapse-btn" id="sidebar-collapse-btn" aria-label="Toggle playlist sidebar">
                Playlist ({{len .Videos}} videos)
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>
            </button>
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
            <div class="player-header">
                <h1 id="current-title">{{if .Videos}}{{(index .Videos 0).Title}}{{end}}</h1>
                <div class="player-meta">
                    <span class="player-counter" id="player-counter">{{if .Videos}}1 of {{len .Videos}}{{end}}</span>
                </div>
            </div>
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
            <div class="branding-footer">
                Shared via <a href="https://sendrec.eu" target="_blank" rel="noopener">SendRec</a>
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
        var titleEl = document.getElementById('current-title');
        var counterEl = document.getElementById('player-counter');
        var nextOverlay = document.getElementById('next-overlay');
        var nextTitleEl = document.getElementById('next-title');
        var progressEl = document.getElementById('next-progress');
        var nextCountdownEl = document.getElementById('next-countdown');
        var listItems = document.querySelectorAll('.video-list-item');
        var countdownTimer = null;
        var autoAdvance = true;
        var storageKey = 'playlist_progress_{{.ShareToken}}';
        var nowPlayingNum = document.getElementById('now-playing-num');
        var aaToggle = document.getElementById('auto-advance-toggle');
        var aaTrack = document.getElementById('aa-toggle-track');
        var sidebarCollapseBtn = document.getElementById('sidebar-collapse-btn');
        if (sidebarCollapseBtn) {
            sidebarCollapseBtn.addEventListener('click', function() {
                var list = document.getElementById('video-list');
                if (list) {
                    list.classList.toggle('mobile-collapsed');
                    sidebarCollapseBtn.classList.toggle('collapsed');
                }
            });
        }
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
        var markersBar = document.getElementById('seek-markers');
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
        player.addEventListener('loadedmetadata', function() { renderCurrentMarkers(); });
        player.addEventListener('durationchange', function() { renderCurrentMarkers(); });

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

        // --- Comment markers ---
        var currentComments = [];

        function formatTimestamp(seconds) {
            var m = Math.floor(seconds / 60);
            var s = Math.floor(seconds % 60);
            return m + ':' + (s < 10 ? '0' : '') + s;
        }

        function renderMarkers(comments) {
            if (!markersBar) return;
            markersBar.innerHTML = '';
            var dur = getEffectiveDuration();
            if (!dur) return;
            var bySecond = {};
            comments.forEach(function(c) {
                if (c.videoTimestamp == null) return;
                var sec = Math.floor(c.videoTimestamp);
                if (!bySecond[sec]) bySecond[sec] = [];
                bySecond[sec].push(c);
            });
            var keys = Object.keys(bySecond);
            if (keys.length === 0) return;
            keys.forEach(function(sec) {
                var group = bySecond[sec];
                var dot = document.createElement('div');
                dot.className = 'seek-marker';
                var pct = Math.min(group[0].videoTimestamp / dur * 100, 99);
                dot.style.left = pct + '%';
                var tooltipText;
                if (group.length === 1) {
                    var author = group[0].authorName || 'Anonymous';
                    tooltipText = author + ' \u00b7 ' + formatTimestamp(group[0].videoTimestamp) + ' \u2014 ' + group[0].body.substring(0, 80);
                } else {
                    tooltipText = formatTimestamp(group[0].videoTimestamp) + ' \u2014 ' + group.length + ' comments';
                }
                var tooltip = document.createElement('div');
                tooltip.className = 'seek-marker-tooltip';
                tooltip.textContent = tooltipText;
                dot.appendChild(tooltip);
                dot.addEventListener('click', function(e) {
                    e.stopPropagation();
                    player.currentTime = group[0].videoTimestamp;
                });
                markersBar.appendChild(dot);
            });
        }

        function renderCurrentMarkers() {
            renderMarkers(currentComments);
        }

        function loadCommentsForVideo(shareToken) {
            currentComments = [];
            renderMarkers([]);
            fetch('/api/watch/' + encodeURIComponent(shareToken) + '/comments')
                .then(function(r) { return r.ok ? r.json() : []; })
                .then(function(comments) {
                    currentComments = comments || [];
                    renderMarkers(currentComments);
                })
                .catch(function() {});
        }

        // --- Switch video ---
        function switchVideo(index) {
            if (index < 0 || index >= videos.length) return;
            cancelCountdown();
            currentIndex = index;
            var v = videos[index];

            // Reset UI state
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
            titleEl.textContent = v.title;
            counterEl.textContent = (index + 1) + ' of ' + videos.length;
            if (nowPlayingNum) nowPlayingNum.textContent = (index + 1);
            listItems.forEach(function(li) {
                li.classList.toggle('active', parseInt(li.getAttribute('data-index'), 10) === index);
            });
            var li = listItems[index];
            if (li) { li.scrollIntoView({ block: 'nearest', behavior: 'smooth' }); }

            loadCommentsForVideo(v.shareToken);
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

        // Load comments for first video
        if (videos.length > 0) {
            loadCommentsForVideo(videos[0].shareToken);
        }
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
