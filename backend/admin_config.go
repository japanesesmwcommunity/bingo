package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func loadAdminConfig(getenv func(string) string) (adminConfig, error) {
	c := adminConfig{
		ClientID:     strings.TrimSpace(getenv("DISCORD_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(getenv("DISCORD_CLIENT_SECRET")),
		RedirectURI:  strings.TrimSpace(getenv("DISCORD_REDIRECT_URI")),
		AllowedIDs:   make(map[string]bool),
	}
	ids := strings.TrimSpace(getenv("ADMIN_DISCORD_USER_IDS"))
	if c.ClientID == "" && c.ClientSecret == "" && c.RedirectURI == "" && ids == "" {
		return c, nil
	}
	if c.ClientID == "" || c.ClientSecret == "" || c.RedirectURI == "" || ids == "" {
		return c, fmt.Errorf("管理者認証には DISCORD_CLIENT_ID・DISCORD_CLIENT_SECRET・DISCORD_REDIRECT_URI・ADMIN_DISCORD_USER_IDS の設定が必要です")
	}
	if !validDiscordID(c.ClientID) {
		return c, fmt.Errorf("DISCORD_CLIENT_ID が不正です")
	}
	for _, raw := range strings.Split(ids, ",") {
		id := strings.TrimSpace(raw)
		if !validDiscordID(id) {
			return c, fmt.Errorf("ADMIN_DISCORD_USER_IDS はDiscordユーザーIDをカンマ区切りで指定してください")
		}
		c.AllowedIDs[id] = true
	}
	u, err := url.Parse(c.RedirectURI)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "/api/admin/callback" || u.RawQuery != "" || u.Fragment != "" {
		return c, fmt.Errorf("DISCORD_REDIRECT_URI は公開URLの /api/admin/callback を指定してください")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return c, fmt.Errorf("DISCORD_REDIRECT_URI はHTTPSが必要です（ローカル開発を除く）")
	}
	return c, nil
}

func validDiscordID(id string) bool {
	n, err := strconv.ParseUint(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatUint(n, 10) == id
}

func newAdminService(config adminConfig, path string) *adminService {
	return &adminService{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		states: make(map[string]time.Time), sessions: make(map[string]adminSession),
		catalog: catalogStore{path: path},
	}
}

func (a *adminService) enabled() bool {
	return a.config.ClientID != "" && a.config.ClientSecret != "" && a.config.RedirectURI != "" && len(a.config.AllowedIDs) > 0
}
