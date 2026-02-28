package validate

import "testing"

func TestTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "My Video", ""},
		{"empty", "", ""},
		{"at limit", string(make([]byte, MaxTitleLength)), ""},
		{"over limit", string(make([]byte, MaxTitleLength+1)), "title must be 500 characters or fewer"},
	}
	for _, tt := range tests {
		if got := Title(tt.input); got != tt.want {
			t.Errorf("Title(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestPlaylistTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "My Playlist", ""},
		{"empty", "", ""},
		{"at limit", string(make([]byte, MaxPlaylistTitleLength)), ""},
		{"over limit", string(make([]byte, MaxPlaylistTitleLength+1)), "playlist title must be 200 characters or fewer"},
	}
	for _, tt := range tests {
		if got := PlaylistTitle(tt.input); got != tt.want {
			t.Errorf("PlaylistTitle(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestPlaylistDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "A description", ""},
		{"empty", "", ""},
		{"at limit", string(make([]byte, MaxPlaylistDescriptionLength)), ""},
		{"over limit", string(make([]byte, MaxPlaylistDescriptionLength+1)), "playlist description must be 2000 characters or fewer"},
	}
	for _, tt := range tests {
		if got := PlaylistDescription(tt.input); got != tt.want {
			t.Errorf("PlaylistDescription(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestFolderName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "Work", ""},
		{"at limit", string(make([]byte, MaxFolderNameLength)), ""},
		{"over limit", string(make([]byte, MaxFolderNameLength+1)), "folder name must be 100 characters or fewer"},
	}
	for _, tt := range tests {
		if got := FolderName(tt.input); got != tt.want {
			t.Errorf("FolderName(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestTagName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "demo", ""},
		{"at limit", string(make([]byte, MaxTagNameLength)), ""},
		{"over limit", string(make([]byte, MaxTagNameLength+1)), "tag name must be 50 characters or fewer"},
	}
	for _, tt := range tests {
		if got := TagName(tt.input); got != tt.want {
			t.Errorf("TagName(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestCommentBody(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "Great video!", ""},
		{"at limit", string(make([]byte, MaxCommentBodyLength)), ""},
		{"over limit", string(make([]byte, MaxCommentBodyLength+1)), "comment must be 5000 characters or fewer"},
	}
	for _, tt := range tests {
		if got := CommentBody(tt.input); got != tt.want {
			t.Errorf("CommentBody(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestCompanyName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "Acme Corp", ""},
		{"at limit", string(make([]byte, MaxCompanyNameLength)), ""},
		{"over limit", string(make([]byte, MaxCompanyNameLength+1)), "company name must be 200 characters or fewer"},
	}
	for _, tt := range tests {
		if got := CompanyName(tt.input); got != tt.want {
			t.Errorf("CompanyName(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestFooterText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "Powered by SendRec", ""},
		{"at limit", string(make([]byte, MaxFooterTextLength)), ""},
		{"over limit", string(make([]byte, MaxFooterTextLength+1)), "footer text must be 500 characters or fewer"},
	}
	for _, tt := range tests {
		if got := FooterText(tt.input); got != tt.want {
			t.Errorf("FooterText(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestCustomCSS(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "body { color: red; }", ""},
		{"at limit", string(make([]byte, MaxCustomCSSLength)), ""},
		{"over limit", string(make([]byte, MaxCustomCSSLength+1)), "custom CSS must be 10240 characters or fewer"},
	}
	for _, tt := range tests {
		if got := CustomCSS(tt.input); got != tt.want {
			t.Errorf("CustomCSS(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestSlackWebhookURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "https://hooks.slack.com/services/xxx", ""},
		{"at limit", string(make([]byte, MaxSlackWebhookURLLength)), ""},
		{"over limit", string(make([]byte, MaxSlackWebhookURLLength+1)), "Slack webhook URL must be 500 characters or fewer"},
	}
	for _, tt := range tests {
		if got := SlackWebhookURL(tt.input); got != tt.want {
			t.Errorf("SlackWebhookURL(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestWebhookURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "https://example.com/hook", ""},
		{"at limit", string(make([]byte, MaxWebhookURLLength)), ""},
		{"over limit", string(make([]byte, MaxWebhookURLLength+1)), "webhook URL must be 500 characters or fewer"},
	}
	for _, tt := range tests {
		if got := WebhookURL(tt.input); got != tt.want {
			t.Errorf("WebhookURL(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestAPIKeyName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "my-key", ""},
		{"at limit", string(make([]byte, MaxAPIKeyNameLength)), ""},
		{"over limit", string(make([]byte, MaxAPIKeyNameLength+1)), "API key name must be 100 characters or fewer"},
	}
	for _, tt := range tests {
		if got := APIKeyName(tt.input); got != tt.want {
			t.Errorf("APIKeyName(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestOrgName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "Acme Corp", ""},
		{"empty", "", ""},
		{"at limit", string(make([]byte, MaxOrgNameLength)), ""},
		{"over limit", string(make([]byte, MaxOrgNameLength+1)), "organization name must be 200 characters or fewer"},
	}
	for _, tt := range tests {
		if got := OrgName(tt.input); got != tt.want {
			t.Errorf("OrgName(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestOrgSlug(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "acme-corp", ""},
		{"empty", "", ""},
		{"at limit", string(make([]byte, MaxOrgSlugLength)), ""},
		{"over limit", string(make([]byte, MaxOrgSlugLength+1)), "organization slug must be 100 characters or fewer"},
	}
	for _, tt := range tests {
		if got := OrgSlug(tt.input); got != tt.want {
			t.Errorf("OrgSlug(%q [len=%d]) = %q, want %q", tt.name, len(tt.input), got, tt.want)
		}
	}
}

func TestFieldLimits(t *testing.T) {
	fl := FieldLimits()
	if fl["title"] != MaxTitleLength {
		t.Errorf("FieldLimits()[title] = %d, want %d", fl["title"], MaxTitleLength)
	}
	if fl["folderName"] != MaxFolderNameLength {
		t.Errorf("FieldLimits()[folderName] = %d, want %d", fl["folderName"], MaxFolderNameLength)
	}
	if len(fl) < 13 {
		t.Errorf("FieldLimits() returned %d entries, expected at least 13", len(fl))
	}
}
