package video

import "testing"

func TestParseFrameRate(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"30/1", 30},
		{"30000/1001", 29.970029970029970},
		{"60/1", 60},
		{"0/0", 0},
		{"", 0},
		{"invalid", 0},
		{"25", 25},
		{"1000/1", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFrameRate(tt.input)
			diff := got - tt.expected
			if diff < -0.01 || diff > 0.01 {
				t.Errorf("parseFrameRate(%q) = %f, want %f", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNeedsNormalization(t *testing.T) {
	tests := []struct {
		name     string
		props    videoProperties
		expected bool
	}{
		{
			name:     "within limits",
			props:    videoProperties{Width: 1920, Height: 1080, Level: 51, FrameRate: 30, CodecName: "h264"},
			expected: false,
		},
		{
			name:     "720p low level low fps",
			props:    videoProperties{Width: 1280, Height: 720, Level: 31, FrameRate: 30, CodecName: "h264"},
			expected: false,
		},
		{
			name:     "resolution exceeds width",
			props:    videoProperties{Width: 3242, Height: 1080, Level: 51, FrameRate: 30, CodecName: "h264"},
			expected: true,
		},
		{
			name:     "resolution exceeds height",
			props:    videoProperties{Width: 1920, Height: 2868, Level: 51, FrameRate: 30, CodecName: "h264"},
			expected: true,
		},
		{
			name:     "level exceeds",
			props:    videoProperties{Width: 1920, Height: 1080, Level: 62, FrameRate: 30, CodecName: "h264"},
			expected: true,
		},
		{
			name:     "fps exceeds",
			props:    videoProperties{Width: 1920, Height: 1080, Level: 51, FrameRate: 1000, CodecName: "h264"},
			expected: true,
		},
		{
			name:     "non-h264 codec",
			props:    videoProperties{Width: 1280, Height: 720, Level: 31, FrameRate: 30, CodecName: "hevc"},
			expected: true,
		},
		{
			name:     "exactly at limits",
			props:    videoProperties{Width: 1920, Height: 1080, Level: 51, FrameRate: 60, CodecName: "h264"},
			expected: false,
		},
		{
			name:     "fps just over limit",
			props:    videoProperties{Width: 1920, Height: 1080, Level: 51, FrameRate: 60.1, CodecName: "h264"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.props.needsNormalization()
			if got != tt.expected {
				t.Errorf("needsNormalization() = %v, want %v for %+v", got, tt.expected, tt.props)
			}
		})
	}
}
