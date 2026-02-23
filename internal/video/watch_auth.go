package video

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
	"golang.org/x/crypto/bcrypt"
)

func hashSharePassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func checkSharePassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func watchCookieName(shareToken string) string {
	prefix := shareToken
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return "wa_" + prefix
}

func signWatchCookie(hmacSecret, shareToken, passwordHash string) string {
	hashPrefix := passwordHash
	if len(hashPrefix) > 16 {
		hashPrefix = hashPrefix[:16]
	}
	mac := hmac.New(sha256.New, []byte(hmacSecret))
	mac.Write([]byte(shareToken + "|" + hashPrefix))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyWatchCookie(hmacSecret, shareToken, passwordHash, cookieValue string) bool {
	expected := signWatchCookie(hmacSecret, shareToken, passwordHash)
	return hmac.Equal([]byte(expected), []byte(cookieValue))
}

func setWatchCookie(w http.ResponseWriter, shareToken, cookieValue string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     watchCookieName(shareToken),
		Value:    cookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   int(7 * 24 * time.Hour / time.Second),
	})
}

func hasValidWatchCookie(r *http.Request, hmacSecret, shareToken, passwordHash string) bool {
	cookie, err := r.Cookie(watchCookieName(shareToken))
	if err != nil {
		return false
	}
	return verifyWatchCookie(hmacSecret, shareToken, passwordHash, cookie.Value)
}

type verifyPasswordRequest struct {
	Password string `json:"password"`
}

func (h *Handler) VerifyWatchPassword(w http.ResponseWriter, r *http.Request) {
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
		`SELECT share_password FROM videos WHERE share_token = $1 AND status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&sharePassword)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
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

// --- Email gate cookie helpers ---

func emailGateCookieName(shareToken string) string {
	prefix := shareToken
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return "eg_" + prefix
}

func signEmailGateCookie(hmacSecret, shareToken, email string) string {
	mac := hmac.New(sha256.New, []byte(hmacSecret))
	mac.Write([]byte(shareToken + "|" + email))
	sig := hex.EncodeToString(mac.Sum(nil))
	return email + "|" + sig
}

func verifyEmailGateCookie(hmacSecret, shareToken, cookieValue string) (string, bool) {
	parts := strings.SplitN(cookieValue, "|", 2)
	if len(parts) != 2 {
		return "", false
	}
	email := parts[0]
	expected := signEmailGateCookie(hmacSecret, shareToken, email)
	if !hmac.Equal([]byte(expected), []byte(cookieValue)) {
		return "", false
	}
	return email, true
}

func setEmailGateCookie(w http.ResponseWriter, shareToken, value string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     emailGateCookieName(shareToken),
		Value:    value,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteNoneMode,
	})
}

func hasValidEmailGateCookie(r *http.Request, hmacSecret, shareToken string) (string, bool) {
	cookie, err := r.Cookie(emailGateCookieName(shareToken))
	if err != nil {
		return "", false
	}
	return verifyEmailGateCookie(hmacSecret, shareToken, cookie.Value)
}

type identifyViewerRequest struct {
	Email string `json:"email"`
}

func (h *Handler) IdentifyViewer(w http.ResponseWriter, r *http.Request) {
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

	var videoID string
	err := h.db.QueryRow(r.Context(),
		`SELECT v.id FROM videos v WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&videoID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	ip := clientIP(r)
	hash := viewerHash(ip, r.UserAgent())

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := h.db.Exec(ctx,
			`INSERT INTO video_viewers (video_id, email, viewer_hash) VALUES ($1, $2, $3)
			 ON CONFLICT (video_id, email) DO NOTHING`,
			videoID, req.Email, hash,
		); err != nil {
			slog.Error("watch-auth: failed to record viewer identity", "video_id", videoID, "error", err)
		}
	}()

	sig := signEmailGateCookie(h.hmacSecret, shareToken, req.Email)
	setEmailGateCookie(w, shareToken, sig, h.secureCookies)

	w.WriteHeader(http.StatusOK)
}
