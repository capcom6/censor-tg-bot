package forwarded

import (
	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

type Config struct {
	AllowedUserIDs []int64
	AllowedChatIDs []int64
}

func NewConfig(config map[string]any) (Config, error) {
	var err error
	cfg := Config{
		AllowedUserIDs: []int64{},
		AllowedChatIDs: []int64{},
	}

	// Parse allowed chat IDs
	if cfg.AllowedChatIDs, err = plugin.SliceFromAnyOrDefault(config, "allowed_chat_ids", []int64{}); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse allowed user IDs
	if cfg.AllowedUserIDs, err = plugin.SliceFromAnyOrDefault(config, "allowed_user_ids", []int64{}); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	return cfg, nil
}
