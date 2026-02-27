package plans

import "testing"

func TestFreePlanValues(t *testing.T) {
	if Free.MaxVideosPerMonth != 25 {
		t.Errorf("expected MaxVideosPerMonth=25, got %d", Free.MaxVideosPerMonth)
	}
	if Free.MaxVideoDurationSeconds != 300 {
		t.Errorf("expected MaxVideoDurationSeconds=300, got %d", Free.MaxVideoDurationSeconds)
	}
	if Free.MaxPlaylists != 3 {
		t.Errorf("expected MaxPlaylists=3, got %d", Free.MaxPlaylists)
	}
}
