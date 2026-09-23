package main

import (
	"bingo/room"
	"time"
)

const (
	roomSyncInterval   = time.Second
	socketWriteTimeout = 5 * time.Second
	socketReadTimeout  = 45 * time.Second
	socketPingInterval = 20 * time.Second
	maxRoomSubscribers = 32
)

type roomEvent struct {
	Type     string       `json:"type"`
	Room     *room.Status `json:"room,omitempty"`
	PlayerID string       `json:"playerId,omitempty"`
	Error    string       `json:"error,omitempty"`
}
