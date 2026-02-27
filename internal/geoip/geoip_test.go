package geoip

import (
	"testing"
)

func TestNew_EmptyPath(t *testing.T) {
	r, err := New("")
	if err != nil {
		t.Fatalf("expected no error for empty path, got %v", err)
	}
	country, city := r.Lookup("8.8.8.8")
	if country != "" || city != "" {
		t.Errorf("expected empty results for nil resolver, got country=%q city=%q", country, city)
	}
}

func TestNew_InvalidPath(t *testing.T) {
	r, err := New("/nonexistent/path.mmdb")
	if err != nil {
		t.Fatalf("expected no error for missing file (graceful fallback), got %v", err)
	}
	country, city := r.Lookup("8.8.8.8")
	if country != "" || city != "" {
		t.Errorf("expected empty results, got country=%q city=%q", country, city)
	}
}

func TestLookup_EmptyIP(t *testing.T) {
	r, _ := New("")
	country, city := r.Lookup("")
	if country != "" || city != "" {
		t.Errorf("expected empty results for empty IP, got country=%q city=%q", country, city)
	}
}

func TestLookup_InvalidIP(t *testing.T) {
	r, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	country, city := r.Lookup("not-an-ip")
	if country != "" {
		t.Errorf("expected empty country for invalid IP, got %q", country)
	}
	if city != "" {
		t.Errorf("expected empty city for invalid IP, got %q", city)
	}
}

func TestLookup_LoopbackIP(t *testing.T) {
	r, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	country, city := r.Lookup("127.0.0.1")
	if country != "" {
		t.Errorf("expected empty country for loopback, got %q", country)
	}
	if city != "" {
		t.Errorf("expected empty city for loopback, got %q", city)
	}
}

func TestClose_NilDB(t *testing.T) {
	r, _ := New("")
	if err := r.Close(); err != nil {
		t.Errorf("expected no error closing nil resolver, got %v", err)
	}
}

func TestClose_AlreadyClosed(t *testing.T) {
	r, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("first close failed: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second close failed: %v", err)
	}
}
