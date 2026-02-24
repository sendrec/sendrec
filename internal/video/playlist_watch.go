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
            background: #0f172a;
            color: #e2e8f0;
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
            border-bottom: 1px solid #1e293b;
        }
        .player-header h1 {
            font-size: 1.25rem;
            font-weight: 600;
            color: #f1f5f9;
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
            overflow: hidden;
        }
        .player-container video {
            width: 100%;
            max-height: 70vh;
            border-radius: 4px;
            background: #000;
        }
        .player-overlay {
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            z-index: 2;
        }
        .player-overlay.hidden { display: none; }
        .play-overlay-btn {
            width: 64px;
            height: 64px;
            border-radius: 50%;
            background: rgba(0, 0, 0, 0.6);
            border: none;
            color: #fff;
            font-size: 28px;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            backdrop-filter: blur(4px);
            transition: background 0.2s;
        }
        .play-overlay-btn:hover { background: rgba(0, 0, 0, 0.8); }
        .player-spinner {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            width: 48px;
            height: 48px;
            border: 4px solid rgba(255, 255, 255, 0.2);
            border-top-color: #fff;
            border-radius: 50%;
            animation: spin 0.8s linear infinite;
            z-index: 4;
            display: none;
        }
        .player-spinner.visible { display: block; }
        @keyframes spin { to { transform: translate(-50%, -50%) rotate(360deg); } }
        .player-error {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            text-align: center;
            color: #e2e8f0;
            font-size: 14px;
            z-index: 4;
            display: none;
        }
        .player-error.visible { display: block; }
        .player-error-icon { font-size: 36px; margin-bottom: 8px; }
        .seek-time-tooltip {
            position: absolute;
            bottom: 100%;
            transform: translateX(-50%);
            background: rgba(0, 0, 0, 0.85);
            color: #fff;
            padding: 3px 7px;
            border-radius: 4px;
            font-size: 11px;
            font-family: monospace;
            white-space: nowrap;
            pointer-events: none;
            display: none;
            margin-bottom: 6px;
        }
        .seek-bar:hover .seek-time-tooltip { display: block; }
        .player-controls {
            position: absolute;
            bottom: 0;
            left: 0;
            right: 0;
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 24px 12px 10px;
            background: linear-gradient(transparent, rgba(0, 0, 0, 0.85));
            z-index: 3;
            transition: opacity 0.3s;
        }
        .player-controls.hidden { opacity: 0; pointer-events: none; }
        .ctrl-btn {
            background: none;
            border: none;
            color: #fff;
            font-size: 18px;
            cursor: pointer;
            padding: 4px;
            line-height: 1;
            opacity: 0.9;
            flex-shrink: 0;
        }
        .ctrl-btn:hover { opacity: 1; }
        .ctrl-btn:focus-visible { outline: 2px solid #00b67a; outline-offset: 2px; }
        .time-display {
            font-size: 12px;
            color: #fff;
            font-family: monospace;
            white-space: nowrap;
            flex-shrink: 0;
            opacity: 0.9;
        }
        .seek-bar {
            position: relative;
            flex: 1;
            height: 20px;
            display: flex;
            align-items: center;
            cursor: pointer;
        }
        .seek-track {
            position: absolute;
            left: 0;
            right: 0;
            height: 4px;
            background: rgba(255, 255, 255, 0.2);
            border-radius: 2px;
            overflow: hidden;
            transition: height 0.15s;
        }
        .seek-bar:hover .seek-track { height: 6px; }
        .seek-buffered {
            position: absolute;
            top: 0;
            left: 0;
            height: 100%;
            background: rgba(255, 255, 255, 0.3);
        }
        .seek-progress {
            position: absolute;
            top: 0;
            left: 0;
            height: 100%;
            background: #00b67a;
            pointer-events: none;
        }
        .seek-markers {
            position: absolute;
            left: 0;
            right: 0;
            top: 50%;
            height: 4px;
            transform: translateY(-50%);
            pointer-events: none;
        }
        .seek-bar:hover .seek-markers { height: 6px; }
        .seek-marker {
            position: absolute;
            width: 6px;
            height: 100%;
            background: #00b67a;
            border-radius: 1px;
            transform: translateX(-50%);
            opacity: 0.8;
            cursor: pointer;
            pointer-events: auto;
        }
        .seek-marker:hover { opacity: 1; transform: translateX(-50%); }
        .seek-marker-tooltip {
            position: absolute;
            bottom: 36px;
            left: 50%;
            transform: translateX(-50%);
            background: #0f172a;
            border: 1px solid #334155;
            border-radius: 6px;
            padding: 4px 8px;
            font-size: 11px;
            color: #e2e8f0;
            white-space: nowrap;
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.15s;
            z-index: 10;
        }
        .seek-marker:hover .seek-marker-tooltip { opacity: 1; }
        .seek-thumb {
            position: absolute;
            width: 14px;
            height: 14px;
            background: #00b67a;
            border-radius: 50%;
            top: 50%;
            transform: translate(-50%, -50%);
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.15s;
        }
        .seek-bar:hover .seek-thumb { opacity: 1; }
        .volume-group {
            display: flex;
            align-items: center;
            gap: 4px;
            flex-shrink: 0;
        }
        .volume-slider {
            width: 60px;
            height: 4px;
            -webkit-appearance: none;
            appearance: none;
            background: rgba(255, 255, 255, 0.3);
            border-radius: 2px;
            outline: none;
        }
        .volume-slider::-webkit-slider-thumb {
            -webkit-appearance: none;
            width: 12px;
            height: 12px;
            background: #fff;
            border-radius: 50%;
            cursor: pointer;
        }
        .volume-slider::-moz-range-thumb {
            width: 12px;
            height: 12px;
            background: #fff;
            border-radius: 50%;
            cursor: pointer;
            border: none;
        }
        .speed-dropdown {
            position: relative;
            flex-shrink: 0;
        }
        .speed-menu {
            display: none;
            position: absolute;
            bottom: 100%;
            right: 0;
            margin-bottom: 8px;
            background: rgba(15, 23, 42, 0.95);
            border: 1px solid #334155;
            border-radius: 6px;
            padding: 4px;
            min-width: 56px;
        }
        .speed-menu.open { display: block; }
        .speed-menu button {
            display: block;
            width: 100%;
            background: none;
            border: none;
            color: #e2e8f0;
            padding: 5px 10px;
            font-size: 12px;
            cursor: pointer;
            border-radius: 4px;
            text-align: center;
        }
        .speed-menu button:hover { background: rgba(255, 255, 255, 0.1); }
        .speed-menu button.active { color: #00b67a; font-weight: 600; }
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
            border-top: 1px solid #1e293b;
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
            .volume-slider { display: none; }
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
            <div class="player-container" id="player-container">
                <video id="player" playsinline{{if .Videos}} src="{{(index .Videos 0).VideoURL}}"{{end}}></video>
                <div class="player-overlay" id="player-overlay">
                    <button class="play-overlay-btn" id="play-overlay-btn" aria-label="Play">&#9654;</button>
                </div>
                <div class="player-spinner" id="player-spinner"></div>
                <div class="player-error" id="player-error"><div class="player-error-icon">&#9888;</div>Video failed to load</div>
                <div class="player-controls" id="player-controls">
                    <button class="ctrl-btn" id="play-btn" aria-label="Play">&#9654;</button>
                    <span class="time-display" id="time-current">0:00</span>
                    <div class="seek-bar" id="seek-bar">
                        <div class="seek-track">
                            <div class="seek-buffered" id="seek-buffered"></div>
                            <div class="seek-progress" id="seek-progress"></div>
                        </div>
                        <div class="seek-markers" id="seek-markers"></div>
                        <div class="seek-thumb" id="seek-thumb"></div>
                        <div class="seek-time-tooltip" id="seek-time-tooltip">0:00</div>
                    </div>
                    <span class="time-display" id="time-duration">0:00</span>
                    <div class="volume-group">
                        <button class="ctrl-btn" id="mute-btn" aria-label="Mute">&#128266;</button>
                        <input type="range" class="volume-slider" id="volume-slider" min="0" max="100" value="100">
                    </div>
                    <div class="speed-dropdown" id="speed-dropdown">
                        <button class="ctrl-btn" id="speed-btn" aria-label="Playback speed">1x</button>
                        <div class="speed-menu" id="speed-menu">
                            <button data-speed="0.5">0.5x</button>
                            <button data-speed="0.75">0.75x</button>
                            <button data-speed="1" class="active">1x</button>
                            <button data-speed="1.25">1.25x</button>
                            <button data-speed="1.5">1.5x</button>
                            <button data-speed="2">2x</button>
                        </div>
                    </div>
                    <button class="ctrl-btn" id="pip-btn" aria-label="Picture in Picture">&#9114;</button>
                    <button class="ctrl-btn" id="fullscreen-btn" aria-label="Fullscreen">&#9974;</button>
                </div>
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
        var container = document.getElementById('player-container');
        var titleEl = document.getElementById('current-title');
        var counterEl = document.getElementById('player-counter');
        var nextOverlay = document.getElementById('next-overlay');
        var nextTitleEl = document.getElementById('next-title');
        var progressEl = document.getElementById('next-progress');
        var listItems = document.querySelectorAll('.video-list-item');
        var countdownTimer = null;
        var storageKey = 'playlist_progress_{{.ShareToken}}';

        // Custom player elements
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

        // --- Custom player controls ---
        function fmtTime(s) {
            if (!isFinite(s) || isNaN(s)) return '0:00';
            s = Math.floor(s);
            if (s >= 3600) return Math.floor(s/3600) + ':' + ('0'+Math.floor((s%3600)/60)).slice(-2) + ':' + ('0'+(s%60)).slice(-2);
            return Math.floor(s/60) + ':' + ('0'+(s%60)).slice(-2);
        }

        function updatePlayBtn() {
            var paused = player.paused;
            playBtn.innerHTML = paused ? '&#9654;' : '&#9646;&#9646;';
            playBtn.setAttribute('aria-label', paused ? 'Play' : 'Pause');
            overlay.classList.toggle('hidden', !paused);
            overlayBtn.innerHTML = paused ? '&#9654;' : '';
        }

        function togglePlay() {
            if (player.paused) player.play().catch(function(){});
            else player.pause();
        }

        playBtn.addEventListener('click', togglePlay);
        overlayBtn.addEventListener('click', togglePlay);
        overlay.addEventListener('click', function(e) {
            if (e.target === overlay) togglePlay();
        });

        player.addEventListener('play', function() { updatePlayBtn(); showControls(); });
        player.addEventListener('pause', function() { updatePlayBtn(); showControls(); });

        function getEffectiveDuration() {
            if (player.duration && isFinite(player.duration)) return player.duration;
            var best = player.currentTime || 0;
            if (player.buffered.length) {
                var end = player.buffered.end(player.buffered.length - 1);
                if (end > best) best = end;
            }
            return best;
        }

        function updateProgress() {
            var dur = getEffectiveDuration();
            if (!dur) return;
            var pct = Math.min((player.currentTime / dur) * 100, 100);
            seekProgress.style.width = pct + '%';
            seekThumb.style.left = pct + '%';
            timeCurrent.textContent = fmtTime(player.currentTime);
        }

        function updateBuffered() {
            var dur = getEffectiveDuration();
            if (!dur || !player.buffered.length) return;
            var end = player.buffered.end(player.buffered.length - 1);
            seekBuffered.style.width = (end / dur * 100) + '%';
        }

        function updateDurationDisplay() {
            var dur = getEffectiveDuration();
            if (dur) timeDuration.textContent = fmtTime(dur);
        }

        player.addEventListener('timeupdate', function() {
            updateProgress();
            updateDurationDisplay();
            // Mark watched at 80%
            if (player.duration > 0 && player.currentTime / player.duration > 0.8) {
                markWatched(videos[currentIndex].id);
            }
        });
        player.addEventListener('progress', function() { updateBuffered(); updateDurationDisplay(); });
        player.addEventListener('loadedmetadata', function() { updateDurationDisplay(); updateProgress(); renderCurrentMarkers(); });
        player.addEventListener('durationchange', function() { updateDurationDisplay(); updateProgress(); renderCurrentMarkers(); });

        // Seek bar
        var seeking = false;
        function seekFromEvent(e) {
            var rect = seekBar.getBoundingClientRect();
            var pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
            var dur = getEffectiveDuration();
            if (dur) {
                player.currentTime = pct * dur;
                updateProgress();
            }
        }
        seekBar.addEventListener('mousedown', function(e) {
            seeking = true;
            seekFromEvent(e);
        });
        document.addEventListener('mousemove', function(e) {
            if (seeking) seekFromEvent(e);
        });
        document.addEventListener('mouseup', function() { seeking = false; });
        seekBar.addEventListener('touchstart', function(e) {
            seeking = true;
            seekFromEvent(e.touches[0]);
        }, { passive: true });
        seekBar.addEventListener('touchmove', function(e) {
            if (seeking) seekFromEvent(e.touches[0]);
        }, { passive: true });
        seekBar.addEventListener('touchend', function() { seeking = false; });

        // Volume
        muteBtn.addEventListener('click', function() {
            player.muted = !player.muted;
            updateMuteBtn();
        });
        function updateMuteBtn() {
            if (player.muted || player.volume === 0) muteBtn.innerHTML = '&#128264;';
            else if (player.volume < 0.5) muteBtn.innerHTML = '&#128265;';
            else muteBtn.innerHTML = '&#128266;';
            muteBtn.setAttribute('aria-label', player.muted ? 'Unmute' : 'Mute');
            volumeSlider.value = player.muted ? 0 : player.volume * 100;
        }
        volumeSlider.addEventListener('input', function() {
            player.volume = volumeSlider.value / 100;
            player.muted = player.volume === 0;
            updateMuteBtn();
        });
        player.addEventListener('volumechange', updateMuteBtn);

        // Speed
        speedBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            speedMenu.classList.toggle('open');
        });
        speedMenu.addEventListener('click', function(e) {
            var btn = e.target.closest('button[data-speed]');
            if (!btn) return;
            player.playbackRate = parseFloat(btn.dataset.speed);
            speedBtn.textContent = btn.textContent;
            speedMenu.querySelectorAll('button').forEach(function(b) { b.classList.remove('active'); });
            btn.classList.add('active');
            speedMenu.classList.remove('open');
        });
        document.addEventListener('click', function(e) {
            if (!e.target.closest('#speed-dropdown')) speedMenu.classList.remove('open');
        });

        // PiP
        if (document.pictureInPictureEnabled) {
            pipBtn.addEventListener('click', function() {
                if (document.pictureInPictureElement) document.exitPictureInPicture().catch(function(){});
                else player.requestPictureInPicture().catch(function(){});
            });
        } else {
            pipBtn.style.display = 'none';
        }

        // Fullscreen
        fullscreenBtn.addEventListener('click', function() {
            if (document.fullscreenElement) document.exitFullscreen().catch(function(){});
            else container.requestFullscreen().catch(function(){});
        });
        document.addEventListener('fullscreenchange', function() {
            fullscreenBtn.innerHTML = document.fullscreenElement ? '&#9723;' : '&#9974;';
            fullscreenBtn.setAttribute('aria-label', document.fullscreenElement ? 'Exit fullscreen' : 'Fullscreen');
        });

        // Auto-hide controls
        function showControls() {
            controls.classList.remove('hidden');
            clearTimeout(hideTimer);
            if (!player.paused) {
                hideTimer = setTimeout(function() { controls.classList.add('hidden'); }, 3000);
            }
        }
        container.addEventListener('mousemove', showControls);
        container.addEventListener('touchstart', showControls, { passive: true });
        container.addEventListener('mouseleave', function() {
            if (!player.paused) {
                hideTimer = setTimeout(function() { controls.classList.add('hidden'); }, 1000);
            }
        });

        // Spinner
        player.addEventListener('waiting', function() { spinner.classList.add('visible'); });
        player.addEventListener('playing', function() { spinner.classList.remove('visible'); });
        player.addEventListener('canplay', function() { spinner.classList.remove('visible'); });

        // Error overlay
        player.addEventListener('error', function() {
            spinner.classList.remove('visible');
            errorOverlay.classList.add('visible');
            controls.classList.add('hidden');
        });

        // Seek time tooltip
        seekBar.addEventListener('mousemove', function(e) {
            if (!player.duration || !isFinite(player.duration)) return;
            var rect = seekBar.getBoundingClientRect();
            var pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
            var time = pct * player.duration;
            seekTooltip.textContent = fmtTime(time);
            seekTooltip.style.left = (pct * 100) + '%';
        });

        // Keyboard shortcuts
        document.addEventListener('keydown', function(e) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;
            var handled = true;
            switch (e.key) {
                case ' ':
                case 'k':
                case 'K':
                    togglePlay();
                    break;
                case 'ArrowLeft':
                    player.currentTime = Math.max(0, player.currentTime - 5);
                    break;
                case 'ArrowRight':
                    player.currentTime = Math.min(player.duration || 0, player.currentTime + 5);
                    break;
                case 'j':
                case 'J':
                    player.currentTime = Math.max(0, player.currentTime - 10);
                    break;
                case 'l':
                case 'L':
                    player.currentTime = Math.min(player.duration || 0, player.currentTime + 10);
                    break;
                case 'm':
                case 'M':
                    player.muted = !player.muted;
                    break;
                case 'f':
                case 'F':
                    if (document.fullscreenElement) document.exitFullscreen().catch(function(){});
                    else container.requestFullscreen().catch(function(){});
                    break;
                case '<':
                    player.playbackRate = Math.max(0.25, player.playbackRate - 0.25);
                    speedBtn.textContent = player.playbackRate + 'x';
                    break;
                case '>':
                    player.playbackRate = Math.min(4, player.playbackRate + 0.25);
                    speedBtn.textContent = player.playbackRate + 'x';
                    break;
                case 'n':
                case 'N':
                    if (currentIndex < videos.length - 1) switchVideo(currentIndex + 1);
                    break;
                case 'p':
                case 'P':
                    if (currentIndex > 0) switchVideo(currentIndex - 1);
                    break;
                default:
                    if (e.key >= '0' && e.key <= '9' && player.duration) {
                        player.currentTime = (parseInt(e.key) / 10) * player.duration;
                    } else {
                        handled = false;
                    }
            }
            if (handled) e.preventDefault();
        });

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

        // Initialize
        updatePlayBtn();
        updateMuteBtn();
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
