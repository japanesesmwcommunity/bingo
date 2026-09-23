package bingo

import (
	_ "embed"
	"encoding/json"
)

//go:embed route_segments.json
var routeSegmentConfig []byte

var retiredRouteAreas = loadRetiredRouteAreas(routeSegmentConfig)

func loadRetiredRouteAreas(contents []byte) map[string]bool {
	var config struct {
		RetiredAreas []string `json:"retiredAreas"`
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		panic(err) // Bundled configuration must be valid before the server starts.
	}
	result := make(map[string]bool, len(config.RetiredAreas))
	for _, area := range config.RetiredAreas {
		result[area] = true
	}
	return result
}
