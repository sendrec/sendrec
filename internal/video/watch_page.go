package video

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

var watchPageTemplate = template.Must(template.New("watch").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} — SendRec</title>
    <meta property="og:title" content="{{.Title}}">
    <meta property="og:type" content="video.other">
    <meta property="og:video" content="{{.VideoURL}}">
    <meta property="og:video:type" content="video/webm">
    {{if .ThumbnailURL}}<meta property="og:image" content="{{.ThumbnailURL}}">{{end}}
    <meta property="og:site_name" content="SendRec">
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: #0a1628;
            color: #ffffff;
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
            border-radius: 8px;
            background: #000;
        }
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
        .branding {
            margin-top: 2rem;
            font-size: 0.75rem;
            color: #64748b;
        }
        .branding a {
            color: #00b67a;
            text-decoration: none;
        }
        .branding a:hover {
            text-decoration: underline;
        }
        .actions {
            margin-top: 1rem;
        }
        .download-btn {
            display: inline-block;
            background: transparent;
            color: #00b67a;
            border: 1px solid #00b67a;
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
            border-top: 1px solid #1e293b;
            padding-top: 1.5rem;
        }
        .comments-header {
            font-size: 1.125rem;
            font-weight: 600;
            margin-bottom: 1rem;
        }
        .comment {
            background: #1e293b;
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
            background: #00b67a;
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
            background: #1e293b;
            color: #fff;
            font-size: 0.875rem;
            font-family: inherit;
            outline: none;
        }
        .comment-form input:focus,
        .comment-form textarea:focus {
            border-color: #00b67a;
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
            background: #00b67a;
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
    </style>
</head>
<body>
    <div class="container">
        <video id="player" controls{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}>
            <source src="{{.VideoURL}}" type="video/webm">
            Your browser does not support video playback.
        </video>
        <script nonce="{{.Nonce}}">
            var v = document.getElementById('player');
            v.play().catch(function() { v.muted = true; v.play(); });
        </script>
        <h1>{{.Title}}</h1>
        <p class="meta">{{.Creator}} · {{.Date}}</p>
        <div class="actions">
            <button class="download-btn" id="download-btn">Download</button>
        </div>
        <script nonce="{{.Nonce}}">
            document.getElementById('download-btn').addEventListener('click', function() {
                fetch('/api/watch/{{.ShareToken}}/download')
                    .then(function(r) { return r.json(); })
                    .then(function(data) { if (data.downloadUrl) window.location.href = data.downloadUrl; });
            });
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
                <textarea id="comment-body" placeholder="Write a comment..." maxlength="5000"></textarea>
                <div class="comment-form-actions">
                    <span id="private-toggle"></span>
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

            function renderComment(c) {
                var authorName = c.authorName || 'Anonymous';
                var badges = '';
                if (c.isOwner) badges += ' <span class="comment-owner-badge">Owner</span>';
                if (c.isPrivate) badges += ' <span class="comment-private-badge">Private</span>';
                return '<div class="comment">' +
                    '<div class="comment-meta">' +
                        '<span class="comment-author">' + escapeHtml(authorName) + '</span>' +
                        badges +
                        '<span>\u00b7 ' + timeAgo(c.createdAt) + '</span>' +
                    '</div>' +
                    '<div class="comment-body">' + escapeHtml(c.body) + '</div>' +
                '</div>';
            }

            function loadComments() {
                var headers = {};
                if (token) headers['Authorization'] = 'Bearer ' + token;
                fetch('/api/watch/' + shareToken + '/comments', { headers: headers })
                    .then(function(r) { return r.json(); })
                    .then(function(data) {
                        if (!data.comments || data.comments.length === 0) {
                            listEl.innerHTML = '<p class="no-comments">No comments yet. Be the first!</p>';
                            headerEl.textContent = 'Comments';
                        } else {
                            headerEl.textContent = 'Comments (' + data.comments.length + ')';
                            listEl.innerHTML = data.comments.map(renderComment).join('');
                        }
                    });
            }

            loadComments();

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
                    body: JSON.stringify({authorName: authorName, authorEmail: authorEmail, body: body, isPrivate: isPrivate})
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
                    submitBtn.disabled = false;
                }).catch(function(err) {
                    errorEl.textContent = err.message; errorEl.style.display = 'block'; submitBtn.disabled = false;
                });
            });
        })();
        </script>
        {{end}}
        <p class="branding">Shared via <a href="https://sendrec.eu">SendRec</a> — open-source video messaging</p>
    </div>
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
	Title        string
	VideoURL     string
	Creator      string
	Date         string
	Nonce        string
	ThumbnailURL string
	ShareToken   string
	CommentMode  string
}

type expiredPageData struct {
	Nonce string
}

type passwordPageData struct {
	Title      string
	ShareToken string
	Nonce      string
}

var passwordPageTemplate = template.Must(template.New("password").Parse(`<!DOCTYPE html>
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
        input[type="password"] {
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
        input[type="password"]:focus { border-color: #00b67a; }
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

func (h *Handler) WatchPage(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var title string
	var fileKey string
	var creator string
	var createdAt time.Time
	var shareExpiresAt time.Time
	var thumbnailKey *string
	var sharePassword *string
	var commentMode string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key, v.share_password, v.comment_mode
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&title, &fileKey, &creator, &createdAt, &shareExpiresAt, &thumbnailKey, &sharePassword, &commentMode)
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

	if time.Now().After(shareExpiresAt) {
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
			if err := passwordPageTemplate.Execute(w, passwordPageData{
				Title:      title,
				ShareToken: shareToken,
				Nonce:      nonce,
			}); err != nil {
				log.Printf("failed to render password page: %v", err)
			}
			return
		}
	}

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
	if err := watchPageTemplate.Execute(w, watchPageData{
		Title:        title,
		VideoURL:     videoURL,
		Creator:      creator,
		Date:         createdAt.Format("Jan 2, 2006"),
		Nonce:        nonce,
		ThumbnailURL: thumbnailURL,
		ShareToken:   shareToken,
		CommentMode:  commentMode,
	}); err != nil {
		log.Printf("failed to render watch page: %v", err)
	}
}
