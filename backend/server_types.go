package main

import (
	"bingo/bingo"
	"bingo/room"
	"embed"
	"net/http"
	"sync"
	"time"
)

//go:embed static
var webFiles embed.FS

type session struct {
	RoomID   string
	PlayerID string
	Expires  time.Time
}
type rateEntry struct {
	Count   int
	Expires time.Time
}
type server struct {
	admin        *adminService
	rooms        *room.RoomManager
	mu           sync.Mutex
	sessions     map[string]session
	attempts     map[string]rateEntry
	secureCookie bool
	handler      http.Handler
	subscribers  map[string]map[chan struct{}]struct{}
}

type playerRequest struct {
	Passphrase string `json:"passphrase"`
	PlayerName string `json:"playerName"`
	Color      string `json:"color"`
}

type progressRequest struct {
	Index     *int  `json:"index"`
	Completed *bool `json:"completed"`
}
type cardRequest struct {
	Seed      string     `json:"seed"`
	MaxTime   int        `json:"maxTime"`
	MinTarget int        `json:"minTarget"`
	Rule      bingo.Rule `json:"rule"`
}
