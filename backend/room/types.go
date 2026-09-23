package room

import (
	"bingo/bingo"
	"sync"
	"time"
)

const MaxPlayers = 4

type Mode string

const (
	Race    Mode = "race"
	Lockout Mode = "lockout"
)

type CreateOptions struct {
	Name       string `json:"name"`
	Passphrase string `json:"passphrase"`
	Seed       string `json:"seed"`
	Mode       Mode   `json:"mode"`
	bingo.Options
}

type RoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

type Room struct {
	mu         sync.RWMutex
	id         string
	name       string
	ownerID    string
	card       bingo.BingoCard
	players    map[string]*Player
	mode       Mode
	options    bingo.Options
	startedAt  time.Time
	finishedAt time.Time
	winnerID   string
	deleted    bool
	version    uint64
	salt       string
	passhash   [32]byte
}

type Status struct {
	Version          uint64          `json:"version"`
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	OwnerID          string          `json:"ownerId"`
	Card             bingo.BingoCard `json:"card"`
	Players          []PlayerStatus  `json:"players"`
	Mode             Mode            `json:"mode"`
	Options          bingo.Options   `json:"options"`
	StartedAt        *time.Time      `json:"startedAt"`
	FinishedAt       *time.Time      `json:"finishedAt"`
	ElapsedSeconds   int             `json:"elapsedSeconds"`
	WinnerID         string          `json:"winnerId"`
	EstimatedMinutes float64         `json:"estimatedMinutes"`
}

// Summary contains only the information shown in the public room directory.
type Summary struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Mode        Mode       `json:"mode"`
	Rule        bingo.Rule `json:"rule"`
	PlayerCount int        `json:"playerCount"`
	MaxPlayers  int        `json:"maxPlayers"`
	Status      string     `json:"status"`
}
