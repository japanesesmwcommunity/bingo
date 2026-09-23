package main

import (
	"bingo/bingo"
	"net/http"
	"sync"
	"time"
)

const (
	discordAuthorizeURL  = "https://discord.com/oauth2/authorize"
	discordTokenURL      = "https://discord.com/api/oauth2/token"
	discordUserURL       = "https://discord.com/api/v10/users/@me"
	adminCookieName      = "bingo_admin"
	oauthCookieName      = "bingo_oauth_state"
	adminSessionLifetime = 8 * time.Hour
	oauthStateLifetime   = 5 * time.Minute
	adminMaxBody         = 1 << 20
)

type adminConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AllowedIDs   map[string]bool
}

type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type adminSession struct {
	User    discordUser
	Expires time.Time
}

type adminService struct {
	config   adminConfig
	client   *http.Client
	mu       sync.Mutex
	states   map[string]time.Time
	sessions map[string]adminSession
	catalog  catalogStore
}

type catalogStore struct {
	mu   sync.Mutex
	path string
}

type catalogDocument struct {
	BowserRoutes   []bingo.FinishRoute `json:"bowserRoutes,omitempty"`
	Goals          []bingo.BingoGoal   `json:"goals"`
	RouteAreaTimes map[string]int      `json:"routeAreaTimes,omitempty"`
	Revision       string              `json:"revision"`
}
