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
}

var Free FreePlan

func init() {
	if err := json.Unmarshal(freeJSON, &Free); err != nil {
		log.Fatalf("failed to parse free.json: %v", err)
	}
}
