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
            width: 56px;
            height: 56px;
            border-radius: 50%;
            background: rgba(0, 0, 0, 0.6);
            border: none;
            color: #fff;
            font-size: 24px;
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
            width: 40px;
            height: 40px;
            border: 3px solid rgba(255, 255, 255, 0.2);
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
            font-size: 13px;
            z-index: 4;
            display: none;
        }
        .player-error.visible { display: block; }
        .player-error-icon { font-size: 32px; margin-bottom: 6px; }
        .seek-time-tooltip {
            position: absolute;
            bottom: 100%;
            transform: translateX(-50%);
            background: rgba(0, 0, 0, 0.85);
            color: #fff;
            padding: 2px 6px;
            border-radius: 3px;
            font-size: 10px;
            font-family: monospace;
            white-space: nowrap;
            pointer-events: none;
            display: none;
            margin-bottom: 4px;
        }
        .seek-bar:hover .seek-time-tooltip { display: block; }
        .player-controls {
            position: absolute;
            bottom: 0;
            left: 0;
            right: 0;
            display: flex;
            align-items: center;
            gap: 6px;
            padding: 20px 10px 8px;
            background: linear-gradient(transparent, rgba(0, 0, 0, 0.85));
            z-index: 3;
            transition: opacity 0.3s;
        }
        .player-controls.hidden { opacity: 0; pointer-events: none; }
        .ctrl-btn {
            background: none;
            border: none;
            color: #fff;
            font-size: 16px;
            cursor: pointer;
            padding: 3px;
            line-height: 1;
            opacity: 0.9;
            flex-shrink: 0;
        }
        .ctrl-btn:hover { opacity: 1; }
        .ctrl-btn:focus-visible { outline: 2px solid #00b67a; outline-offset: 2px; }
        .time-display {
            font-size: 11px;
            color: #fff;
            font-family: monospace;
            white-space: nowrap;
            flex-shrink: 0;
            opacity: 0.9;
        }
        .seek-bar {
            position: relative;
            flex: 1;
            height: 18px;
            display: flex;
            align-items: center;
            cursor: pointer;
        }
        .seek-track {
            position: absolute;
            left: 0;
            right: 0;
            height: 3px;
            background: rgba(255, 255, 255, 0.2);
            border-radius: 2px;
            overflow: hidden;
            transition: height 0.15s;
        }
        .seek-bar:hover .seek-track { height: 5px; }
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
        .seek-thumb {
            position: absolute;
            width: 12px;
            height: 12px;
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
            gap: 3px;
            flex-shrink: 0;
        }
        .volume-slider {
            width: 50px;
            height: 3px;
            -webkit-appearance: none;
            appearance: none;
            background: rgba(255, 255, 255, 0.3);
            border-radius: 2px;
            outline: none;
        }
        .volume-slider::-webkit-slider-thumb {
            -webkit-appearance: none;
            width: 10px;
            height: 10px;
            background: #fff;
            border-radius: 50%;
            cursor: pointer;
        }
        .volume-slider::-moz-range-thumb {
            width: 10px;
            height: 10px;
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
            margin-bottom: 6px;
            background: rgba(15, 23, 42, 0.95);
            border: 1px solid #334155;
            border-radius: 5px;
            padding: 3px;
            min-width: 48px;
        }
        .speed-menu.open { display: block; }
        .speed-menu button {
            display: block;
            width: 100%;
            background: none;
            border: none;
            color: #e2e8f0;
            padding: 4px 8px;
            font-size: 11px;
            cursor: pointer;
            border-radius: 3px;
            text-align: center;
        }
        .speed-menu button:hover { background: rgba(255, 255, 255, 0.1); }
        .speed-menu button.active { color: #00b67a; font-weight: 600; }
        .shortcuts-panel {
            display: none;
            position: absolute;
            bottom: 52px;
            right: 8px;
            background: rgba(15, 23, 42, 0.95);
            border: 1px solid #334155;
            border-radius: 8px;
            padding: 12px 16px;
            z-index: 20;
            color: #e2e8f0;
            font-size: 12px;
            min-width: 220px;
        }
        .shortcuts-panel.open { display: block; }
        .shortcuts-panel h4 {
            margin: 0 0 8px;
            font-size: 13px;
            color: #fff;
        }
        .shortcuts-panel table {
            width: 100%;
            border-collapse: collapse;
        }
        .shortcuts-panel td {
            padding: 2px 0;
        }
        .shortcuts-panel td:first-child {
            color: #94a3b8;
            padding-right: 12px;
            white-space: nowrap;
        }
        .shortcuts-panel kbd {
            background: rgba(255, 255, 255, 0.1);
            border: 1px solid #475569;
            border-radius: 3px;
            padding: 1px 5px;
            font-family: monospace;
            font-size: 11px;
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
                    <div style="position:relative">
                        <button class="ctrl-btn" id="shortcuts-btn" aria-label="Keyboard shortcuts" style="font-size:14px;font-weight:700">?</button>
                        <div class="shortcuts-panel" id="shortcuts-panel">
                            <h4>Keyboard shortcuts</h4>
                            <table>
                                <tr><td>Play / Pause</td><td><kbd>Space</kbd> <kbd>K</kbd></td></tr>
                                <tr><td>Rewind 5s</td><td><kbd>&#8592;</kbd></td></tr>
                                <tr><td>Forward 5s</td><td><kbd>&#8594;</kbd></td></tr>
                                <tr><td>Rewind 10s</td><td><kbd>J</kbd></td></tr>
                                <tr><td>Forward 10s</td><td><kbd>L</kbd></td></tr>
                                <tr><td>Mute</td><td><kbd>M</kbd></td></tr>
                                <tr><td>Fullscreen</td><td><kbd>F</kbd></td></tr>
                                <tr><td>Slower / Faster</td><td><kbd>&lt;</kbd> <kbd>&gt;</kbd></td></tr>
                                <tr><td>Seek to %</td><td><kbd>0</kbd>-<kbd>9</kbd></td></tr>
                                <tr><td>Next video</td><td><kbd>N</kbd></td></tr>
                                <tr><td>Previous video</td><td><kbd>P</kbd></td></tr>
                            </table>
                        </div>
                    </div>
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
            <div class="footer">
                <span class="footer-title">{{.Title}}</span>
                <a href="{{.BaseURL}}/watch/playlist/{{.ShareToken}}" target="_blank" rel="noopener">Watch on SendRec</a>
            </div>
        </main>
    </div>
    <script nonce="{{.Nonce}}">
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
            if (player.duration > 0 && player.currentTime / player.duration > 0.8) {
                markWatched(videos[currentIndex].id);
            }
        });
        player.addEventListener('progress', function() { updateBuffered(); updateDurationDisplay(); });
        player.addEventListener('loadedmetadata', function() { updateDurationDisplay(); updateProgress(); });
        player.addEventListener('durationchange', function() { updateDurationDisplay(); updateProgress(); });

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
            if (!e.target.closest('#shortcuts-btn') && !e.target.closest('#shortcuts-panel')) shortcutsPanel.classList.remove('open');
        });
        shortcutsBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            shortcutsPanel.classList.toggle('open');
            speedMenu.classList.remove('open');
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
                case '?':
                    shortcutsPanel.classList.toggle('open');
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

        // Initialize
        updatePlayBtn();
        updateMuteBtn();
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
