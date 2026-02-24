package video

import (
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

type embedPageData struct {
	Title        string
	VideoURL     string
	ThumbnailURL string
	TranscriptURL string
	ShareToken   string
	Nonce        string
	BaseURL      string
	ContentType  string
	CtaText      string
	CtaUrl       string
	Chapters     []Chapter
	ChaptersJSON template.JS
}

type embedPasswordPageData struct {
	Title      string
	ShareToken string
	Nonce      string
}

type embedEmailGatePageData struct {
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
            display: block;
        }
        .player-container {
            position: relative;
            width: 100%;
            height: 100%;
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
        .player-error-icon { font-size: 30px; margin-bottom: 6px; }
        .seek-time-tooltip {
            position: absolute;
            bottom: 100%;
            transform: translateX(-50%);
            background: rgba(0, 0, 0, 0.85);
            color: #fff;
            padding: 2px 5px;
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
            padding: 20px 8px 6px;
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
            padding: 2px;
            line-height: 1;
            opacity: 0.9;
            flex-shrink: 0;
        }
        .ctrl-btn:hover { opacity: 1; }
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
            height: 16px;
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
            background: #22c55e;
            pointer-events: none;
        }
        .seek-chapters {
            position: absolute;
            left: 0;
            right: 0;
            top: 50%;
            height: 3px;
            transform: translateY(-50%);
        }
        .seek-bar:hover .seek-chapters { height: 5px; }
        .seek-chapter {
            position: absolute;
            top: 0;
            height: 100%;
            background: rgba(255, 255, 255, 0.15);
            cursor: pointer;
        }
        .seek-chapter:hover { background: rgba(255, 255, 255, 0.3); }
        .seek-chapter-tooltip {
            position: absolute;
            bottom: calc(100% + 6px);
            left: 50%;
            transform: translateX(-50%);
            background: #0f172a;
            color: #e2e8f0;
            padding: 3px 6px;
            border-radius: 3px;
            font-size: 10px;
            white-space: nowrap;
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.15s;
        }
        .seek-chapter:hover .seek-chapter-tooltip { opacity: 1; }
        .seek-thumb {
            position: absolute;
            width: 12px;
            height: 12px;
            background: #22c55e;
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
            gap: 2px;
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
            <div class="player-container" id="player-container">
                <video id="player" playsinline webkit-playsinline crossorigin="anonymous" controlsList="nodownload" src="{{.VideoURL}}"{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}>{{if .TranscriptURL}}<track kind="subtitles" src="{{.TranscriptURL}}" srclang="en" label="Subtitles">{{end}}</video>
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
                        <div class="seek-chapters" id="seek-chapters"></div>
                        <div class="seek-thumb" id="seek-thumb"></div>
                        <div class="seek-time-tooltip" id="seek-time-tooltip">0:00</div>
                    </div>
                    <span class="time-display" id="time-duration">0:00</span>
                    <div class="volume-group">
                        <button class="ctrl-btn" id="mute-btn" aria-label="Mute">&#128266;</button>
                        <input type="range" class="volume-slider" id="volume-slider" min="0" max="100" value="100">
                    </div>
                    <button class="ctrl-btn" id="fullscreen-btn" aria-label="Fullscreen">&#9974;</button>
                </div>
            </div>
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
            var player = document.getElementById('player');
            var container = document.getElementById('player-container');
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
            var fullscreenBtn = document.getElementById('fullscreen-btn');
            var spinner = document.getElementById('player-spinner');
            var errorOverlay = document.getElementById('player-error');
            var seekTooltip = document.getElementById('seek-time-tooltip');
            var hideTimer = null;

            player.muted = true;
            player.play().catch(function() {});

            function fmtTime(s) {
                if (!isFinite(s) || isNaN(s)) return '0:00';
                s = Math.floor(s);
                if (s >= 3600) return Math.floor(s/3600) + ':' + ('0'+Math.floor((s%3600)/60)).slice(-2) + ':' + ('0'+(s%60)).slice(-2);
                return Math.floor(s/60) + ':' + ('0'+(s%60)).slice(-2);
            }

            function updatePlayBtn() {
                var paused = player.paused;
                playBtn.innerHTML = paused ? '&#9654;' : '&#9646;&#9646;';
                overlay.classList.toggle('hidden', !paused);
            }
            function togglePlay() {
                if (player.paused) player.play().catch(function(){});
                else player.pause();
            }
            playBtn.addEventListener('click', togglePlay);
            overlayBtn.addEventListener('click', togglePlay);
            overlay.addEventListener('click', function(e) { if (e.target === overlay) togglePlay(); });
            player.addEventListener('play', function() { updatePlayBtn(); showControls(); });
            player.addEventListener('pause', function() { updatePlayBtn(); showControls(); });
            player.addEventListener('ended', updatePlayBtn);

            function updateProgress() {
                if (!player.duration || !isFinite(player.duration)) return;
                var pct = (player.currentTime / player.duration) * 100;
                seekProgress.style.width = pct + '%';
                seekThumb.style.left = pct + '%';
                timeCurrent.textContent = fmtTime(player.currentTime);
            }
            function updateBuffered() {
                if (!player.duration || !isFinite(player.duration) || !player.buffered.length) return;
                seekBuffered.style.width = (player.buffered.end(player.buffered.length - 1) / player.duration * 100) + '%';
            }
            function updateDurationDisplay() {
                if (player.duration && isFinite(player.duration)) timeDuration.textContent = fmtTime(player.duration);
            }
            player.addEventListener('timeupdate', updateProgress);
            player.addEventListener('progress', updateBuffered);
            player.addEventListener('loadedmetadata', function() { updateDurationDisplay(); updateProgress(); });
            player.addEventListener('durationchange', updateDurationDisplay);

            var seeking = false;
            function seekFromEvent(e) {
                var rect = seekBar.getBoundingClientRect();
                var pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
                if (player.duration && isFinite(player.duration)) {
                    player.currentTime = pct * player.duration;
                    updateProgress();
                }
            }
            seekBar.addEventListener('mousedown', function(e) { seeking = true; seekFromEvent(e); });
            document.addEventListener('mousemove', function(e) { if (seeking) seekFromEvent(e); });
            document.addEventListener('mouseup', function() { seeking = false; });
            seekBar.addEventListener('touchstart', function(e) { seeking = true; seekFromEvent(e.touches[0]); }, { passive: true });
            seekBar.addEventListener('touchmove', function(e) { if (seeking) seekFromEvent(e.touches[0]); }, { passive: true });
            seekBar.addEventListener('touchend', function() { seeking = false; });

            muteBtn.addEventListener('click', function() { player.muted = !player.muted; updateMuteBtn(); });
            function updateMuteBtn() {
                if (player.muted || player.volume === 0) muteBtn.innerHTML = '&#128264;';
                else if (player.volume < 0.5) muteBtn.innerHTML = '&#128265;';
                else muteBtn.innerHTML = '&#128266;';
                volumeSlider.value = player.muted ? 0 : player.volume * 100;
            }
            volumeSlider.addEventListener('input', function() {
                player.volume = volumeSlider.value / 100;
                player.muted = player.volume === 0;
                updateMuteBtn();
            });
            player.addEventListener('volumechange', updateMuteBtn);

            fullscreenBtn.addEventListener('click', function() {
                if (document.fullscreenElement) document.exitFullscreen().catch(function(){});
                else container.requestFullscreen().catch(function(){});
            });
            document.addEventListener('fullscreenchange', function() {
                fullscreenBtn.innerHTML = document.fullscreenElement ? '&#9723;' : '&#9974;';
            });

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
                if (!player.paused) hideTimer = setTimeout(function() { controls.classList.add('hidden'); }, 1000);
            });

            // Loading spinner
            player.addEventListener('waiting', function() { spinner.classList.add('visible'); });
            player.addEventListener('playing', function() { spinner.classList.remove('visible'); });
            player.addEventListener('canplay', function() { spinner.classList.remove('visible'); });

            // Error overlay
            player.addEventListener('error', function() {
                spinner.classList.remove('visible');
                errorOverlay.classList.add('visible');
                controls.classList.add('hidden');
            });

            // Seek bar time tooltip
            seekBar.addEventListener('mousemove', function(e) {
                if (!player.duration || !isFinite(player.duration)) return;
                var rect = seekBar.getBoundingClientRect();
                var pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
                var time = pct * player.duration;
                var label = fmtTime(time);
                var chapters = seekBar.querySelectorAll('.seek-chapter');
                for (var i = 0; i < chapters.length; i++) {
                    var ch = chapters[i];
                    var start = parseFloat(ch.dataset.start || 0);
                    var end = parseFloat(ch.dataset.end || 0);
                    if (time >= start && time < end && ch.dataset.title) {
                        label = ch.dataset.title + ' \u2013 ' + label;
                        break;
                    }
                }
                seekTooltip.textContent = label;
                seekTooltip.style.left = (pct * 100) + '%';
            });

            // Keyboard shortcuts
            document.addEventListener('keydown', function(e) {
                if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
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
                    case 'm':
                    case 'M':
                        player.muted = !player.muted;
                        break;
                    case 'f':
                    case 'F':
                        if (document.fullscreenElement) document.exitFullscreen().catch(function(){});
                        else container.requestFullscreen().catch(function(){});
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

            updatePlayBtn();
            updateMuteBtn();
        })();
        {{if and .CtaText .CtaUrl}}
        (function() {
            var player = document.getElementById('player');
            var overlay = document.getElementById('cta-overlay');
            var btn = document.getElementById('cta-btn');
            if (player && overlay) {
                player.addEventListener('ended', function() { overlay.classList.add('visible'); });
                player.addEventListener('play', function() { overlay.classList.remove('visible'); });
            }
            if (btn) {
                btn.addEventListener('click', function() {
                    fetch('/api/watch/{{.ShareToken}}/cta-click', { method: 'POST' }).catch(function() {});
                });
            }
        })();
        {{end}}
        {{if .Chapters}}
        (function() {
            var chaptersLayer = document.getElementById('seek-chapters');
            if (!chaptersLayer) return;
            var player = document.getElementById('player');
            var chapters = {{.ChaptersJSON}};

            function renderChapters() {
                var duration = player.duration;
                if (!duration || !isFinite(duration) || chapters.length === 0) return;
                chaptersLayer.innerHTML = '';
                for (var i = 0; i < chapters.length; i++) {
                    var start = chapters[i].start;
                    var end = (i + 1 < chapters.length) ? chapters[i + 1].start : duration;
                    var leftPct = (start / duration) * 100;
                    var widthPct = ((end - start) / duration) * 100;
                    var seg = document.createElement('div');
                    seg.className = 'seek-chapter';
                    seg.style.left = leftPct + '%';
                    seg.style.width = widthPct + '%';
                    seg.setAttribute('data-start', start);
                    seg.setAttribute('data-end', end);
                    seg.setAttribute('data-title', chapters[i].title);
                    seg.setAttribute('data-index', i);
                    if (i > 0) {
                        seg.style.left = (leftPct + 0.1) + '%';
                        seg.style.width = (widthPct - 0.1) + '%';
                    }
                    var tooltip = document.createElement('div');
                    tooltip.className = 'seek-chapter-tooltip';
                    tooltip.textContent = chapters[i].title;
                    seg.appendChild(tooltip);
                    seg.addEventListener('click', (function(s) {
                        return function(e) {
                            e.stopPropagation();
                            player.currentTime = s;
                            player.play().catch(function() {});
                        };
                    })(start));
                    chaptersLayer.appendChild(seg);
                }
            }

            player.addEventListener('timeupdate', function() {
                var segments = chaptersLayer.querySelectorAll('.seek-chapter');
                var currentTime = player.currentTime;
                segments.forEach(function(seg) {
                    var idx = parseInt(seg.getAttribute('data-index'));
                    var start = chapters[idx].start;
                    var end = (idx + 1 < chapters.length) ? chapters[idx + 1].start : player.duration;
                    seg.classList.toggle('active', currentTime >= start && currentTime < end);
                });
            });

            player.addEventListener('loadedmetadata', renderChapters);
            player.addEventListener('durationchange', renderChapters);
            if (player.duration && isFinite(player.duration)) renderChapters();
        })();
        {{end}}
        (function() {
            var player = document.getElementById('player');
            if (!player) return;
            var milestones = [25, 50, 75, 100];
            var reached = {};
            player.addEventListener('timeupdate', function() {
                if (!player.duration) return;
                var pct = (player.currentTime / player.duration) * 100;
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

var embedEmailGatePageTemplate = template.Must(template.New("embed-emailgate").Parse(`<!DOCTYPE html>
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
        input[type="email"]:focus { border-color: #22c55e; }
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
        <h1>Enter your email to watch this video</h1>
        <p>{{.Title}}</p>
        <p class="error" id="error-msg"></p>
        <form id="email-gate-form">
            <input type="email" id="email-input" placeholder="you@example.com" required maxlength="320" autofocus>
            <button type="submit" id="submit-btn">Watch Video</button>
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
            fetch('/api/watch/{{.ShareToken}}/identify', {
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
	var transcriptKey *string
	var emailGateEnabled bool
	var chaptersJSON *string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.title, v.file_key, u.name, v.created_at, v.share_expires_at,
		        v.thumbnail_key, v.share_password, v.content_type,
		        v.user_id, u.email, v.view_notification,
		        v.cta_text, v.cta_url, v.transcript_key,
		        v.email_gate_enabled, v.chapters
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID, &title, &fileKey, &creator, &createdAt, &shareExpiresAt,
		&thumbnailKey, &sharePassword, &contentType,
		&ownerID, &ownerEmail, &viewNotification,
		&ctaText, &ctaUrl, &transcriptKey,
		&emailGateEnabled, &chaptersJSON)
	if err != nil {
		nonce := httputil.NonceFromContext(r.Context())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		if err := notFoundPageTemplate.Execute(w, notFoundPageData{Nonce: nonce}); err != nil {
			slog.Error("embed-page: failed to render not found page", "error", err)
		}
		return
	}

	nonce := httputil.NonceFromContext(r.Context())

	if shareExpiresAt != nil && time.Now().After(*shareExpiresAt) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		if err := expiredPageTemplate.Execute(w, expiredPageData{Nonce: nonce}); err != nil {
			slog.Error("embed-page: failed to render expired page", "error", err)
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
				slog.Error("embed-page: failed to render password page", "error", err)
			}
			return
		}
	}

	if emailGateEnabled {
		if _, ok := hasValidEmailGateCookie(r, h.hmacSecret, shareToken); !ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := embedEmailGatePageTemplate.Execute(w, embedEmailGatePageData{
				Title:      title,
				ShareToken: shareToken,
				Nonce:      nonce,
			}); err != nil {
				slog.Error("embed-page: failed to render email gate page", "error", err)
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
			slog.Error("embed-page: failed to record view", "video_id", videoID, "error", err)
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

	var transcriptURL string
	if transcriptKey != nil {
		if u, err := h.storage.GenerateDownloadURL(r.Context(), *transcriptKey, 1*time.Hour); err == nil {
			transcriptURL = u
		}
	}

	chapterList := make([]Chapter, 0)
	if chaptersJSON != nil {
		_ = json.Unmarshal([]byte(*chaptersJSON), &chapterList)
	}
	chaptersJSONBytes, _ := json.Marshal(chapterList)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := embedPageTemplate.Execute(w, embedPageData{
		Title:        title,
		VideoURL:     videoURL,
		ThumbnailURL: thumbnailURL,
		TranscriptURL: transcriptURL,
		ShareToken:   shareToken,
		Nonce:        nonce,
		BaseURL:      h.baseURL,
		ContentType:  contentType,
		CtaText:      derefString(ctaText),
		CtaUrl:       derefString(ctaUrl),
		Chapters:     chapterList,
		ChaptersJSON: template.JS(chaptersJSONBytes),
	}); err != nil {
		slog.Error("embed-page: failed to render embed page", "error", err)
	}
}
