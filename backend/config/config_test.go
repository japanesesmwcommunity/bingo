package config

import "testing"

func TestGetConfig(t *testing.T) {
	for _, test := range []struct{ input, want int }{{0, 4}, {2, 2}, {4, 4}} {
		c := Config{MaxPlayer: test.input}
		if c.GetConfig().MaxPlayer != test.want {
			t.Fatal(test)
		}
	}
}
