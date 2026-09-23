package config

type Config struct {
	MaxPlayer int
}

const (
	defaultMaxPlayer = 4
)

func (c *Config) GetConfig() *Config {
	if c.MaxPlayer == 0 {
		return &Config{
			MaxPlayer: defaultMaxPlayer,
		}
	}
	return c
}
