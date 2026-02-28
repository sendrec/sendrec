package plans

import (
	_ "embed"
	"encoding/json"
	"log"
)

//go:embed free.json
var freeJSON []byte

type FreePlan struct {
	MaxVideosPerMonth       int `json:"maxVideosPerMonth"`
	MaxVideoDurationSeconds int `json:"maxVideoDurationSeconds"`
	MaxPlaylists            int `json:"maxPlaylists"`
	MaxOrgsOwned            int `json:"maxOrgsOwned"`
	MaxOrgMembers           int `json:"maxOrgMembers"`
}

var Free FreePlan

func init() {
	if err := json.Unmarshal(freeJSON, &Free); err != nil {
		log.Fatalf("failed to parse free.json: %v", err)
	}
}
