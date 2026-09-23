package room

import "bingo/bingo"

type Player struct {
	ID             string
	Name           string
	Color          string
	progress       [25]bool
	bowserDefeated bool
}

type PlayerStatus struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Color          string   `json:"color"`
	Progress       [25]bool `json:"progress"`
	BowserDefeated bool     `json:"bowserDefeated"`
	HasLine        bool     `json:"hasLine"`
}

func (p *Player) updateProgress(index int, completed bool) { p.progress[index] = completed }
func (p *Player) status() PlayerStatus {
	return PlayerStatus{ID: p.ID, Name: p.Name, Color: p.Color, Progress: p.progress, BowserDefeated: p.bowserDefeated, HasLine: bingo.HasLine(p.progress)}
}
