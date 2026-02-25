package languages

import "testing"

func TestIsValidTranscriptionLanguage(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"auto", true},
		{"en", true},
		{"de", true},
		{"fr", true},
		{"ja", true},
		{"zh", true},
		{"", false},
		{"xx", false},
		{"english", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := IsValidTranscriptionLanguage(tt.code); got != tt.valid {
				t.Errorf("IsValidTranscriptionLanguage(%q) = %v, want %v", tt.code, got, tt.valid)
			}
		})
	}
}

func TestTranscriptionLanguages_HasDisplayNames(t *testing.T) {
	langs := TranscriptionLanguages()
	if len(langs) < 99 {
		t.Errorf("expected at least 99 languages, got %d", len(langs))
	}
	for _, l := range langs {
		if l.Code == "" || l.Name == "" {
			t.Errorf("language with empty code or name: %+v", l)
		}
	}
}
