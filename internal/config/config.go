package config

import (
	"fmt"
	"os"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor"
	plug "github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/go-core-fx/config"
	"github.com/samber/lo"
)

type Bot struct {
	AdminID      int64 `koanf:"admin_id"`
	BanThreshold uint8 `koanf:"ban_threshold"`

	Media botMedia `koanf:"media"`
}

type botMedia struct {
	MaxBytes int64         `koanf:"max_bytes"`
	Timeout  time.Duration `koanf:"timeout"`
}

type telegram struct {
	Token    string        `koanf:"token"`
	ProxyURL string        `koanf:"proxy_url"`
	Timeout  time.Duration `koanf:"timeout"`
}

type plugin struct {
	Enabled  bool           `koanf:"enabled"`
	Priority int            `koanf:"priority"`
	Config   map[string]any `koanf:"config"`
}

type Censor struct {
	Strategy    censor.ExecutionStrategy `koanf:"strategy"`
	Plugins     map[string]plugin        `koanf:"plugins"`
	Timeout     time.Duration            `koanf:"timeout"`
	EnabledOnly bool                     `koanf:"enabled_only"`

	ErrorAction plug.Action `koanf:"error_action"`
	SkipAction  plug.Action `koanf:"skip_action"`

	// Deprecated
	Blacklist []string `koanf:"blacklist"`
}

type Storage struct {
	TTL time.Duration `koanf:"ttl"`
}

type cacheConfig struct {
	URL           string        `koanf:"url"`
	SweepInterval time.Duration `koanf:"sweep_interval"`
}

type http struct {
	Address     string   `koanf:"address"`
	ProxyHeader string   `koanf:"proxy_header"`
	Proxies     []string `koanf:"proxies"`
}

type Config struct {
	Bot      Bot         `koanf:"bot"`
	Telegram telegram    `koanf:"telegram"`
	Censor   Censor      `koanf:"censor"`
	Storage  Storage     `koanf:"storage"`
	Cache    cacheConfig `koanf:"cache"`
	HTTP     http        `koanf:"http"`
}

func Default() Config {
	//nolint:exhaustruct_v5,mnd // default values
	return Config{
		Bot: Bot{
			BanThreshold: 3,

			Media: botMedia{
				MaxBytes: 10 * 1024 * 1024,
				Timeout:  5 * time.Second,
			},
		},
		Telegram: telegram{
			Token:    "",
			ProxyURL: "",
			Timeout:  time.Minute,
		},
		Censor: Censor{
			Strategy:    censor.StrategySequential,
			Timeout:     30 * time.Second,
			EnabledOnly: true,
			Plugins: map[string]plugin{
				"keyword": {
					Enabled:  true,
					Priority: 10,
					Config: map[string]any{
						// The plugin config parser expects the YAML-decoded
						// []any shape, not the struct-typed []string.
						"blacklist": lo.ToAnySlice([]string{"$", "долл"}),
					},
				},
			},
			ErrorAction: plug.ActionBlock,
			SkipAction:  plug.ActionAllow,
		},
		Storage: Storage{
			TTL: 5 * time.Minute,
		},
		Cache: cacheConfig{
			URL:           "memory://?max_entries=1000",
			SweepInterval: time.Minute,
		},
		HTTP: http{
			Address:     "127.0.0.1:3000",
			ProxyHeader: "X-Forwarded-For",
			Proxies:     []string{},
		},
	}
}

func New() (Config, error) {
	cfg := Default()

	options := []config.Option{}
	if yamlPath := os.Getenv("CONFIG_PATH"); yamlPath != "" {
		options = append(options, config.WithLocalYAML(yamlPath))
	}

	if err := config.Load(&cfg, options...); err != nil {
		return Config{}, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}
