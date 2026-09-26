// Package room manages in-memory games. All mutable state is protected by the room lock.
package room

import (
	"bingo/bingo"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrNotFound  = errors.New("ルームまたはプレイヤーが見つかりません")
	ErrForbidden = errors.New("合言葉または操作権限を確認してください")
	ErrConflict  = errors.New("現在の試合状態では操作できません（終了後・占有済みなど）")
	ErrFull      = errors.New("満員です。1ルームの定員は4名です")
)

func NewRoomManager() *RoomManager { return &RoomManager{rooms: make(map[string]*Room)} }

func (rm *RoomManager) CreateRoom(o CreateOptions) (*Room, string, error) {
	o.Name = strings.TrimSpace(o.Name)
	if o.Name == "" || utf8.RuneCountInString(o.Name) > 80 {
		return nil, "", fmt.Errorf("ルーム名は1〜80文字で入力してください")
	}
	if strings.TrimSpace(o.Passphrase) == "" || len(o.Passphrase) > 256 {
		return nil, "", fmt.Errorf("合言葉は1〜256バイトで入力してください")
	}
	if o.Mode == "" {
		o.Mode = Race
	}
	if o.Mode != Race && o.Mode != Lockout {
		return nil, "", fmt.Errorf("対戦形式が不正です")
	}
	options, err := o.Options.Normalize()
	if err != nil {
		return nil, "", err
	}
	card, err := bingo.CreateCard(o.Seed, options)
	if err != nil {
		return nil, "", err
	}
	r := &Room{id: uuid.NewString(), ownerID: uuid.NewString(), name: o.Name, players: make(map[string]*Player), mode: o.Mode, options: options, card: card, salt: uuid.NewString(), version: 1}
	r.passhash = sha256.Sum256([]byte(r.salt + o.Passphrase))
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rm.rooms == nil {
		rm.rooms = make(map[string]*Room)
	}
	rm.rooms[r.id] = r
	return r, r.ownerID, nil
}

func (rm *RoomManager) GetRoom(id string) (*Room, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	r, ok := rm.rooms[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (rm *RoomManager) ListActiveRooms() []Summary {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rooms := make([]Summary, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		r.mu.RLock()
		if !r.deleted && r.finishedAt.IsZero() {
			status := "waiting"
			if len(r.players) > 0 {
				status = "playing"
			}
			rooms = append(rooms, Summary{
				ID: r.id, Name: r.name, Mode: r.mode, Rule: r.options.Rule,
				PlayerCount: len(r.players), MaxPlayers: MaxPlayers, Status: status,
			})
		}
		r.mu.RUnlock()
	}
	// Keep the order stable across refreshes, with waiting rooms first.
	sort.Slice(rooms, func(i, j int) bool {
		if rooms[i].Status != rooms[j].Status {
			return rooms[i].Status == "waiting"
		}
		if rooms[i].Name != rooms[j].Name {
			return rooms[i].Name < rooms[j].Name
		}
		return rooms[i].ID < rooms[j].ID
	})
	return rooms
}

func (rm *RoomManager) DeleteRoom(id, playerID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	r, ok := rm.rooms[id]
	if !ok {
		return ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ownerID != playerID {
		return ErrForbidden
	}
	r.deleted = true
	r.version++
	delete(rm.rooms, id)
	return nil
}

func (r *Room) AddPlayer(passphrase, name, color string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted {
		return "", ErrNotFound
	}
	hash := sha256.Sum256([]byte(r.salt + passphrase))
	if subtle.ConstantTimeCompare(hash[:], r.passhash[:]) != 1 {
		return "", ErrForbidden
	}
	id := uuid.NewString()
	if err := r.addPlayer(id, name, color); err != nil {
		return "", err
	}
	return id, nil
}

// JoinOwner explicitly enrolls the authenticated organizer as a player.
func (r *Room) JoinOwner(ownerID, name, color string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted {
		return ErrNotFound
	}
	if ownerID != r.ownerID {
		return ErrForbidden
	}
	if r.players[ownerID] != nil {
		return nil
	}
	return r.addPlayer(ownerID, name, color)
}

// addPlayer requires the room write lock.
func (r *Room) addPlayer(id, name, color string) error {
	if !r.finishedAt.IsZero() {
		return ErrConflict
	}
	if len(r.players) >= MaxPlayers {
		return ErrFull
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 40 {
		return fmt.Errorf("表示名は1〜40文字で入力してください")
	}
	for _, p := range r.players {
		if p.Name == name {
			return fmt.Errorf("その表示名はすでに使われています")
		}
	}
	if color == "" {
		color = "#52c7a5"
	}
	if len(color) != 7 || color[0] != '#' || strings.Trim(color[1:], "0123456789abcdefABCDEF") != "" {
		return fmt.Errorf("カラーは #RRGGBB 形式で指定してください")
	}
	r.players[id] = &Player{ID: id, Name: name, Color: color}
	r.version++
	return nil
}

// CheckAccess accepts organizers independently of player enrollment.
func (r *Room) CheckAccess(id string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.deleted || (id != r.ownerID && r.players[id] == nil) {
		return ErrNotFound
	}
	return nil
}

func (r *Room) DeletePlayer(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted || r.players[id] == nil {
		return ErrNotFound
	}
	if !r.finishedAt.IsZero() {
		return ErrConflict
	}
	delete(r.players, id)
	r.version++
	return nil
}

func (r *Room) KickPlayer(ownerID, playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted {
		return ErrNotFound
	}
	if ownerID != r.ownerID || playerID == r.ownerID {
		return ErrForbidden
	}
	if !r.finishedAt.IsZero() {
		return ErrConflict
	}
	if r.players[playerID] == nil {
		return ErrNotFound
	}
	delete(r.players, playerID)
	r.version++
	return nil
}

func (r *Room) GetPlayer(id string) (PlayerStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.deleted || r.players[id] == nil {
		return PlayerStatus{}, ErrNotFound
	}
	return r.players[id].status(), nil
}

func (r *Room) GetRoomStatus() Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := Status{ID: r.id, Name: r.name, OwnerID: r.ownerID, Card: r.card.Clone(), Mode: r.mode, Options: r.options, WinnerID: r.winnerID, Players: make([]PlayerStatus, 0, len(r.players)), EstimatedMinutes: bingo.EstimatedMinutes(r.card, r.options)}
	s.Version = r.version
	for _, p := range r.players {
		s.Players = append(s.Players, p.status())
	}
	sort.Slice(s.Players, func(i, j int) bool { return s.Players[i].ID < s.Players[j].ID })
	if !r.finishedAt.IsZero() {
		finished := r.finishedAt
		s.FinishedAt = &finished
	}
	return s
}

func (r *Room) GenerateCard(seed string, maxTime, minTarget int, rule bingo.Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted {
		return ErrNotFound
	}
	if !r.finishedAt.IsZero() {
		return ErrConflict
	}
	for _, p := range r.players {
		if p.bowserDefeated {
			return ErrConflict
		}
		for _, completed := range p.progress {
			if completed {
				return ErrConflict
			}
		}
	}
	o := r.options
	o.MaxTime = maxTime
	o.MinTarget = minTarget
	o.Rule = rule
	o, err := o.Normalize()
	if err != nil {
		return err
	}
	card, err := bingo.CreateCard(seed, o)
	if err != nil {
		return err
	}
	r.card = card
	r.options = o
	r.version++
	return nil
}

func (r *Room) Finish(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted {
		return ErrNotFound
	}
	if playerID != r.ownerID {
		return ErrForbidden
	}
	if !r.finishedAt.IsZero() {
		return ErrConflict
	}
	r.finishedAt = time.Now()
	r.version++
	return nil
}

func (r *Room) UpdatePlayerProgress(id string, index int, completed bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.canPlay(id); err != nil {
		return err
	}
	if index < 0 || index >= 25 {
		return fmt.Errorf("マス番号は0〜24で指定してください")
	}
	if completed && r.mode == Lockout {
		for other, p := range r.players {
			if other != id && p.progress[index] {
				return ErrConflict
			}
		}
	}
	r.players[id].updateProgress(index, completed)
	r.checkWinner(id)
	r.version++
	return nil
}

func (r *Room) UpdateBowser(id string, completed bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.canPlay(id); err != nil {
		return err
	}
	r.players[id].bowserDefeated = completed
	r.checkWinner(id)
	r.version++
	return nil
}

// These helpers are called only while holding the write lock.
func (r *Room) canPlay(id string) error {
	if !r.deleted && id == r.ownerID && r.players[id] == nil {
		return ErrForbidden
	}
	if r.deleted || r.players[id] == nil {
		return ErrNotFound
	}
	if !r.finishedAt.IsZero() {
		return ErrConflict
	}
	return nil
}

func (r *Room) checkWinner(id string) {
	p := r.players[id]
	if bingo.HasLine(p.progress) && (r.options.Rule == bingo.LineOnly || p.bowserDefeated) {
		r.winnerID = id
		r.finishedAt = time.Now()
	}
}
