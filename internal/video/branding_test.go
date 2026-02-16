package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func newBrandingHandler(t *testing.T, mock pgxmock.PgxPoolIface, storage *mockStorage) *Handler {
	t.Helper()
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.brandingEnabled = true
	return handler
}

// --- Feature gating ---

func TestGetBrandingSettings_DisabledReturns403(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/branding", handler.GetBrandingSettings)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/branding", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "branding requires a paid plan" {
		t.Errorf("expected error %q, got %q", "branding requires a paid plan", errMsg)
	}
}

func TestPutBrandingSettings_DisabledReturns403(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(setBrandingRequest{})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/branding", handler.PutBrandingSettings)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/branding", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestSetVideoBranding_DisabledReturns403(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(setVideoBrandingRequest{})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/branding", handler.SetVideoBranding)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/video-123/branding", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

// --- GET branding settings ---

func TestGetBrandingSettings_NoBranding(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	mock.ExpectQuery(`SELECT company_name, logo_key, color_background, color_surface, color_text, color_accent, footer_text`).
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/branding", handler.GetBrandingSettings)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/branding", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp brandingSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.CompanyName != nil {
		t.Errorf("expected nil companyName, got %q", *resp.CompanyName)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGetBrandingSettings_WithBranding(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	companyName := "Acme Corp"
	colorBg := "#112233"
	colorAccent := "#00ff00"

	mock.ExpectQuery(`SELECT company_name, logo_key, color_background, color_surface, color_text, color_accent, footer_text`).
		WithArgs(testUserID).
		WillReturnRows(
			pgxmock.NewRows([]string{"company_name", "logo_key", "color_background", "color_surface", "color_text", "color_accent", "footer_text"}).
				AddRow(&companyName, (*string)(nil), &colorBg, (*string)(nil), (*string)(nil), &colorAccent, (*string)(nil)),
		)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/branding", handler.GetBrandingSettings)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/branding", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp brandingSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.CompanyName == nil || *resp.CompanyName != "Acme Corp" {
		t.Errorf("expected companyName %q, got %v", "Acme Corp", resp.CompanyName)
	}
	if resp.ColorBackground == nil || *resp.ColorBackground != "#112233" {
		t.Errorf("expected colorBackground %q, got %v", "#112233", resp.ColorBackground)
	}
	if resp.LogoKey != nil {
		t.Errorf("expected nil logoKey, got %q", *resp.LogoKey)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- PUT branding settings ---

func TestPutBrandingSettings_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	companyName := "Acme Corp"
	colorBg := "#112233"

	mock.ExpectExec(`INSERT INTO user_branding`).
		WithArgs(testUserID, &companyName, &colorBg, (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(setBrandingRequest{
		CompanyName:     &companyName,
		ColorBackground: &colorBg,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/branding", handler.PutBrandingSettings)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/branding", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPutBrandingSettings_InvalidColor(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	badColor := "not-a-color"
	body, _ := json.Marshal(setBrandingRequest{
		ColorBackground: &badColor,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/branding", handler.PutBrandingSettings)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/branding", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invalid colorBackground: must be a hex color like #1a2b3c" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

func TestPutBrandingSettings_CompanyNameTooLong(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	longName := make([]byte, 201)
	for i := range longName {
		longName[i] = 'a'
	}
	name := string(longName)
	body, _ := json.Marshal(setBrandingRequest{
		CompanyName: &name,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/branding", handler.PutBrandingSettings)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/branding", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "company name must be 200 characters or fewer" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

func TestPutBrandingSettings_FooterTextTooLong(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	longFooter := make([]byte, 501)
	for i := range longFooter {
		longFooter[i] = 'x'
	}
	footer := string(longFooter)
	body, _ := json.Marshal(setBrandingRequest{
		FooterText: &footer,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/branding", handler.PutBrandingSettings)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/branding", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "footer text must be 500 characters or fewer" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

// --- Logo upload ---

func TestUploadBrandingLogo_ValidPNG(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload?signed=abc"}
	handler := newBrandingHandler(t, mock, storage)

	mock.ExpectExec(`INSERT INTO user_branding`).
		WithArgs(testUserID, "branding/"+testUserID+"/logo.png").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/png",
		ContentLength: 100000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/branding/logo", handler.UploadBrandingLogo)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/branding/logo", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp logoUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.UploadURL != "https://s3.example.com/upload?signed=abc" {
		t.Errorf("unexpected upload URL: %s", resp.UploadURL)
	}
	if resp.LogoKey != "branding/"+testUserID+"/logo.png" {
		t.Errorf("unexpected logo key: %s", resp.LogoKey)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUploadBrandingLogo_ValidSVG(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload?signed=svg"}
	handler := newBrandingHandler(t, mock, storage)

	mock.ExpectExec(`INSERT INTO user_branding`).
		WithArgs(testUserID, "branding/"+testUserID+"/logo.svg").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/svg+xml",
		ContentLength: 5000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/branding/logo", handler.UploadBrandingLogo)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/branding/logo", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp logoUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.LogoKey != "branding/"+testUserID+"/logo.svg" {
		t.Errorf("unexpected logo key: %s", resp.LogoKey)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUploadBrandingLogo_TooLarge(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/png",
		ContentLength: 600 * 1024,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/branding/logo", handler.UploadBrandingLogo)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/branding/logo", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "logo must be 512KB or smaller" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

func TestUploadBrandingLogo_InvalidType(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/jpeg",
		ContentLength: 100000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/branding/logo", handler.UploadBrandingLogo)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/branding/logo", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "logo must be PNG or SVG" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

// --- Delete logo ---

func TestDeleteBrandingLogo_WithLogo(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := newBrandingHandler(t, mock, storage)

	logoKey := "branding/" + testUserID + "/logo.png"
	mock.ExpectQuery(`SELECT logo_key FROM user_branding`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"logo_key"}).AddRow(&logoKey))

	mock.ExpectExec(`UPDATE user_branding SET logo_key = NULL`).
		WithArgs(testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/settings/branding/logo", handler.DeleteBrandingLogo)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/settings/branding/logo", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDeleteBrandingLogo_NoBrandingExists(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	mock.ExpectQuery(`SELECT logo_key FROM user_branding`).
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/settings/branding/logo", handler.DeleteBrandingLogo)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/settings/branding/logo", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Per-video branding ---

func TestGetVideoBranding_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})
	videoID := "video-branding-1"

	companyName := "Acme Corp"
	mock.ExpectQuery(`SELECT branding_company_name, branding_logo_key, branding_color_background, branding_color_surface`).
		WithArgs(videoID, testUserID).
		WillReturnRows(
			pgxmock.NewRows([]string{"branding_company_name", "branding_logo_key", "branding_color_background", "branding_color_surface", "branding_color_text", "branding_color_accent", "branding_footer_text"}).
				AddRow(&companyName, (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil)),
		)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/branding", handler.GetVideoBranding)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/branding", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp brandingSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.CompanyName == nil || *resp.CompanyName != "Acme Corp" {
		t.Errorf("expected companyName %q, got %v", "Acme Corp", resp.CompanyName)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGetVideoBranding_NotOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})
	videoID := "video-not-mine"

	mock.ExpectQuery(`SELECT branding_company_name`).
		WithArgs(videoID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/branding", handler.GetVideoBranding)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/branding", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetVideoBranding_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})
	videoID := "video-branding-1"

	companyName := "My Company"
	colorBg := "#aabbcc"

	mock.ExpectExec(`UPDATE videos SET`).
		WithArgs(&companyName, &colorBg, (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(setVideoBrandingRequest{
		CompanyName:     &companyName,
		ColorBackground: &colorBg,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/branding", handler.SetVideoBranding)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/branding", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetVideoBranding_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})
	videoID := "nonexistent"

	mock.ExpectExec(`UPDATE videos SET`).
		WithArgs((*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body, _ := json.Marshal(setVideoBrandingRequest{})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/branding", handler.SetVideoBranding)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/branding", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetVideoBranding_InvalidColor(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := newBrandingHandler(t, mock, &mockStorage{})

	badColor := "rgb(0,0,0)"
	body, _ := json.Marshal(setVideoBrandingRequest{
		ColorAccent: &badColor,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/branding", handler.SetVideoBranding)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/video-123/branding", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invalid colorAccent: must be a hex color like #1a2b3c" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

// --- Branding resolution ---

func TestResolveBranding_DefaultsOnly(t *testing.T) {
	storage := &mockStorage{}
	cfg := resolveBranding(
		nil,
		storage,
		brandingSettingsResponse{},
		brandingSettingsResponse{},
	)

	if cfg.CompanyName != "SendRec" {
		t.Errorf("expected company name %q, got %q", "SendRec", cfg.CompanyName)
	}
	if cfg.ColorBackground != "#0a1628" {
		t.Errorf("expected background %q, got %q", "#0a1628", cfg.ColorBackground)
	}
	if cfg.ColorAccent != "#00b67a" {
		t.Errorf("expected accent %q, got %q", "#00b67a", cfg.ColorAccent)
	}
	if cfg.HasCustomLogo {
		t.Error("expected no custom logo")
	}
	if cfg.LogoURL != "/images/logo.png" {
		t.Errorf("expected default logo URL, got %q", cfg.LogoURL)
	}
}

func TestResolveBranding_UserOverrides(t *testing.T) {
	storage := &mockStorage{}
	companyName := "Acme"
	colorBg := "#111111"

	cfg := resolveBranding(
		nil,
		storage,
		brandingSettingsResponse{
			CompanyName:     &companyName,
			ColorBackground: &colorBg,
		},
		brandingSettingsResponse{},
	)

	if cfg.CompanyName != "Acme" {
		t.Errorf("expected company name %q, got %q", "Acme", cfg.CompanyName)
	}
	if cfg.ColorBackground != "#111111" {
		t.Errorf("expected background %q, got %q", "#111111", cfg.ColorBackground)
	}
	if cfg.ColorAccent != "#00b67a" {
		t.Errorf("expected default accent %q, got %q", "#00b67a", cfg.ColorAccent)
	}
}

func TestResolveBranding_VideoOverridesUser(t *testing.T) {
	storage := &mockStorage{}
	userCompany := "User Corp"
	videoCompany := "Video Corp"
	userBg := "#111111"
	videoBg := "#222222"

	cfg := resolveBranding(
		nil,
		storage,
		brandingSettingsResponse{
			CompanyName:     &userCompany,
			ColorBackground: &userBg,
		},
		brandingSettingsResponse{
			CompanyName:     &videoCompany,
			ColorBackground: &videoBg,
		},
	)

	if cfg.CompanyName != "Video Corp" {
		t.Errorf("expected company name %q, got %q", "Video Corp", cfg.CompanyName)
	}
	if cfg.ColorBackground != "#222222" {
		t.Errorf("expected background %q, got %q", "#222222", cfg.ColorBackground)
	}
}

func TestResolveBranding_CustomLogo(t *testing.T) {
	storage := &mockStorage{downloadURL: "https://storage.sendrec.eu/branding/user/logo.png?signed=abc"}
	logoKey := "branding/user/logo.png"

	cfg := resolveBranding(
		nil,
		storage,
		brandingSettingsResponse{
			LogoKey: &logoKey,
		},
		brandingSettingsResponse{},
	)

	if !cfg.HasCustomLogo {
		t.Error("expected custom logo")
	}
	if cfg.LogoURL != "https://storage.sendrec.eu/branding/user/logo.png?signed=abc" {
		t.Errorf("unexpected logo URL: %s", cfg.LogoURL)
	}
}

// --- Hex color validation ---

func TestIsValidHexColor(t *testing.T) {
	valid := []string{"#000000", "#ffffff", "#1a2B3c", "#AABBCC"}
	for _, c := range valid {
		if !isValidHexColor(c) {
			t.Errorf("expected %q to be valid", c)
		}
	}

	invalid := []string{"", "000000", "#fff", "#1234567", "rgb(0,0,0)", "#gggggg", "not-a-color"}
	for _, c := range invalid {
		if isValidHexColor(c) {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}
