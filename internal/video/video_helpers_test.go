package video

import (
	"testing"
)

func TestCategorizeReferrer(t *testing.T) {
	tests := []struct {
		referer string
		want    string
	}{
		{"", "Direct"},
		{"https://mail.google.com/mail/u/0/", "Email"},
		{"https://outlook.live.com/mail/0/inbox", "Email"},
		{"https://mail.proton.me/u/0/inbox", "Email"},
		{"https://app.slack.com/client/T123/C456", "Slack"},
		{"https://twitter.com/someone/status/123", "Twitter"},
		{"https://x.com/someone/status/123", "Twitter"},
		{"https://www.linkedin.com/feed/", "LinkedIn"},
		{"https://news.ycombinator.com/item?id=123", "Other"},
		{"https://example.com", "Other"},
	}
	for _, tt := range tests {
		t.Run(tt.referer, func(t *testing.T) {
			got := categorizeReferrer(tt.referer)
			if got != tt.want {
				t.Errorf("categorizeReferrer(%q) = %q, want %q", tt.referer, got, tt.want)
			}
		})
	}
}

func TestParseBrowser(t *testing.T) {
	tests := []struct {
		ua   string
		want string
	}{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15", "Safari"},
		{"Mozilla/5.0 (Windows NT 10.0; rv:121.0) Gecko/20100101 Firefox/121.0", "Firefox"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0", "Edge"},
		{"", "Other"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := parseBrowser(tt.ua)
			if got != tt.want {
				t.Errorf("parseBrowser(%q) = %q, want %q", tt.ua, got, tt.want)
			}
		})
	}
}

func TestParseDevice(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{"windows desktop", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36", "Desktop"},
		{"iphone mobile", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "Mobile"},
		{"ipad tablet", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "Tablet"},
		{"android tablet", "Mozilla/5.0 (Linux; Android 14; SM-T736B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Tablet"},
		{"empty ua", "", "Desktop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDevice(tt.ua)
			if got != tt.want {
				t.Errorf("parseDevice(%q) = %q, want %q", tt.ua, got, tt.want)
			}
		})
	}
}
