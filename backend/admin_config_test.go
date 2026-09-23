package main

import (
	"net/http"
	"testing"
)

func TestAdminConfig(t *testing.T) {
	values := map[string]string{}
	get := func(key string) string { return values[key] }
	c, err := loadAdminConfig(get)
	if err != nil || newAdminService(c, "unused").enabled() {
		t.Fatal(c, err)
	}
	values["DISCORD_CLIENT_ID"] = "123456789012345678"
	if _, err := loadAdminConfig(get); err == nil {
		t.Fatal("partial config accepted")
	}
	values["DISCORD_CLIENT_SECRET"] = "server-only-secret"
	values["DISCORD_REDIRECT_URI"] = "http://127.0.0.1:8080/api/admin/callback"
	values["ADMIN_DISCORD_USER_IDS"] = " 234567890123456789, 345678901234567890 "
	c, err = loadAdminConfig(get)
	if err != nil || len(c.AllowedIDs) != 2 {
		t.Fatal(c, err)
	}
	a := newAdminService(c, "unused")
	if !a.enabled() || a.client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("unsafe client")
	}
	for _, uri := range []string{"https://example.com/api/admin/callback", "http://localhost:8080/api/admin/callback", "http://[::1]:8080/api/admin/callback"} {
		values["DISCORD_REDIRECT_URI"] = uri
		if _, err := loadAdminConfig(get); err != nil {
			t.Fatal(uri, err)
		}
	}
	for _, item := range [][2]string{{"DISCORD_CLIENT_ID", "abc"}, {"ADMIN_DISCORD_USER_IDS", "123,"}, {"ADMIN_DISCORD_USER_IDS", "0"}, {"DISCORD_REDIRECT_URI", "http://example.com/api/admin/callback"}, {"DISCORD_REDIRECT_URI", "https://example.com/wrong"}, {"DISCORD_REDIRECT_URI", "https://a:b@example.com/api/admin/callback"}, {"DISCORD_REDIRECT_URI", "https://example.com/api/admin/callback?x=y"}, {"DISCORD_REDIRECT_URI", "%"}} {
		old := values[item[0]]
		values[item[0]] = item[1]
		if _, err := loadAdminConfig(get); err == nil {
			t.Fatal(item)
		}
		values[item[0]] = old
	}
	for _, id := range []string{"", "-1", "+1", "01", "18446744073709551616"} {
		if validDiscordID(id) {
			t.Fatal(id)
		}
	}
}
