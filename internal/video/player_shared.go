package video

// playerCSS contains shared CSS for the custom video player controls.
// Pages should set --player-accent via their own CSS to customize the accent color.
const playerCSS = `
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
            pointer-events: none;
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
            pointer-events: none;
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
        .ctrl-btn:focus-visible { outline: 2px solid var(--player-accent, #00b67a); outline-offset: 2px; }
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
            background: var(--player-accent, #00b67a);
            pointer-events: none;
        }
        .seek-chapters {
            position: absolute;
            left: 0;
            right: 0;
            top: 50%;
            height: 4px;
            transform: translateY(-50%);
            pointer-events: none;
        }
        .seek-bar:hover .seek-chapters { height: 6px; }
        .seek-chapter {
            position: absolute;
            top: 0;
            height: 100%;
            background: rgba(255, 255, 255, 0.15);
            cursor: pointer;
            pointer-events: auto;
        }
        .seek-chapter:hover { background: rgba(255, 255, 255, 0.3); }
        .seek-chapter-tooltip {
            position: absolute;
            bottom: 36px;
            left: 50%;
            transform: translateX(-50%);
            background: #0f172a;
            color: #e2e8f0;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 11px;
            white-space: nowrap;
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.15s;
            z-index: 10;
        }
        .seek-chapter:hover .seek-chapter-tooltip { opacity: 1; }
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
            background: var(--player-accent, #00b67a);
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
            background: var(--player-accent, #00b67a);
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
        .speed-menu button.active { color: var(--player-accent, #00b67a); font-weight: 600; }
        .shortcuts-wrapper {
            position: relative;
        }
        .shortcuts-wrapper > .ctrl-btn {
            font-size: 14px;
            font-weight: 700;
        }
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
`

// playerControlsHTML contains the shared HTML for player overlay, spinner,
// error state, and controls bar. The <video> element stays per-page.
const playerControlsHTML = `
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
                    <div class="shortcuts-wrapper">
                        <button class="ctrl-btn" id="shortcuts-btn" aria-label="Keyboard shortcuts">?</button>
                        <div class="shortcuts-panel" id="shortcuts-panel">
                            <h4>Keyboard shortcuts</h4>
                            <table id="shortcuts-table">
                                <tr><td>Play / Pause</td><td><kbd>Space</kbd> <kbd>K</kbd></td></tr>
                                <tr><td>Rewind 5s</td><td><kbd>&#8592;</kbd></td></tr>
                                <tr><td>Forward 5s</td><td><kbd>&#8594;</kbd></td></tr>
                                <tr><td>Rewind 10s</td><td><kbd>J</kbd></td></tr>
                                <tr><td>Forward 10s</td><td><kbd>L</kbd></td></tr>
                                <tr><td>Mute</td><td><kbd>M</kbd></td></tr>
                                <tr><td>Fullscreen</td><td><kbd>F</kbd></td></tr>
                                <tr><td>Slower / Faster</td><td><kbd>&lt;</kbd> <kbd>&gt;</kbd></td></tr>
                                <tr><td>Seek to %</td><td><kbd>0</kbd>-<kbd>9</kbd></td></tr>
                            </table>
                        </div>
                    </div>
                    <button class="ctrl-btn" id="fullscreen-btn" aria-label="Fullscreen">&#9974;</button>
                </div>
`

// playerJS contains the shared JavaScript for the custom video player.
// It expects: player, container, controls, overlay, playBtn, overlayBtn,
// seekBar, seekProgress, seekBuffered, seekThumb, timeCurrent, timeDuration,
// muteBtn, volumeSlider, speedBtn, speedMenu, pipBtn, fullscreenBtn,
// spinner, errorOverlay, seekTooltip, shortcutsBtn, shortcutsPanel, hideTimer
// to be declared as variables before this code runs.
//
// Exposes onPlayerKeyOverride (function or null) for page-specific key handling.
const playerJS = `
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
        player.addEventListener('ended', updatePlayBtn);

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

        player.addEventListener('timeupdate', function() { updateProgress(); updateDurationDisplay(); });
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

        // Fullscreen (with webkit fallback for iOS Safari)
        function enterFullscreen() {
            if (container.requestFullscreen) container.requestFullscreen().catch(function(){});
            else if (container.webkitRequestFullscreen) container.webkitRequestFullscreen();
            else if (player.webkitEnterFullscreen) player.webkitEnterFullscreen();
        }
        function exitFullscreen() {
            if (document.exitFullscreen) document.exitFullscreen().catch(function(){});
            else if (document.webkitExitFullscreen) document.webkitExitFullscreen();
        }
        function isFullscreen() {
            return document.fullscreenElement || document.webkitFullscreenElement || false;
        }
        fullscreenBtn.addEventListener('click', function() {
            if (isFullscreen()) exitFullscreen();
            else enterFullscreen();
        });
        document.addEventListener('fullscreenchange', function() {
            fullscreenBtn.innerHTML = isFullscreen() ? '&#9723;' : '&#9974;';
            fullscreenBtn.setAttribute('aria-label', isFullscreen() ? 'Exit fullscreen' : 'Fullscreen');
        });
        document.addEventListener('webkitfullscreenchange', function() {
            fullscreenBtn.innerHTML = isFullscreen() ? '&#9723;' : '&#9974;';
            fullscreenBtn.setAttribute('aria-label', isFullscreen() ? 'Exit fullscreen' : 'Fullscreen');
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

        // Seek bar time tooltip with chapter name
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

        // Hook for page-specific key handling (e.g. N/P for playlists)
        var onPlayerKeyOverride = null;

        // Keyboard shortcuts
        document.addEventListener('keydown', function(e) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;
            if (onPlayerKeyOverride && onPlayerKeyOverride(e)) return;
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
                    if (isFullscreen()) exitFullscreen();
                    else enterFullscreen();
                    break;
                case '<':
                    player.playbackRate = Math.max(0.25, player.playbackRate - 0.25);
                    speedBtn.textContent = player.playbackRate + 'x';
                    break;
                case '>':
                    player.playbackRate = Math.min(4, player.playbackRate + 0.25);
                    speedBtn.textContent = player.playbackRate + 'x';
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

        updatePlayBtn();
        updateMuteBtn();

        // iOS Safari: fall back to native controls.
        // Custom controls have touch/playback issues on iOS; native controls work reliably.
        var isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) ||
                    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
        if (isIOS) {
            player.setAttribute('controls', '');
            controls.style.display = 'none';
            overlay.style.display = 'none';
        }
`

// safariWarningCSS contains the shared CSS for the Safari WebM warning banner.
const safariWarningCSS = `
        .browser-warning {
            background: #1e293b;
            border: 1px solid #f59e0b;
            border-radius: 8px;
            padding: 1rem;
            margin-top: 0.75rem;
            color: #fbbf24;
            font-size: 0.875rem;
            line-height: 1.5;
        }
`

// safariWarningHTML contains the shared HTML for the Safari WebM warning div.
const safariWarningHTML = `
        <div id="safari-webm-warning" class="hidden" role="alert">
            <p id="safari-webm-warning-text">This video was recorded in WebM format, which is not supported by Safari. Please open this link in Chrome or Firefox to watch.</p>
        </div>
`

// safariWarningJS contains the shared JS snippet that detects Safari + WebM and shows the warning.
// It checks <source type="video/webm">, src attributes ending in .webm, and for playlist pages,
// the contentType field in the videos JSON data.
// On iOS, it shows a gentler "processing" message and keeps the native player visible.
const safariWarningJS = `
            var isSafari = /^((?!chrome|android).)*safari/i.test(navigator.userAgent);
            if (isSafari) {
                var warningEl = document.getElementById('safari-webm-warning');
                var warningText = document.getElementById('safari-webm-warning-text');
                var playerEl = document.getElementById('player');
                var isIOSDevice = /iPad|iPhone|iPod/.test(navigator.userAgent) ||
                                  (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
                function checkWebM() {
                    if (!warningEl || !playerEl) return;
                    var src = playerEl.querySelector('source');
                    var isWebM = (src && src.getAttribute('type') === 'video/webm') ||
                                 (playerEl.src && playerEl.src.match(/\.webm(\?|$)/i));
                    if (isWebM) {
                        warningEl.className = 'browser-warning';
                        if (isIOSDevice) {
                            warningText.textContent = 'This video is still being processed. Please check back in a moment.';
                        } else {
                            playerEl.style.display = 'none';
                        }
                    } else {
                        warningEl.className = 'hidden';
                        playerEl.style.display = '';
                    }
                }
                checkWebM();
                playerEl.addEventListener('loadstart', checkWebM);
            }
`
