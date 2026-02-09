package video

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
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
		SameSite: http.SameSiteStrictMode,
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

	var sharePassword *string
	err := h.db.QueryRow(r.Context(),
		`SELECT share_password FROM videos WHERE share_token = $1 AND status = 'ready'`,
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
