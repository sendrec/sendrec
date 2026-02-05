package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userIDKey contextKey = "userID"

type Handler struct {
	db            database.DBTX
	jwtSecret     string
	secureCookies bool
}

func NewHandler(db database.DBTX, jwtSecret string, secureCookies bool) *Handler {
	return &Handler{db: db, jwtSecret: jwtSecret, secureCookies: secureCookies}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"accessToken"`
}


func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email, password, and name are required")
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid email address")
		return
	}

	if len(req.Password) < 8 {
		httputil.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	if len(req.Password) > 72 {
		httputil.WriteError(w, http.StatusBadRequest, "password must be at most 72 characters")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	var userID string
	err = h.db.QueryRow(r.Context(),
		"INSERT INTO users (email, password, name) VALUES ($1, $2, $3) RETURNING id",
		req.Email, string(hashedPassword), req.Name,
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httputil.WriteError(w, http.StatusConflict, "email already registered")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	accessToken, err := GenerateAccessToken(h.jwtSecret, userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}

	refreshToken, err := GenerateRefreshToken(h.jwtSecret, userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	httputil.WriteJSON(w, http.StatusCreated, tokenResponse{AccessToken: accessToken})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var userID, hashedPassword string
	err := h.db.QueryRow(r.Context(),
		"SELECT id, password FROM users WHERE email = $1", req.Email,
	).Scan(&userID, &hashedPassword)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	accessToken, err := GenerateAccessToken(h.jwtSecret, userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}

	refreshToken, err := GenerateRefreshToken(h.jwtSecret, userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	httputil.WriteJSON(w, http.StatusOK, tokenResponse{AccessToken: accessToken})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "refresh token not found")
		return
	}

	claims, err := ValidateToken(h.jwtSecret, cookie.Value)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	if claims.TokenType != "refresh" {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	accessToken, err := GenerateAccessToken(h.jwtSecret, claims.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}

	refreshToken, err := GenerateRefreshToken(h.jwtSecret, claims.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	httputil.WriteJSON(w, http.StatusOK, tokenResponse{AccessToken: accessToken})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httputil.WriteError(w, http.StatusUnauthorized, "authorization header required")
			return
		}

		tokenStr, found := strings.CutPrefix(authHeader, "Bearer ")
		if !found {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		claims, err := ValidateToken(h.jwtSecret, tokenStr)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		if claims.TokenType != "access" {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid token type")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(userIDKey).(string)
	return userID
}

func (h *Handler) setRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(RefreshTokenDuration / time.Second),
	})
}
