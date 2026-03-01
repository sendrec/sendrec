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
	if Free.MaxOrgsOwned != 1 {
		t.Errorf("expected MaxOrgsOwned=1, got %d", Free.MaxOrgsOwned)
	}
	if Free.MaxOrgMembers != 3 {
		t.Errorf("expected MaxOrgMembers=3, got %d", Free.MaxOrgMembers)
	}
}

func TestRank(t *testing.T) {
	tests := []struct {
		plan string
		rank int
	}{
		{"free", 0},
		{"", 0},
		{"pro", 1},
		{"business", 2},
	}
	for _, tt := range tests {
		if got := Rank(tt.plan); got != tt.rank {
			t.Errorf("Rank(%q) = %d, want %d", tt.plan, got, tt.rank)
		}
	}
	if Rank("pro") >= Rank("business") {
		t.Error("expected pro < business")
	}
	if Rank("free") >= Rank("pro") {
		t.Error("expected free < pro")
	}
}
