package video

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatDuration(totalSeconds int) string {
	if totalSeconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", totalSeconds/3600, (totalSeconds%3600)/60, totalSeconds%60)
	}
	return fmt.Sprintf("%d:%02d", totalSeconds/60, totalSeconds%60)
}

func formatISO8601Duration(totalSeconds int) string {
	if totalSeconds <= 0 {
		return ""
	}
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	if h > 0 {
		return fmt.Sprintf("PT%dH%dM%dS", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("PT%dM%dS", m, s)
	}
	return fmt.Sprintf("PT%dS", s)
}

func buildVideoObjectJSONLD(title, description, baseURL, shareToken string, createdAt time.Time, duration int, downloadEnabled, hasThumbnail bool) string {
	obj := map[string]string{
		"@context":    "https://schema.org",
		"@type":       "VideoObject",
		"name":        title,
		"description": description,
		"uploadDate":  createdAt.Format(time.RFC3339),
		"embedUrl":    baseURL + "/embed/" + shareToken,
	}
	if hasThumbnail {
		obj["thumbnailUrl"] = baseURL + "/api/watch/" + shareToken + "/thumbnail"
	}
	iso := formatISO8601Duration(duration)
	if iso != "" {
		obj["duration"] = iso
	}
	if downloadEnabled {
		obj["contentUrl"] = baseURL + "/api/watch/" + shareToken + "/download"
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

var watchFuncs = template.FuncMap{
	"formatTimestamp": func(seconds float64) string {
		return formatDuration(int(seconds))
	},
}

var watchPageTemplate = template.Must(template.New("watch").Funcs(watchFuncs).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="icon" type="image/png" sizes="32x32" href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAABGdBTUEAALGPC/xhBQAAACBjSFJNAAB6JgAAgIQAAPoAAACA6AAAdTAAAOpgAAA6mAAAF3CculE8AAAAeGVYSWZNTQAqAAAACAAEARoABQAAAAEAAAA+ARsABQAAAAEAAABGASgAAwAAAAEAAgAAh2kABAAAAAEAAABOAAAAAAAAAEgAAAABAAAASAAAAAEAA6ABAAMAAAABAAEAAKACAAQAAAABAAAAIKADAAQAAAABAAAAIAAAAACfCVbEAAAACXBIWXMAAAsTAAALEwEAmpwYAAAEa0lEQVRYCe1VW2icRRQ+Z+af/xK1kdQlvlhatWC7tamKoIilVrFSjdhoIqZtQBERn7RKXwQJvigUqy8iIQ9KU4Msrq0+eHlJUChiIWLTpFHrgxAkIiE2t+a/zMzxzCYb/t3ECyj4spNsZnbmzPm+850zJwCN0VCgocD/rAD+V/i3fFoqxHHc6ilPo68nR/f1LP4T3/+aQLF88nbjyaNkabew1AKIBqSYJKKSDOUb4w90zfwVkb8lsP2TwSKCuMuQ3miF6P+xvXu66nBb+cRhUOodkN4VNssA7cqJFICBAkrTc5jajonHDl6s3qmf/5TAztKJLZmvXrOCHhZSRSIIwC7MHZt45NBR52Rn+eQdqSeHgCACY4AjB4684p/sMhPh+0BJ8m1zCPd8s//QXD24+758o+6k+NFAWxqqIQjU42ApsmkKZn7BmT+7/cP3is48ldALSkbE4OSA3ceaKbL6EipZ8WiTFDCKbp2N4bnKxjp/1hDYderdqw2I91nWzXYpYaccnJCAygPZvOEqQnFgx8eDrUS4G1LNgXOCUGhp9ItKYdHzqA20KaNSFTjKKjade4Z7vXXwYc1mTN5TGAXFCjirip4HmGVjwsCItXZOePgBoS4KGUTgnDMQ6fTr8Y7Dx1cAfucgXoo17kOJV7Iqbvv6+ZkbCjxPrdisTjUEOkslOYZpFxkLSMTOfUCTDXKJPZN/VjeVB/bKQHD62UYwS8KJVY+8iEnMA1EKwglMTkUv9mlZkrwhr2tS8IMXb7QEN6JmAk52qxcFyVfy4O6+QNziao5/XX7cPOmW1SGF9zSnrAWI/VR+aAaWwnWfY40CRkOIEgJXxS63/LZnm8Xib1XH1dkibHJr6/LP4bHl9zvKAw9SGB7kgi0Ywr3uZTiC7jlCbL4af7TLVfGaUaOAyOQ8a7/AQbFyjoQozKb+zflbe4aHPY5ql0uTG6S5Dsj+zGQewih8Ajz/PkZmXiw9p4CyLGYt38r7yK8rKq5ucEq3nR74HP3wfuInxE0GUOtzYLInizYavQBzzaSil8nzjjjgivxkLjUtia1JqAtGqrPcAbnwHDizkDLFLDky3tHz9ipG3aJGAaelsNjvCrCSYa5yfo5tVnhnRlU6YlX4HTA4OHCHEfggLHwx0t09PdbRM4HW9EluPq4qBHFsxsSC9Gd1mDVfawnwUeH8T6etTk9hU1DhQDpjJIgkE0Epr3PgyPJLfp6QJNOCst6qRyHEMZskU64pkUuhUhsI1KvV8/Xm2hSsWBRLpRbrJwOciv3cVMDyx4W80nQYXPFe9gvZrIflHco75n9Oz0MYvkkpNzH2zqQMarr3fEf3l3m76nqNAu5gvKtrRqTBAUySF7gYL7Ajjd5ye2VlfzU67Rdpdnc9uLsrF22/TZZGXed0BMCTksC+fltfX5M7rx/rKpA3urN0PJoPrt3K+WwliZelsRdHO3rWPM38He6Em43ftEnHS8BpI5FpGYTXnB1pb7+ct2usGwo0FGgo4BT4A0kx06ZKzSjiAAAAAElFTkSuQmCC">
    <title>{{.Title}} — {{.Branding.CompanyName}}</title>
    <link rel="canonical" href="{{.BaseURL}}/watch/{{.ShareToken}}">
    <meta name="description" content="{{.Description}}">
    <meta property="og:title" content="{{.Title}}">
    <meta property="og:type" content="video.other">
    <meta property="og:description" content="{{.Description}}">
    <meta property="og:url" content="{{.BaseURL}}/watch/{{.ShareToken}}">
    {{if .DownloadEnabled}}<meta property="og:video" content="{{.VideoURL}}">
    <meta property="og:video:type" content="{{.ContentType}}">
    <meta property="og:video:width" content="1920">
    <meta property="og:video:height" content="1080">{{end}}
    {{if .HasThumbnail}}<meta property="og:image" content="{{.BaseURL}}/api/watch/{{.ShareToken}}/thumbnail">{{end}}
    <meta property="og:site_name" content="{{.Branding.CompanyName}}">
    <meta name="twitter:card" content="summary_large_image">
    <meta name="twitter:title" content="{{.Title}}">
    <meta name="twitter:description" content="{{.Description}}">
    {{if .HasThumbnail}}<meta name="twitter:image" content="{{.BaseURL}}/api/watch/{{.ShareToken}}/thumbnail">{{end}}
    <script type="application/ld+json">{{.JSONLD}}</script>
    <style nonce="{{.Nonce}}">
        :root {
            --brand-bg: {{.Branding.ColorBackground}};
            --brand-surface: {{.Branding.ColorSurface}};
            --brand-text: {{.Branding.ColorText}};
            --brand-accent: {{.Branding.ColorAccent}};
        }
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: var(--brand-bg);
            color: var(--brand-text);
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
            display: block;
            background: #000;
        }
        .player-container {
            position: relative;
            background: #000;
            border-radius: 8px;
            overflow: hidden;
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
        .ctrl-btn:focus-visible { outline: 2px solid var(--brand-accent); outline-offset: 2px; }
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
            background: var(--brand-accent);
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
            background: var(--brand-accent);
            border-radius: 1px;
            transform: translateX(-50%);
            opacity: 0.8;
            cursor: pointer;
            pointer-events: auto;
        }
        .seek-marker:hover { opacity: 1; transform: translateX(-50%); }
        .seek-marker.private { background: #3b82f6; }
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
            background: var(--brand-accent);
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
        .speed-menu button.active { color: var(--brand-accent); font-weight: 600; }
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
        .logo {
            display: inline-flex;
            align-items: center;
            gap: 0.4rem;
            text-decoration: none;
            color: #94a3b8;
            font-size: 0.8rem;
            font-weight: 600;
            margin-bottom: 1rem;
            transition: color 0.15s;
        }
        .logo:hover {
            color: var(--brand-accent);
        }
        .logo img {
            width: 20px;
            height: 20px;
        }
        .branding {
            margin-top: 2rem;
            font-size: 0.75rem;
            color: #64748b;
        }
        .branding a {
            color: var(--brand-accent);
            text-decoration: none;
        }
        .branding a:hover {
            text-decoration: underline;
        }
        .actions {
            margin-top: 1rem;
            display: flex;
            align-items: center;
            gap: 1rem;
        }
        .download-btn {
            display: inline-block;
            background: transparent;
            color: var(--brand-accent);
            border: 1px solid var(--brand-accent);
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
        .comments-section {
            margin-top: 2rem;
            border-top: 1px solid var(--brand-surface);
            padding-top: 1.5rem;
        }
        .comments-header {
            font-size: 1.125rem;
            font-weight: 600;
            margin-bottom: 1rem;
        }
        .comment {
            background: var(--brand-surface);
            border-radius: 8px;
            padding: 0.875rem 1rem;
            margin-bottom: 0.75rem;
        }
        .comment-meta {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            margin-bottom: 0.375rem;
            font-size: 0.8125rem;
            color: #94a3b8;
        }
        .comment-author {
            font-weight: 600;
            color: #e2e8f0;
        }
        .comment-owner-badge {
            background: var(--brand-accent);
            color: #fff;
            font-size: 0.6875rem;
            font-weight: 600;
            padding: 0.125rem 0.375rem;
            border-radius: 4px;
        }
        .comment-private-badge {
            background: #3b82f6;
            color: #fff;
            font-size: 0.6875rem;
            font-weight: 600;
            padding: 0.125rem 0.375rem;
            border-radius: 4px;
        }
        .comment-body {
            font-size: 0.9375rem;
            line-height: 1.5;
            color: #cbd5e1;
            white-space: pre-wrap;
            word-break: break-word;
        }
        .comment-form {
            margin-top: 1rem;
        }
        .form-row {
            display: flex;
            gap: 0.75rem;
            margin-bottom: 0.75rem;
        }
        .form-row input {
            flex: 1;
        }
        .comment-form input,
        .comment-form textarea {
            width: 100%;
            padding: 0.625rem 0.75rem;
            border-radius: 6px;
            border: 1px solid #334155;
            background: var(--brand-surface);
            color: #fff;
            font-size: 0.875rem;
            font-family: inherit;
            outline: none;
        }
        .comment-form input:focus,
        .comment-form textarea:focus {
            border-color: var(--brand-accent);
        }
        .comment-form textarea {
            min-height: 80px;
            resize: vertical;
            margin-bottom: 0.75rem;
        }
        .comment-form-actions {
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .comment-form-actions label {
            font-size: 0.8125rem;
            color: #94a3b8;
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 0.375rem;
        }
        .comment-submit {
            background: var(--brand-accent);
            color: #fff;
            border: none;
            padding: 0.5rem 1.25rem;
            border-radius: 6px;
            font-size: 0.875rem;
            font-weight: 600;
            cursor: pointer;
        }
        .comment-submit:hover { opacity: 0.9; }
        .comment-submit:disabled { opacity: 0.5; cursor: not-allowed; }
        .comment-error {
            color: #ef4444;
            font-size: 0.8125rem;
            margin-bottom: 0.5rem;
            display: none;
        }
        .no-comments {
            color: #64748b;
            font-size: 0.875rem;
            margin-bottom: 1rem;
        }
        .reaction-bar {
            display: flex;
            gap: 0.5rem;
            margin-top: 0.75rem;
            padding: 0.5rem 0;
            position: relative;
        }
        .reaction-btn {
            background: var(--brand-surface);
            border: 1px solid #334155;
            border-radius: 20px;
            padding: 0.375rem 0.625rem;
            min-width: 44px;
            min-height: 44px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            font-size: 1.5rem;
            cursor: pointer;
            transition: transform 0.15s, border-color 0.15s, background 0.15s;
            line-height: 1;
        }
        .reaction-btn:hover {
            transform: scale(1.15);
            border-color: var(--brand-accent);
            background: rgba(0, 182, 122, 0.1);
        }
        .reaction-btn:active {
            transform: scale(0.95);
        }
        .reaction-btn:disabled {
            opacity: 0.4;
            cursor: not-allowed;
            transform: none;
        }
        .reaction-btn:focus-visible {
            outline: 2px solid var(--brand-accent);
            outline-offset: 1px;
        }
        .reaction-float {
            position: fixed;
            font-size: 1.5rem;
            pointer-events: none;
            z-index: 100;
            animation: float-up 1s ease-out forwards;
        }
        @keyframes float-up {
            0% { opacity: 1; transform: translateY(0) scale(1); }
            100% { opacity: 0; transform: translateY(-80px) scale(1.4); }
        }
        .comment.emoji-reaction {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.375rem 0.75rem;
            margin-right: 0.5rem;
            margin-bottom: 0.5rem;
            cursor: pointer;
            border: 1px solid #334155;
            background: var(--brand-surface);
            color: inherit;
            font: inherit;
            text-align: left;
        }
        .comment.emoji-reaction:focus-visible {
            outline: 2px solid var(--brand-accent);
            outline-offset: 1px;
        }
        .comment.emoji-reaction .comment-body {
            font-size: 1.25rem;
            line-height: 1;
        }
        .comment.emoji-reaction .comment-meta {
            margin-bottom: 0;
            font-size: 0.75rem;
        }
        .reaction-error {
            margin-top: 0.5rem;
        }
        .comment-highlight {
            animation: glow 1.5s ease-out;
        }
        @keyframes glow {
            0% { box-shadow: 0 0 0 3px rgba(0, 182, 122, 0.5); }
            100% { box-shadow: 0 0 0 0 rgba(0, 182, 122, 0); }
        }
        .comment-timestamp {
            background: var(--brand-accent);
            color: #fff;
            font-size: 0.75rem;
            font-weight: 600;
            padding: 0.125rem 0.5rem;
            border-radius: 10px;
            cursor: pointer;
        }
        .comment-timestamp:hover {
            opacity: 0.85;
        }
        .timestamp-toggle {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            font-size: 0.8125rem;
            font-weight: 500;
            padding: 0.375rem 0.75rem;
            border-radius: 12px;
            margin-bottom: 0.5rem;
            cursor: pointer;
            border: none;
            background: rgba(100, 116, 139, 0.15);
            color: #94a3b8;
        }
        .timestamp-toggle:hover {
            background: rgba(100, 116, 139, 0.25);
            color: #cbd5e1;
        }
        .timestamp-toggle.active {
            background: rgba(0, 182, 122, 0.15);
            color: var(--brand-accent);
            font-weight: 600;
        }
        .timestamp-toggle.active:hover {
            background: rgba(0, 182, 122, 0.25);
        }
        .timestamp-edit-input {
            display: none;
            background: transparent;
            border: none;
            color: inherit;
            font: inherit;
            font-size: 0.8125rem;
            font-weight: 600;
            width: 3.5rem;
            padding: 0;
            outline: none;
        }
        .timestamp-edit-input.editing {
            display: inline-block;
        }
        .timestamp-toggle-remove {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            width: 14px;
            height: 14px;
            border-radius: 50%;
            background: rgba(148, 163, 184, 0.2);
        }
        .timestamp-toggle-remove:hover {
            background: rgba(239, 68, 68, 0.3);
        }
        .timestamp-toggle-remove svg {
            width: 8px;
            height: 8px;
            stroke: #94a3b8;
            stroke-width: 2;
            stroke-linecap: round;
        }
        .timestamp-toggle-remove:hover svg {
            stroke: #ef4444;
        }
        .emoji-picker-wrapper {
            position: relative;
            display: inline-block;
        }
        .emoji-trigger {
            background: transparent;
            border: 1px solid #334155;
            border-radius: 6px;
            padding: 0.375rem 0.5rem;
            font-size: 1.125rem;
            cursor: pointer;
            line-height: 1;
        }
        .emoji-trigger:hover {
            border-color: var(--brand-accent);
        }
        .emoji-grid {
            display: none;
            position: absolute;
            bottom: 100%;
            right: 0;
            margin-bottom: 0.5rem;
            background: #111d32;
            border: 1px solid #334155;
            border-radius: 8px;
            padding: 0.5rem;
            width: 260px;
            max-height: 240px;
            overflow-y: auto;
            z-index: 20;
            box-shadow: 0 4px 16px rgba(0,0,0,0.4);
        }
        .emoji-grid.open {
            display: block;
        }
        .emoji-category {
            font-size: 0.625rem;
            color: #475569;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin: 0.5rem 0 0.25rem;
            padding: 0 0.125rem;
        }
        .emoji-category:first-child {
            margin-top: 0;
        }
        .emoji-btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            width: 2rem;
            height: 2rem;
            font-size: 1.125rem;
            cursor: pointer;
            border-radius: 6px;
            border: none;
            background: transparent;
        }
        .emoji-btn:hover {
            background: var(--brand-surface);
        }
        .transcript-section {
            margin-top: 2rem;
            border-top: 1px solid var(--brand-surface);
            padding-top: 1.5rem;
        }
        .panel-tabs {
            display: flex;
            gap: 0;
            border-bottom: 1px solid var(--brand-surface);
            margin-bottom: 16px;
        }
        .panel-tab {
            background: none;
            border: none;
            color: #94a3b8;
            font-size: 14px;
            font-weight: 600;
            padding: 8px 16px;
            cursor: pointer;
            border-bottom: 2px solid transparent;
            transition: color 0.2s, border-color 0.2s;
        }
        .panel-tab:hover {
            color: var(--brand-text);
        }
        .panel-tab--active {
            color: var(--brand-accent);
            border-bottom-color: var(--brand-accent);
        }
        .summary-text {
            color: var(--brand-text);
            font-size: 14px;
            line-height: 1.6;
            margin: 0 0 16px;
        }
        .chapter-list-title {
            color: var(--brand-text);
            font-size: 13px;
            font-weight: 600;
            margin: 0 0 8px;
        }
        .chapter-item {
            display: flex;
            align-items: center;
            gap: 12px;
            padding: 6px 0;
            cursor: pointer;
            border-radius: 4px;
        }
        .chapter-item:hover {
            background: rgba(255,255,255,0.05);
        }
        .chapter-timestamp {
            color: var(--brand-accent);
            font-size: 13px;
            font-family: monospace;
            min-width: 48px;
        }
        .chapter-title {
            color: var(--brand-text);
            font-size: 14px;
        }
        .cta-card { display: none; margin: 1rem 0; padding: 1.25rem; background: var(--brand-surface); border: 1px solid var(--brand-accent); border-radius: 8px; text-align: center; position: relative; }
        .cta-card.visible { display: block; }
        .cta-dismiss { position: absolute; top: 8px; right: 12px; background: none; border: none; color: #94a3b8; cursor: pointer; font-size: 1.25rem; line-height: 1; padding: 4px; }
        .cta-dismiss:hover { color: #e2e8f0; }
        .cta-btn { display: inline-block; padding: 0.75rem 2rem; background: var(--brand-accent); color: #fff; border: none; border-radius: 6px; font-size: 1rem; font-weight: 600; cursor: pointer; text-decoration: none; }
        .cta-btn:hover { opacity: 0.9; }
        .hidden { display: none; }
        .flex-center { display: flex; align-items: center; gap: 0.5rem; }
        .transcribe-btn { font-size: 0.7rem; padding: 0.2rem 0.6rem; }
        .transcript-header {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            font-size: 1.125rem;
            font-weight: 600;
            margin-bottom: 1rem;
            color: #f8fafc;
        }
        .transcript-segment {
            display: flex;
            gap: 0.75rem;
            padding: 0.5rem 0.625rem;
            border-radius: 6px;
            cursor: pointer;
            transition: background 0.15s;
        }
        .transcript-segment:hover {
            background: rgba(255, 255, 255, 0.05);
        }
        .transcript-segment.active {
            background: rgba(0, 182, 122, 0.1);
        }
        .transcript-timestamp {
            color: var(--brand-accent);
            font-size: 0.8125rem;
            font-weight: 600;
            white-space: nowrap;
            min-width: 3rem;
            padding-top: 0.125rem;
        }
        .transcript-text {
            font-size: 0.9375rem;
            line-height: 1.5;
            color: #cbd5e1;
        }
        .transcript-processing {
            color: #94a3b8;
            font-size: 0.875rem;
            font-style: italic;
        }
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
        @media (max-width: 640px) {
            .container { padding: 1rem 0.75rem; }
            h1 { font-size: 1.25rem; }
            .actions { flex-wrap: wrap; }
            .form-row { flex-direction: column; }
            .download-btn { min-height: 44px; }
            .volume-slider { display: none; }
            .comment-submit { min-height: 44px; }
            .emoji-trigger { min-height: 44px; min-width: 44px; }
            .emoji-grid { width: min(260px, calc(100vw - 2rem)); right: auto; left: 0; }
        }
        {{if .CustomCSS}}{{.CustomCSS}}{{end}}
    </style>
</head>
<body>
    <div class="container">
        <a href="{{.BaseURL}}" class="logo">{{if .Branding.LogoURL}}<img src="{{.Branding.LogoURL}}" alt="{{.Branding.CompanyName}}" width="20" height="20">{{end}}{{.Branding.CompanyName}}</a>
        <div class="player-container" id="player-container">
            <video id="player" playsinline webkit-playsinline crossorigin="anonymous"{{if not .DownloadEnabled}} controlsList="nodownload" oncontextmenu="return false;"{{end}}{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}>
                <source src="{{.VideoURL}}" type="{{.ContentType}}">
                {{if .TranscriptURL}}<track kind="subtitles" src="{{.TranscriptURL}}" srclang="en" label="Subtitles" default>{{end}}
                Your browser does not support video playback.
            </video>
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
                <button class="ctrl-btn" id="fullscreen-btn" aria-label="Fullscreen">&#9974;</button>
            </div>
        </div>
        <div id="safari-webm-warning" class="hidden" role="alert">
            <p>This video was recorded in WebM format, which is not supported by Safari. Please open this link in Chrome or Firefox to watch.</p>
        </div>
        <script nonce="{{.Nonce}}">
            var v = document.getElementById('player');
            (function() {
                var isSafari = /^((?!chrome|android).)*safari/i.test(navigator.userAgent);
                var src = document.querySelector('#player source');
                if (isSafari && src && src.getAttribute('type') === 'video/webm') {
                    document.getElementById('safari-webm-warning').className = 'browser-warning';
                    document.getElementById('player').style.display = 'none';
                }
            })();
        </script>
        {{if ne .CommentMode "disabled"}}
        <div class="reaction-bar" id="reaction-bar">
            {{range .ReactionEmojis}}
            <button type="button" class="reaction-btn" data-emoji="{{.}}" title="React with {{.}}" aria-label="React with {{.}}">{{.}}</button>
            {{end}}
        </div>
        <p class="comment-error reaction-error" id="reaction-error" role="alert"></p>
        {{end}}
        <h1>{{.Title}}</h1>
        <p class="meta">{{.Creator}} · {{.Date}}</p>
        {{if .DownloadEnabled}}<div class="actions"><button class="download-btn" id="download-btn">Download</button></div>{{end}}
        {{if and .CtaText .CtaUrl}}
        <div class="cta-card" id="cta-card">
            <button class="cta-dismiss" onclick="document.getElementById('cta-card').classList.remove('visible')" aria-label="Dismiss">&times;</button>
            <a href="{{.CtaUrl}}" target="_blank" rel="noopener noreferrer" class="cta-btn" id="cta-btn">{{.CtaText}}</a>
        </div>
        {{end}}
        <script nonce="{{.Nonce}}">
            {{if .DownloadEnabled}}
            document.getElementById('download-btn').addEventListener('click', function() {
                fetch('/api/watch/{{.ShareToken}}/download')
                    .then(function(r) { return r.json(); })
                    .then(function(data) { if (data.downloadUrl) window.location.href = data.downloadUrl; });
            });
            {{end}}
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
                var speedBtn = document.getElementById('speed-btn');
                var speedMenu = document.getElementById('speed-menu');
                var pipBtn = document.getElementById('pip-btn');
                var fullscreenBtn = document.getElementById('fullscreen-btn');
                var spinner = document.getElementById('player-spinner');
                var errorOverlay = document.getElementById('player-error');
                var seekTooltip = document.getElementById('seek-time-tooltip');
                var hideTimer = null;

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

                if (document.pictureInPictureEnabled) {
                    pipBtn.addEventListener('click', function() {
                        if (document.pictureInPictureElement) document.exitPictureInPicture().catch(function(){});
                        else player.requestPictureInPicture().catch(function(){});
                    });
                } else {
                    pipBtn.style.display = 'none';
                }

                fullscreenBtn.addEventListener('click', function() {
                    if (document.fullscreenElement) document.exitFullscreen().catch(function(){});
                    else container.requestFullscreen().catch(function(){});
                });
                document.addEventListener('fullscreenchange', function() {
                    fullscreenBtn.innerHTML = document.fullscreenElement ? '&#9723;' : '&#9974;';
                    fullscreenBtn.setAttribute('aria-label', document.fullscreenElement ? 'Exit fullscreen' : 'Fullscreen');
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
                    if (!player.paused) {
                        hideTimer = setTimeout(function() { controls.classList.add('hidden'); }, 1000);
                    }
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
                    // Show chapter name if applicable
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
        </script>
        {{if ne .CommentMode "disabled"}}
        <div class="comments-section" id="comments-section">
            <h2 class="comments-header" id="comments-header">Comments</h2>
            <div id="comments-list"></div>
            <div class="comment-form" id="comment-form">
                <p class="comment-error" id="comment-error"></p>
                {{if or (eq .CommentMode "name_required") (eq .CommentMode "name_email_required")}}
                <div class="form-row">
                    <input type="text" id="comment-name" placeholder="Your name" maxlength="200">
                    {{if eq .CommentMode "name_email_required"}}
                    <input type="email" id="comment-email" placeholder="Your email" maxlength="320">
                    {{end}}
                </div>
                {{end}}
                <span class="timestamp-toggle" id="timestamp-toggle">
                    <span id="timestamp-toggle-label">&#x1F551;</span>
                    <span id="timestamp-toggle-text">Add timestamp</span>
                    <input type="text" class="timestamp-edit-input" id="timestamp-edit-input" placeholder="0:00">
                    <span class="timestamp-toggle-remove hidden" id="timestamp-toggle-remove"><svg viewBox="0 0 10 10"><line x1="2" y1="2" x2="8" y2="8"/><line x1="8" y1="2" x2="2" y2="8"/></svg></span>
                </span>
                <textarea id="comment-body" placeholder="Write a comment..." maxlength="5000"></textarea>
                <div class="comment-form-actions">
                    <div class="flex-center">
                        <span id="private-toggle"></span>
                        <div class="emoji-picker-wrapper" id="emoji-wrapper">
                            <button type="button" class="emoji-trigger" id="emoji-trigger">&#x1F642;</button>
                            <div class="emoji-grid" id="emoji-grid"></div>
                        </div>
                    </div>
                    <button class="comment-submit" id="comment-submit">Post comment</button>
                </div>
            </div>
        </div>
        <script nonce="{{.Nonce}}">
        (function() {
            var shareToken = '{{.ShareToken}}';
            var commentMode = '{{.CommentMode}}';
            var listEl = document.getElementById('comments-list');
            var headerEl = document.getElementById('comments-header');
            var errorEl = document.getElementById('comment-error');
            var submitBtn = document.getElementById('comment-submit');
            var bodyEl = document.getElementById('comment-body');
            var nameEl = document.getElementById('comment-name');
            var emailEl = document.getElementById('comment-email');
            var privateToggleEl = document.getElementById('private-toggle');
            var markersBar = document.getElementById('seek-markers');
            var player = document.getElementById('player');
            var videoDuration = 0;
            var lastComments = null;
            var timestampToggle = document.getElementById('timestamp-toggle');
            var timestampToggleText = document.getElementById('timestamp-toggle-text');
            var timestampToggleRemove = document.getElementById('timestamp-toggle-remove');
            var timestampEditInput = document.getElementById('timestamp-edit-input');
            var capturedTimestamp = null;
            var emojiTrigger = document.getElementById('emoji-trigger');
            var emojiGrid = document.getElementById('emoji-grid');
            var reactionErrorEl = document.getElementById('reaction-error');

            function getAuthToken() {
                try { return localStorage.getItem('token') || ''; } catch(e) { return ''; }
            }

            var token = getAuthToken();
            if (token && privateToggleEl) {
                privateToggleEl.innerHTML = '<label><input type="checkbox" id="comment-private"> Private comment</label>';
            }

            function timeAgo(dateStr) {
                var seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
                if (seconds < 60) return 'just now';
                var minutes = Math.floor(seconds / 60);
                if (minutes < 60) return minutes + (minutes === 1 ? ' min ago' : ' mins ago');
                var hours = Math.floor(minutes / 60);
                if (hours < 24) return hours + (hours === 1 ? ' hour ago' : ' hours ago');
                var days = Math.floor(hours / 24);
                return days + (days === 1 ? ' day ago' : ' days ago');
            }

            function escapeHtml(text) {
                var div = document.createElement('div');
                div.textContent = text;
                return div.innerHTML;
            }

            function formatTimestamp(seconds) {
                var m = Math.floor(seconds / 60);
                var s = Math.floor(seconds % 60);
                return m + ':' + (s < 10 ? '0' : '') + s;
            }

            function getFiniteDuration() {
                if (!player.duration || player.duration === Infinity || isNaN(player.duration)) return 0;
                return player.duration;
            }

            function getTimelineDuration(comments) {
                var finiteDuration = getFiniteDuration();
                if (finiteDuration > 0) return finiteDuration;
                var maxTimestamp = 0;
                var hasTimestamp = false;
                comments.forEach(function(c) {
                    if (c.videoTimestamp == null) return;
                    hasTimestamp = true;
                    if (c.videoTimestamp > maxTimestamp) maxTimestamp = c.videoTimestamp;
                });
                if (!hasTimestamp) return 0;
                return maxTimestamp > 0 ? maxTimestamp : 1;
            }

            function clampTimestampToVideo(seconds) {
                var safeSeconds = Math.max(0, seconds || 0);
                var finiteDuration = getFiniteDuration();
                if (finiteDuration > 0) return Math.min(safeSeconds, finiteDuration);
                return safeSeconds;
            }

            function clampReactionTimestamp(seconds) {
                var safeSeconds = Math.max(0, Math.floor(seconds || 0));
                var finiteDuration = getFiniteDuration();
                if (finiteDuration > 0) return Math.min(safeSeconds, Math.max(0, Math.floor(finiteDuration) - 1));
                return safeSeconds;
            }

            function renderMarkers(comments) {
                if (!markersBar) return;
                markersBar.innerHTML = '';
                var bySecond = {};
                comments.forEach(function(c) {
                    if (c.videoTimestamp == null) return;
                    var sec = Math.floor(c.videoTimestamp);
                    if (!bySecond[sec]) bySecond[sec] = [];
                    bySecond[sec].push(c);
                });
                var keys = Object.keys(bySecond);
                if (keys.length === 0) return;
                var timelineDuration = getTimelineDuration(comments);
                if (!timelineDuration) return;
                keys.forEach(function(sec) {
                    var group = bySecond[sec];
                    var dot = document.createElement('div');
                    dot.className = 'seek-marker' + (group[0].isPrivate ? ' private' : '');
                    var pct = Math.min(group[0].videoTimestamp / timelineDuration * 100, 99);
                    dot.style.left = pct + '%';
                    var tooltipText;
                    if (group.length === 1) {
                        var author = group[0].authorName || 'Anonymous';
                        tooltipText = author + ' \u00b7 ' + formatTimestamp(group[0].videoTimestamp) + ' \u2014 ' + group[0].body.substring(0, 80);
                    } else {
                        tooltipText = formatTimestamp(group[0].videoTimestamp) + ' \u2014 ' + group.map(function(c) {
                            return (isReactionEmoji(c.body) ? c.body : (c.authorName || 'Anonymous'));
                        }).join(' ');
                    }
                    var tooltip = document.createElement('div');
                    tooltip.className = 'seek-marker-tooltip';
                    tooltip.textContent = tooltipText;
                    dot.appendChild(tooltip);
                    dot.addEventListener('click', function(e) {
                        e.stopPropagation();
                        player.currentTime = group[0].videoTimestamp;
                        var commentEl = document.getElementById('comment-' + group[0].id);
                        if (commentEl) {
                            commentEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
                            commentEl.classList.add('comment-highlight');
                            setTimeout(function() { commentEl.classList.remove('comment-highlight'); }, 1500);
                        }
                    });
                    markersBar.appendChild(dot);
                });
            }

            function updateDuration() {
                var finiteDuration = getFiniteDuration();
                if (finiteDuration > 0 && finiteDuration !== videoDuration) {
                    videoDuration = finiteDuration;
                    if (lastComments) renderMarkers(lastComments);
                }
            }
            player.addEventListener('loadedmetadata', updateDuration);
            player.addEventListener('durationchange', updateDuration);

            var reactionEmojis = {{.ReactionEmojisJSON}};
            function isReactionEmoji(text) {
                return reactionEmojis.indexOf(text.trim()) !== -1;
            }

            function renderComment(c) {
                var authorName = c.authorName || 'Anonymous';
                if (isReactionEmoji(c.body)) {
                    var tsBadge = '';
                    if (c.videoTimestamp != null) {
                        tsBadge = ' <span class="comment-timestamp" data-ts="' + c.videoTimestamp + '">' + formatTimestamp(c.videoTimestamp) + '</span>';
                    }
                    return '<button type="button" class="comment emoji-reaction" id="comment-' + c.id + '">' +
                        '<span class="comment-body">' + escapeHtml(c.body) + '</span>' +
                        '<span class="comment-meta">' +
                            escapeHtml(authorName) + tsBadge +
                            ' \u00b7 ' + timeAgo(c.createdAt) +
                        '</span>' +
                    '</button>';
                }
                var badges = '';
                if (c.videoTimestamp != null) {
                    badges += ' <span class="comment-timestamp" data-ts="' + c.videoTimestamp + '">' + formatTimestamp(c.videoTimestamp) + '</span>';
                }
                if (c.isOwner) badges += ' <span class="comment-owner-badge">Owner</span>';
                if (c.isPrivate) badges += ' <span class="comment-private-badge">Private</span>';
                return '<div class="comment" id="comment-' + c.id + '">' +
                    '<div class="comment-meta">' +
                        '<span class="comment-author">' + escapeHtml(authorName) + '</span>' +
                        badges +
                        '<span>\u00b7 ' + timeAgo(c.createdAt) + '</span>' +
                    '</div>' +
                    '<div class="comment-body">' + escapeHtml(c.body) + '</div>' +
                '</div>';
            }

            listEl.addEventListener('click', function(e) {
                var tsEl = e.target.closest('.comment-timestamp');
                if (tsEl) {
                    player.currentTime = parseFloat(tsEl.getAttribute('data-ts'));
                    player.play().catch(function() {});
                    return;
                }
                var reactionEl = e.target.closest('.emoji-reaction');
                if (reactionEl) {
                    var ts = reactionEl.querySelector('.comment-timestamp');
                    if (ts) {
                        player.currentTime = parseFloat(ts.getAttribute('data-ts'));
                        player.play().catch(function() {});
                    }
                }
            });

            function loadComments() {
                var headers = {};
                if (token) headers['Authorization'] = 'Bearer ' + token;
                fetch('/api/watch/' + shareToken + '/comments', { headers: headers })
                    .then(function(r) { return r.json(); })
                    .then(function(data) {
                        if (!data.comments || data.comments.length === 0) {
                            listEl.innerHTML = '<p class="no-comments">No comments yet. Be the first!</p>';
                            headerEl.textContent = 'Comments';
                            lastComments = [];
                        } else {
                            headerEl.textContent = 'Comments (' + data.comments.length + ')';
                            listEl.innerHTML = data.comments.map(renderComment).join('');
                            lastComments = data.comments;
                            renderMarkers(data.comments);
                        }
                    });
            }

            loadComments();

            var reactionBar = document.getElementById('reaction-bar');
            if (reactionBar) {
                reactionBar.addEventListener('click', function(e) {
                    var btn = e.target.closest('.reaction-btn');
                    if (!btn || btn.disabled) return;
                    var emoji = btn.getAttribute('data-emoji');
                    var timestamp = clampReactionTimestamp(player.currentTime);
                    if (reactionErrorEl) reactionErrorEl.style.display = 'none';
                    btn.disabled = true;
                    var rect = btn.getBoundingClientRect();
                    var floater = document.createElement('span');
                    floater.className = 'reaction-float';
                    floater.textContent = emoji;
                    floater.style.left = rect.left + rect.width / 2 - 12 + 'px';
                    floater.style.top = rect.top + 'px';
                    document.body.appendChild(floater);
                    floater.addEventListener('animationend', function() { floater.remove(); });
                    var headers = {'Content-Type': 'application/json'};
                    if (token) headers['Authorization'] = 'Bearer ' + token;
                    fetch('/api/watch/' + shareToken + '/comments', {
                        method: 'POST',
                        headers: headers,
                        body: JSON.stringify({authorName: '', authorEmail: '', body: emoji, isPrivate: false, videoTimestamp: timestamp})
                    }).then(function(r) {
                        if (!r.ok) {
                            return r.json()
                                .then(function(d) { throw new Error(d && d.error ? d.error : 'Could not post reaction'); })
                                .catch(function() { throw new Error('Could not post reaction'); });
                        }
                        return r.json();
                    }).then(function(comment) {
                        var noComments = listEl.querySelector('.no-comments');
                        if (noComments) noComments.remove();
                        listEl.insertAdjacentHTML('beforeend', renderComment(comment));
                        var count = listEl.querySelectorAll('.comment').length;
                        headerEl.textContent = 'Comments (' + count + ')';
                        if (lastComments) {
                            lastComments.push(comment);
                            renderMarkers(lastComments);
                        }
                        btn.disabled = false;
                    }).catch(function(err) {
                        if (reactionErrorEl) {
                            reactionErrorEl.textContent = err && err.message ? err.message : 'Could not post reaction';
                            reactionErrorEl.style.display = 'block';
                        }
                        btn.disabled = false;
                    });
                });
            }

            function parseTimestamp(str) {
                var parts = str.trim().split(':');
                if (parts.length === 2) {
                    var m = parseInt(parts[0], 10);
                    var s = parseInt(parts[1], 10);
                    if (!isNaN(m) && !isNaN(s) && s >= 0 && s < 60 && m >= 0) {
                        return m * 60 + s;
                    }
                }
                if (parts.length === 1) {
                    var sec = parseInt(parts[0], 10);
                    if (!isNaN(sec) && sec >= 0) return sec;
                }
                return null;
            }

            function setTimestamp(seconds) {
                capturedTimestamp = clampTimestampToVideo(seconds);
                timestampToggle.classList.add('active');
                timestampToggleText.textContent = formatTimestamp(capturedTimestamp);
                timestampToggleText.style.display = '';
                timestampEditInput.classList.remove('editing');
                timestampToggleRemove.style.display = 'inline-flex';
            }

            function deactivateTimestamp() {
                capturedTimestamp = null;
                timestampToggle.classList.remove('active');
                timestampToggleText.textContent = 'Add timestamp';
                timestampToggleText.style.display = '';
                timestampEditInput.classList.remove('editing');
                timestampToggleRemove.style.display = 'none';
            }

            function startEditing() {
                var current = capturedTimestamp !== null ? formatTimestamp(capturedTimestamp) : '';
                timestampToggleText.style.display = 'none';
                timestampEditInput.classList.add('editing');
                timestampEditInput.value = current;
                timestampEditInput.focus();
                timestampEditInput.select();
            }

            function commitEdit() {
                var parsed = parseTimestamp(timestampEditInput.value);
                if (parsed !== null) {
                    setTimestamp(parsed);
                } else if (capturedTimestamp !== null) {
                    timestampToggleText.style.display = '';
                    timestampEditInput.classList.remove('editing');
                } else {
                    deactivateTimestamp();
                }
            }

            timestampToggle.addEventListener('click', function(e) {
                if (e.target.closest('.timestamp-toggle-remove')) {
                    e.stopPropagation();
                    deactivateTimestamp();
                    return;
                }
                if (e.target === timestampEditInput) return;
                if (capturedTimestamp !== null) {
                    startEditing();
                } else {
                    player.pause();
                    setTimestamp(player.currentTime);
                }
            });

            timestampEditInput.addEventListener('keydown', function(e) {
                if (e.key === 'Enter') { e.preventDefault(); commitEdit(); }
                if (e.key === 'Escape') {
                    e.preventDefault();
                    timestampToggleText.style.display = '';
                    timestampEditInput.style.display = 'none';
                }
            });

            timestampEditInput.addEventListener('blur', function() {
                commitEdit();
            });

            var emojiCategories = {
                'Smileys': ['\uD83D\uDE00','\uD83D\uDE03','\uD83D\uDE04','\uD83D\uDE01','\uD83D\uDE06','\uD83D\uDE05','\uD83E\uDD23','\uD83D\uDE02','\uD83D\uDE42','\uD83D\uDE09','\uD83D\uDE0A','\uD83D\uDE07','\uD83D\uDE0D','\uD83E\uDD29','\uD83D\uDE18','\uD83D\uDE0B','\uD83D\uDE1C','\uD83E\uDD17','\uD83E\uDD14','\uD83D\uDE10','\uD83D\uDE11','\uD83D\uDE36'],
                'Hands': ['\uD83D\uDC4D','\uD83D\uDC4E','\uD83D\uDC4F','\uD83D\uDE4C','\uD83E\uDD1D','\u270C\uFE0F','\uD83E\uDD1E','\uD83E\uDD1F','\uD83D\uDC4B','\uD83D\uDD90\uFE0F','\u270B','\uD83D\uDC4A'],
                'Symbols': ['\u2764\uFE0F','\uD83D\uDD25','\u2B50','\u2705','\u274C','\u26A1','\uD83D\uDCA1','\uD83C\uDFAF','\uD83C\uDFC6','\uD83D\uDCAC','\uD83D\uDCCC','\uD83D\uDD17'],
                'Reactions': ['\uD83D\uDCAF','\uD83D\uDC40','\uD83C\uDF89','\uD83D\uDE31','\uD83D\uDE2C','\uD83E\uDD2F','\uD83E\uDD26','\uD83E\uDD37','\uD83D\uDCAA','\uD83D\uDE4F','\uD83D\uDE22','\uD83D\uDE21'],
                'Work': ['\uD83D\uDCBB','\uD83D\uDCCA','\uD83D\uDCDD','\uD83D\uDCC1','\uD83D\uDD27','\uD83D\uDD0D','\uD83D\uDCF1','\uD83D\uDDA5\uFE0F','\u23F0','\uD83D\uDCC5','\uD83D\uDCC8','\uD83D\uDDC2\uFE0F'],
                'Misc': ['\uD83D\uDE80','\uD83C\uDF1F','\uD83D\uDC8E','\uD83C\uDFB5','\u2615','\uD83C\uDF55','\uD83C\uDF08','\uD83C\uDFA8','\uD83D\uDCF8','\uD83C\uDFE0','\uD83C\uDF0D','\uD83D\uDCA4']
            };

            var gridHTML = '';
            Object.keys(emojiCategories).forEach(function(cat) {
                gridHTML += '<div class="emoji-category">' + cat + '</div><div>';
                emojiCategories[cat].forEach(function(em) {
                    gridHTML += '<button type="button" class="emoji-btn" data-emoji="' + em + '">' + em + '</button>';
                });
                gridHTML += '</div>';
            });
            emojiGrid.innerHTML = gridHTML;

            emojiTrigger.addEventListener('click', function(e) {
                e.stopPropagation();
                emojiGrid.classList.toggle('open');
            });

            emojiGrid.addEventListener('click', function(e) {
                var btn = e.target.closest('.emoji-btn');
                if (!btn) return;
                var emoji = btn.getAttribute('data-emoji');
                var start = bodyEl.selectionStart || 0;
                var end = bodyEl.selectionEnd || 0;
                bodyEl.value = bodyEl.value.substring(0, start) + emoji + bodyEl.value.substring(end);
                bodyEl.selectionStart = bodyEl.selectionEnd = start + emoji.length;
                bodyEl.focus();
                emojiGrid.classList.remove('open');
            });

            document.addEventListener('click', function(e) {
                if (!e.target.closest('#emoji-wrapper')) {
                    emojiGrid.classList.remove('open');
                }
            });

            document.addEventListener('keydown', function(e) {
                if (e.key === 'Escape') {
                    emojiGrid.classList.remove('open');
                }
            });

            submitBtn.addEventListener('click', function() {
                var body = bodyEl.value.trim();
                if (!body) { errorEl.textContent = 'Please write a comment.'; errorEl.style.display = 'block'; return; }
                var authorName = nameEl ? nameEl.value.trim() : '';
                var authorEmail = emailEl ? emailEl.value.trim() : '';
                if ((commentMode === 'name_required' || commentMode === 'name_email_required') && !authorName) {
                    errorEl.textContent = 'Name is required.'; errorEl.style.display = 'block'; return;
                }
                if (commentMode === 'name_email_required' && !authorEmail) {
                    errorEl.textContent = 'Email is required.'; errorEl.style.display = 'block'; return;
                }
                var privateEl = document.getElementById('comment-private');
                var isPrivate = privateEl ? privateEl.checked : false;
                submitBtn.disabled = true;
                errorEl.style.display = 'none';
                var headers = {'Content-Type': 'application/json'};
                if (token) headers['Authorization'] = 'Bearer ' + token;
                fetch('/api/watch/' + shareToken + '/comments', {
                    method: 'POST',
                    headers: headers,
                    body: JSON.stringify({authorName: authorName, authorEmail: authorEmail, body: body, isPrivate: isPrivate, videoTimestamp: capturedTimestamp})
                }).then(function(r) {
                    if (!r.ok) return r.json().then(function(d) { throw new Error(d.error || 'Could not post comment'); });
                    return r.json();
                }).then(function(comment) {
                    listEl.querySelector('.no-comments') && listEl.querySelector('.no-comments').remove();
                    listEl.insertAdjacentHTML('beforeend', renderComment(comment));
                    var count = listEl.querySelectorAll('.comment').length;
                    headerEl.textContent = 'Comments (' + count + ')';
                    bodyEl.value = '';
                    if (privateEl) privateEl.checked = false;
                    deactivateTimestamp();
                    if (lastComments) {
                        lastComments.push(comment);
                        renderMarkers(lastComments);
                    }
                    submitBtn.disabled = false;
                }).catch(function(err) {
                    errorEl.textContent = err.message; errorEl.style.display = 'block'; submitBtn.disabled = false;
                });
            });
        })();
        </script>
        {{end}}
        {{if ne .TranscriptStatus "no_audio"}}
        <div class="transcript-section">
            {{if and (eq .TranscriptStatus "ready") (eq .SummaryStatus "ready")}}
            <div class="panel-tabs">
                <button class="panel-tab panel-tab--active" data-tab="summary">Summary</button>
                <button class="panel-tab" data-tab="transcript">Transcript</button>
            </div>
            <div class="panel-content" id="summary-panel">
                <p class="summary-text">{{.Summary}}</p>
                {{if .Chapters}}
                <div class="chapter-list">
                    <h3 class="chapter-list-title">Chapters</h3>
                    {{range .Chapters}}
                    <div class="chapter-item" data-start="{{.Start}}">
                        <span class="chapter-timestamp">{{formatTimestamp .Start}}</span>
                        <span class="chapter-title">{{.Title}}</span>
                    </div>
                    {{end}}
                </div>
                {{end}}
            </div>
            <div class="panel-content hidden" id="transcript-panel">
                {{range .Segments}}
                <div class="transcript-segment" data-start="{{.Start}}" data-end="{{.End}}">
                    <span class="transcript-timestamp">{{formatTimestamp .Start}}</span>
                    <span class="transcript-text">{{.Text}}</span>
                </div>
                {{end}}
            </div>
            {{else}}
            <h2 class="transcript-header">Transcript <button class="download-btn transcribe-btn hidden" id="transcribe-btn">Transcribe</button></h2>
            {{if eq .TranscriptStatus "pending"}}
            <p class="transcript-processing">Transcription queued...</p>
            {{else if eq .TranscriptStatus "processing"}}
            <p class="transcript-processing">Transcription in progress...</p>
            {{else if eq .TranscriptStatus "ready"}}
            <div id="transcript-panel">
                {{range .Segments}}
                <div class="transcript-segment" data-start="{{.Start}}" data-end="{{.End}}">
                    <span class="transcript-timestamp">{{formatTimestamp .Start}}</span>
                    <span class="transcript-text">{{.Text}}</span>
                </div>
                {{end}}
            </div>
            {{else if eq .TranscriptStatus "failed"}}
            <p class="transcript-processing hidden" id="transcript-failed">Transcription failed.</p>
            {{end}}
            {{end}}
        </div>
        {{end}}
        <script nonce="{{.Nonce}}">
        (function() {
            var panel = document.getElementById('transcript-panel');
            if (panel) {
                var player = document.getElementById('player');
                var segments = panel.querySelectorAll('.transcript-segment');

                panel.addEventListener('click', function(e) {
                    var seg = e.target.closest('.transcript-segment');
                    if (!seg) return;
                    var start = parseFloat(seg.getAttribute('data-start'));
                    player.currentTime = start;
                    player.play().catch(function() {});
                });

                player.addEventListener('timeupdate', function() {
                    var currentTime = player.currentTime;
                    segments.forEach(function(seg) {
                        var start = parseFloat(seg.getAttribute('data-start'));
                        var end = parseFloat(seg.getAttribute('data-end'));
                        if (currentTime >= start && currentTime < end) {
                            seg.classList.add('active');
                        } else {
                            seg.classList.remove('active');
                        }
                    });
                });
            }

            var token = localStorage.getItem('token');
            var failedMsg = document.getElementById('transcript-failed');
            if (token && failedMsg) failedMsg.classList.remove('hidden');
            var btn = document.getElementById('transcribe-btn');
            if (token && btn) {
                var status = '{{.TranscriptStatus}}';
                if (status === 'none') btn.textContent = 'Transcribe';
                else if (status === 'failed') btn.textContent = 'Retry';
                else if (status === 'ready') btn.textContent = 'Redo';
                else btn.textContent = '';
                if (status !== 'processing' && status !== 'pending' && btn.textContent) {
                    btn.classList.remove('hidden');
                }
                btn.addEventListener('click', function() {
                    btn.disabled = true;
                    btn.textContent = 'Starting...';
                    fetch('/api/videos/{{.VideoID}}/retranscribe', {
                        method: 'POST',
                        headers: { 'Authorization': 'Bearer ' + token }
                    }).then(function(r) {
                        if (r.ok) window.location.reload();
                        else { btn.textContent = 'Error'; btn.disabled = false; }
                    }).catch(function() { btn.textContent = 'Error'; btn.disabled = false; });
                });
            }
        })();
        (function() {
            var tabs = document.querySelectorAll('.panel-tab');
            if (!tabs.length) return;
            tabs.forEach(function(tab) {
                tab.addEventListener('click', function() {
                    tabs.forEach(function(t) { t.classList.remove('panel-tab--active'); });
                    tab.classList.add('panel-tab--active');
                    var target = tab.getAttribute('data-tab');
                    document.getElementById('summary-panel').classList.toggle('hidden', target !== 'summary');
                    document.getElementById('transcript-panel').classList.toggle('hidden', target !== 'transcript');
                });
            });
            var chapters = document.querySelectorAll('.chapter-item');
            var player = document.getElementById('player');
            chapters.forEach(function(ch) {
                ch.addEventListener('click', function() {
                    var start = parseFloat(ch.getAttribute('data-start'));
                    player.currentTime = start;
                    player.play().catch(function() {});
                });
            });
        })();
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
                    if (currentTime >= start && currentTime < end) {
                        seg.classList.add('active');
                    } else {
                        seg.classList.remove('active');
                    }
                });
            });

            player.addEventListener('loadedmetadata', renderChapters);
            player.addEventListener('durationchange', renderChapters);
            if (player.duration && isFinite(player.duration)) {
                renderChapters();
            }
        })();
        {{end}}
        {{if and .CtaText .CtaUrl}}
        (function() {
            var player = document.getElementById('player');
            var ctaCard = document.getElementById('cta-card');
            var ctaBtn = document.getElementById('cta-btn');
            if (player && ctaCard) {
                player.addEventListener('ended', function() {
                    ctaCard.classList.add('visible');
                });
            }
            if (ctaBtn) {
                ctaBtn.addEventListener('click', function() {
                    fetch('/api/watch/{{.ShareToken}}/cta-click', { method: 'POST' }).catch(function() {});
                });
            }
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
        {{if or (eq .TranscriptStatus "processing") (eq .TranscriptStatus "pending") (eq .SummaryStatus "pending") (eq .SummaryStatus "processing")}}
        (function() {
            var pollInterval = setInterval(function() {
                fetch('/api/watch/{{.ShareToken}}?poll=transcript')
                    .then(function(r) { return r.json(); })
                    .then(function(data) {
                        var tDone = data.transcriptStatus === 'ready' || data.transcriptStatus === 'failed' || data.transcriptStatus === 'none';
                        var sDone = data.summaryStatus === 'ready' || data.summaryStatus === 'failed' || data.summaryStatus === 'none';
                        if (tDone && sDone) {
                            clearInterval(pollInterval);
                            window.location.reload();
                        }
                    })
                    .catch(function() {});
            }, 10000);
        })();
        {{end}}
        </script>
        {{if eq .SubscriptionPlan "pro"}}{{if .Branding.FooterText}}<p class="branding">{{.Branding.FooterText}}</p>{{end}}{{else}}<p class="branding">{{if .Branding.FooterText}}{{.Branding.FooterText}} · {{end}}Shared via <a href="https://sendrec.eu">SendRec</a>{{if not .Branding.FooterText}} — open-source video messaging{{end}}</p>{{end}}
    </div>
{{.AnalyticsScript}}
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

var notFoundPageTemplate = template.Must(template.New("notfound").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Video Not Found — SendRec</title>
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
        <h1>Video not found</h1>
        <p>This video doesn't exist or has been deleted.</p>
        <a href="https://sendrec.eu">Go to SendRec</a>
    </div>
</body>
</html>`))

type notFoundPageData struct {
	Nonce string
}

type watchPageData struct {
	Title              string
	VideoURL           string
	Creator            string
	Date               string
	Nonce              string
	ThumbnailURL       string
	ShareToken         string
	VideoID            string
	CommentMode        string
	TranscriptURL      string
	TranscriptStatus   string
	Segments           []TranscriptSegment
	BaseURL            string
	ContentType        string
	Branding           brandingConfig
	AnalyticsScript    template.HTML
	DownloadEnabled    bool
	CustomCSS          template.CSS
	ReactionEmojis     []string
	ReactionEmojisJSON template.JS
	CtaText            string
	CtaUrl             string
	Summary            string
	Chapters           []Chapter
	ChaptersJSON       template.JS
	SummaryStatus      string
	Description        string
	Duration           int
	HasThumbnail       bool
	JSONLD             template.JS
	SubscriptionPlan   string
}

type expiredPageData struct {
	Nonce string
}

type passwordPageData struct {
	Title      string
	ShareToken string
	Nonce      string
	Branding   brandingConfig
}

var passwordPageTemplate = template.Must(template.New("password").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} — {{.Branding.CompanyName}}</title>
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: {{.Branding.ColorBackground}};
            color: {{.Branding.ColorText}};
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
            background: {{.Branding.ColorSurface}};
            color: #fff;
            font-size: 1rem;
            margin-bottom: 1rem;
            outline: none;
        }
        input[type="password"]:focus { border-color: {{.Branding.ColorAccent}}; }
        button {
            width: 100%;
            background: {{.Branding.ColorAccent}};
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

type emailGatePageData struct {
	Title      string
	ShareToken string
	Nonce      string
}

var emailGatePageTemplate = template.Must(template.New("emailgate").Parse(`<!DOCTYPE html>
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
        input[type="email"] {
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
        input[type="email"]:focus { border-color: #00b67a; }
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
        <h1>{{.Title}}</h1>
        <p>Enter your email to watch this video</p>
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

func injectScriptNonce(scriptTag, nonce string) template.HTML {
	if scriptTag == "" {
		return ""
	}
	injected := strings.Replace(scriptTag, "<script", "<script nonce=\""+nonce+"\"", 1)
	return template.HTML(injected)
}

func (h *Handler) WatchPage(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var title string
	var fileKey string
	var creator string
	var createdAt time.Time
	var shareExpiresAt *time.Time
	var thumbnailKey *string
	var sharePassword *string
	var commentMode string
	var transcriptKey *string
	var transcriptJSON *string
	var transcriptStatus string
	var videoID string
	var ownerID string
	var ownerEmail string
	var viewNotification *string
	var contentType string
	var ubCompanyName, ubLogoKey, ubColorBg, ubColorSurface, ubColorText, ubColorAccent, ubFooterText, ubCustomCSS *string
	var vbCompanyName, vbLogoKey, vbColorBg, vbColorSurface, vbColorText, vbColorAccent, vbFooterText *string
	var downloadEnabled bool
	var ctaText, ctaUrl *string
	var emailGateEnabled bool
	var summaryText *string
	var chaptersJSON *string
	var summaryStatus string
	var duration int
	var subscriptionPlan string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key, v.share_password, v.comment_mode,
		        v.transcript_key, v.transcript_json, v.transcript_status,
		        v.user_id, u.email, v.view_notification, v.content_type,
		        ub.company_name, ub.logo_key, ub.color_background, ub.color_surface, ub.color_text, ub.color_accent, ub.footer_text, ub.custom_css,
		        v.branding_company_name, v.branding_logo_key, v.branding_color_background, v.branding_color_surface, v.branding_color_text, v.branding_color_accent, v.branding_footer_text,
		        v.download_enabled, v.cta_text, v.cta_url, v.email_gate_enabled,
		        v.summary, v.chapters, v.summary_status, v.duration,
		        u.subscription_plan
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 LEFT JOIN user_branding ub ON ub.user_id = v.user_id
		 WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID, &title, &fileKey, &creator, &createdAt, &shareExpiresAt, &thumbnailKey, &sharePassword, &commentMode,
		&transcriptKey, &transcriptJSON, &transcriptStatus,
		&ownerID, &ownerEmail, &viewNotification, &contentType,
		&ubCompanyName, &ubLogoKey, &ubColorBg, &ubColorSurface, &ubColorText, &ubColorAccent, &ubFooterText, &ubCustomCSS,
		&vbCompanyName, &vbLogoKey, &vbColorBg, &vbColorSurface, &vbColorText, &vbColorAccent, &vbFooterText,
		&downloadEnabled,
		&ctaText, &ctaUrl, &emailGateEnabled,
		&summaryText, &chaptersJSON, &summaryStatus, &duration, &subscriptionPlan)
	if err != nil {
		nonce := httputil.NonceFromContext(r.Context())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		if err := notFoundPageTemplate.Execute(w, notFoundPageData{Nonce: nonce}); err != nil {
			slog.Error("watch-page: failed to render not found page", "error", err)
		}
		return
	}

	nonce := httputil.NonceFromContext(r.Context())

	branding := resolveBranding(r.Context(), h.storage,
		brandingSettingsResponse{
			CompanyName: ubCompanyName, LogoKey: ubLogoKey,
			ColorBackground: ubColorBg, ColorSurface: ubColorSurface,
			ColorText: ubColorText, ColorAccent: ubColorAccent, FooterText: ubFooterText,
			CustomCSS: ubCustomCSS,
		},
		brandingSettingsResponse{
			CompanyName: vbCompanyName, LogoKey: vbLogoKey,
			ColorBackground: vbColorBg, ColorSurface: vbColorSurface,
			ColorText: vbColorText, ColorAccent: vbColorAccent, FooterText: vbFooterText,
		},
	)

	if shareExpiresAt != nil && time.Now().After(*shareExpiresAt) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		if err := expiredPageTemplate.Execute(w, expiredPageData{Nonce: nonce}); err != nil {
			slog.Error("watch-page: failed to render expired page", "error", err)
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
				Branding:   branding,
			}); err != nil {
				slog.Error("watch-page: failed to render password page", "error", err)
			}
			return
		}
	}

	if emailGateEnabled {
		if _, ok := hasValidEmailGateCookie(r, h.hmacSecret, shareToken); !ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := emailGatePageTemplate.Execute(w, emailGatePageData{
				Title:      title,
				ShareToken: shareToken,
				Nonce:      nonce,
			}); err != nil {
				slog.Error("watch-page: failed to render email gate page", "error", err)
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
			slog.Error("watch-page: failed to record view", "video_id", videoID, "error", err)
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
	segments := make([]TranscriptSegment, 0)
	if transcriptKey != nil {
		if u, err := h.storage.GenerateDownloadURL(r.Context(), *transcriptKey, 1*time.Hour); err == nil {
			transcriptURL = u
		}
	}
	if transcriptJSON != nil {
		_ = json.Unmarshal([]byte(*transcriptJSON), &segments)
	}

	var summaryStr string
	chapterList := make([]Chapter, 0)
	if summaryText != nil {
		summaryStr = *summaryText
	}
	if chaptersJSON != nil {
		_ = json.Unmarshal([]byte(*chaptersJSON), &chapterList)
	}
	chaptersJSONBytes, _ := json.Marshal(chapterList)

	description := summaryStr
	if description == "" {
		if duration > 0 {
			description = fmt.Sprintf("Video by %s (%s)", creator, formatDuration(duration))
		} else {
			description = title
		}
	}

	reactionEmojisJSON, err := json.Marshal(quickReactionEmojis)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	jsonLD := buildVideoObjectJSONLD(title, description, h.baseURL, shareToken, createdAt, duration, downloadEnabled, thumbnailKey != nil)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := watchPageTemplate.Execute(w, watchPageData{
		Title:              title,
		VideoURL:           videoURL,
		Creator:            creator,
		Date:               createdAt.Format("Jan 2, 2006"),
		Nonce:              nonce,
		ThumbnailURL:       thumbnailURL,
		ShareToken:         shareToken,
		VideoID:            videoID,
		CommentMode:        commentMode,
		TranscriptURL:      transcriptURL,
		TranscriptStatus:   transcriptStatus,
		Segments:           segments,
		BaseURL:            h.baseURL,
		ContentType:        contentType,
		Branding:           branding,
		AnalyticsScript:    injectScriptNonce(h.analyticsScript, nonce),
		DownloadEnabled:    downloadEnabled,
		CustomCSS:          template.CSS(branding.CustomCSS),
		ReactionEmojis:     quickReactionEmojis,
		ReactionEmojisJSON: template.JS(string(reactionEmojisJSON)),
		CtaText:            derefString(ctaText),
		CtaUrl:             derefString(ctaUrl),
		Summary:            summaryStr,
		Chapters:           chapterList,
		ChaptersJSON:       template.JS(chaptersJSONBytes),
		SummaryStatus:      summaryStatus,
		Description:        description,
		Duration:           duration,
		HasThumbnail:       thumbnailKey != nil,
		JSONLD:             template.JS(jsonLD),
		SubscriptionPlan:   subscriptionPlan,
	}); err != nil {
		slog.Error("watch-page: failed to render watch page", "error", err)
	}
}

func (h *Handler) WatchThumbnail(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var thumbnailKey *string
	var shareExpiresAt *time.Time

	err := h.db.QueryRow(r.Context(),
		`SELECT v.thumbnail_key, v.share_expires_at
		 FROM videos v
		 WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&thumbnailKey, &shareExpiresAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if shareExpiresAt != nil && time.Now().After(*shareExpiresAt) {
		http.NotFound(w, r)
		return
	}

	if thumbnailKey == nil {
		http.NotFound(w, r)
		return
	}

	url, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
