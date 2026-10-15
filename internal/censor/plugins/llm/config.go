package llm

import (
	"fmt"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	// DefaultBaseURL is the default base URL for the LLM API.
	DefaultBaseURL = "https://openrouter.ai/api/v1"
	// DefaultModel is the default LLM model.
	DefaultModel = "nvidia/nemotron-nano-9b-v2:free"
	// DefaultConfidenceThreshold is the default confidence threshold for blocking messages.
	DefaultConfidenceThreshold = 0.8
	// DefaultPrompt is the default instruction sent with the message text.
	DefaultPrompt = "Analyze the following message for inappropriate content, spam, or violations. " +
		"Respond with JSON: {\"inappropriate\": boolean, \"confidence\": float, \"reason\": string}"
	// DefaultTimeout is the default timeout for LLM API calls.
	DefaultTimeout = 30 * time.Second
	// DefaultTemperature is the default temperature for the LLM.
	DefaultTemperature = 0.1

	// DefaultImagePrompt is the default instruction sent with every photo. It asks for
	// a description because the reason reaches an admin notification rendered as HTML,
	// and a description is what makes that notification actionable.
	DefaultImagePrompt = "Analyze the attached photo for inappropriate content, spam, or violations. " +
		"Describe what the photo shows in your reason. " +
		"Respond with JSON: {\"inappropriate\": boolean, \"confidence\": float, \"reason\": string}"

	// DefaultCacheTTL is the default TTL for cached responses.
	DefaultCacheTTL = 1 * time.Hour
	// DefaultCacheMaxSize is the default maximum size of the cache.
	DefaultCacheMaxSize = 1000
	// MinCacheTTL is the minimum TTL for cached responses.
	MinCacheTTL = 1 * time.Minute
	// MaxCacheTTL is the maximum TTL for cached responses.
	MaxCacheTTL = 24 * time.Hour

	// MinConfidenceThreshold is the minimum confidence threshold.
	MinConfidenceThreshold = 0.0
	// MaxConfidenceThreshold is the maximum confidence threshold.
	MaxConfidenceThreshold = 1.0
	// MinTimeout is the minimum timeout duration.
	MinTimeout = 5 * time.Second
	// MaxTimeout is the maximum timeout duration.
	MaxTimeout = 5 * time.Minute
	// MinTemperature is the minimum temperature for LLM API calls.
	MinTemperature = 0.0
	// MaxTemperature is the maximum temperature for LLM API calls.
	MaxTemperature = 2.0

	// MinVisionModelLength is the minimum length of a configured vision model id.
	MinVisionModelLength = 1
	// MaxVisionModelLength bounds vision_model so a copy-and-pasted blob cannot become
	// an unbounded request parameter.
	MaxVisionModelLength = 256
	// MinImagePromptLength is the minimum length of a configured image prompt.
	MinImagePromptLength = 1
	// MaxImagePromptLength bounds image_prompt for the same reason.
	MaxImagePromptLength = 4096
)

// Config represents the configuration for the LLM plugin.
type Config struct {
	BaseURL             string        // Base URL for the LLM service
	APIKey              string        `json:"-"` // API key for the LLM service
	Model               string        // LLM model to use
	ConfidenceThreshold float64       // Confidence threshold for blocking (0.0 - 1.0)
	Timeout             time.Duration // Timeout for API calls
	Prompt              string        // Custom prompt for the LLM
	Temperature         float64       // Temperature for the LLM
	CacheTTL            time.Duration // TTL for cached responses
	CacheMaxSize        int           // Maximum number of cached responses
	CacheEnabled        bool          // Whether caching is enabled

	// ImageEnabled turns on photo moderation. It is false by default because it ships
	// every photo to a third-party provider and costs money per call.
	ImageEnabled bool
	// VisionModel is the model asked to judge a photo. There is deliberately no
	// default: the configured Model is a small text model, and no vision model was
	// verified as dependable enough to delete and ban on. Required when ImageEnabled.
	VisionModel string
	// ImagePrompt is the instruction sent with every photo.
	ImagePrompt string

	// MaxInlineImageBytes is an optional local request-body safety budget for one
	// photo, measured in DECODED image bytes - the same unit as Media.SizeBytes and
	// bot.media.max_bytes, and deliberately NOT the base64-inflated or HTTP
	// request-body size, which stdlib base64 encoding expands by 4/3.
	//
	// STATUS: this is local policy, not a provider contract - "a local request-body
	// safety budget; the provider publishes no inline-image limit". There is
	// therefore NO default: 0 means the guard does not run and bot.media.max_bytes is
	// the only bound. A capping default would silently kill the feature.
	//
	// It must be <= bot.media.max_bytes to have any effect. A larger value is not a
	// startup error (the plugin config is an untyped map, so the plugin cannot see
	// bot.media at all), it is simply unreachable: Media.SizeBytes <=
	// bot.media.max_bytes is already guaranteed twice over, by the byte filter in
	// internal/bot/media.go and by the io.LimitReader hard cap in
	// pkg/tgbotapifx, so any derived threshold at or above bot.media.max_bytes would
	// be dead code. That is also why deriving it from bot.media.max_bytes was
	// rejected - the knob exists so an operator can say "do not ship big photos to a
	// third party", which a Telegram download-feasibility bound does not express.
	MaxInlineImageBytes int64
}

// NewConfig creates a new configuration from the provided map.
func NewConfig(config map[string]any) (Config, error) {
	var err error
	c := DefaultConfig()

	// Parse BaseURL
	if c.BaseURL, err = plugin.ConfigValue(config, "base_url", c.BaseURL); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse APIKey
	if c.APIKey, err = plugin.ConfigValue(config, "api_key", c.APIKey); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse Model
	if c.Model, err = plugin.ConfigValue(config, "model", c.Model); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse ConfidenceThreshold
	if c.ConfidenceThreshold, err = plugin.ConfigValue(
		config,
		"confidence_threshold",
		c.ConfidenceThreshold,
	); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse Timeout
	timeoutStr, err := plugin.ConfigValue(config, "timeout", c.Timeout.String())
	if err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	if c.Timeout, err = time.ParseDuration(timeoutStr); err != nil {
		return Config{}, fmt.Errorf("%w: failed to parse timeout: %w", plugin.ErrInvalidConfig, err)
	}

	// Parse Prompt
	if c.Prompt, err = plugin.ConfigValue(config, "prompt", c.Prompt); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse Temperature
	if c.Temperature, err = plugin.ConfigValue(config, "temperature", c.Temperature); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse CacheTTL
	cacheTTLStr, err := plugin.ConfigValue(config, "cache_ttl", c.CacheTTL.String())
	if err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	if c.CacheTTL, err = time.ParseDuration(cacheTTLStr); err != nil {
		return Config{}, fmt.Errorf("%w: failed to parse cache_ttl: %w", plugin.ErrInvalidConfig, err)
	}

	// Parse CacheMaxSize
	if c.CacheMaxSize, err = plugin.ConfigValue(config, "cache_max_size", c.CacheMaxSize); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse CacheEnabled
	if c.CacheEnabled, err = plugin.ConfigValue(config, "cache_enabled", c.CacheEnabled); err != nil {
		return Config{}, fmt.Errorf("parse cache_enabled: %w", err)
	}

	// Parse ImageEnabled
	if c.ImageEnabled, err = plugin.ConfigValue(config, "image_enabled", c.ImageEnabled); err != nil {
		return Config{}, fmt.Errorf("parse image_enabled: %w", err)
	}

	// Parse VisionModel
	if c.VisionModel, err = plugin.ConfigValue(config, "vision_model", c.VisionModel); err != nil {
		return Config{}, fmt.Errorf("parse vision_model: %w", err)
	}

	// Parse ImagePrompt
	if c.ImagePrompt, err = plugin.ConfigValue(config, "image_prompt", c.ImagePrompt); err != nil {
		return Config{}, fmt.Errorf("parse image_prompt: %w", err)
	}

	// Parse MaxInlineImageBytes. The seed is an untyped int constant because the
	// plugin config map is untyped: a YAML integer reaches it as int, so an int64
	// seed would reject every working configuration. The field itself stays int64 so
	// its unit matches Media.SizeBytes and bot.media.max_bytes.
	const noInlineBudget = 0

	inlineBudget, err := plugin.ConfigValue(config, "max_inline_image_bytes", noInlineBudget)
	if err != nil {
		return Config{}, fmt.Errorf("parse max_inline_image_bytes: %w", err)
	}

	c.MaxInlineImageBytes = int64(inlineBudget)

	// Validate the configuration
	if validErr := c.Validate(); validErr != nil {
		return Config{}, validErr
	}

	return c, nil
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:             DefaultBaseURL,
		APIKey:              "",
		ConfidenceThreshold: DefaultConfidenceThreshold,
		Timeout:             DefaultTimeout,
		Model:               DefaultModel,
		Prompt:              DefaultPrompt,
		Temperature:         DefaultTemperature,
		CacheTTL:            DefaultCacheTTL,
		CacheMaxSize:        DefaultCacheMaxSize,
		CacheEnabled:        true,

		ImageEnabled: false,
		VisionModel:  "",
		ImagePrompt:  DefaultImagePrompt,
		// No inline budget by default: bot.media.max_bytes is already the download
		// bound, and a capping default would silently disable photo moderation.
		MaxInlineImageBytes: 0,
	}
}

// Validate checks if the configuration values are valid.
func (c Config) Validate() error {
	// Check Model
	if c.Model == "" {
		return fmt.Errorf(
			"%w: model is required",
			plugin.ErrInvalidConfig,
		)
	}

	// Check ConfidenceThreshold
	if c.ConfidenceThreshold < MinConfidenceThreshold || c.ConfidenceThreshold > MaxConfidenceThreshold {
		return fmt.Errorf(
			"%w: confidence_threshold must be between %f and %f, got: %f",
			plugin.ErrInvalidConfig,
			MinConfidenceThreshold,
			MaxConfidenceThreshold,
			c.ConfidenceThreshold,
		)
	}

	// Check Timeout
	if c.Timeout < MinTimeout || c.Timeout > MaxTimeout {
		return fmt.Errorf(
			"%w: timeout must be between %s and %s, got: %s",
			plugin.ErrInvalidConfig,
			MinTimeout,
			MaxTimeout,
			c.Timeout,
		)
	}

	// Check Prompt
	if c.Prompt == "" {
		return fmt.Errorf(
			"%w: prompt is required",
			plugin.ErrInvalidConfig,
		)
	}

	// Check Temperature
	if c.Temperature < MinTemperature || c.Temperature > MaxTemperature {
		return fmt.Errorf(
			"%w: temperature must be between %f and %f, got: %f",
			plugin.ErrInvalidConfig,
			MinTemperature,
			MaxTemperature,
			c.Temperature,
		)
	}

	// Check CacheTTL
	if c.CacheEnabled && (c.CacheTTL < MinCacheTTL || c.CacheTTL > MaxCacheTTL) {
		return fmt.Errorf(
			"%w: cache_ttl must be between %s and %s, got: %s",
			plugin.ErrInvalidConfig,
			MinCacheTTL,
			MaxCacheTTL,
			c.CacheTTL,
		)
	}

	// Check CacheMaxSize
	if c.CacheEnabled && c.CacheMaxSize <= 0 {
		return fmt.Errorf(
			"%w: cache_max_size must be positive, got: %d",
			plugin.ErrInvalidConfig,
			c.CacheMaxSize,
		)
	}

	// Check VisionModel. It is required exactly when image moderation is on: turning
	// the feature on without saying which model judges the photos would otherwise
	// silently send them to the TEXT model, and then block on a text verdict.
	if c.ImageEnabled && len(c.VisionModel) < MinVisionModelLength {
		return fmt.Errorf(
			"%w: vision_model is required when image_enabled is true",
			plugin.ErrInvalidConfig,
		)
	}

	if len(c.VisionModel) > MaxVisionModelLength {
		return fmt.Errorf(
			"%w: vision_model must be at most %d characters, got: %d",
			plugin.ErrInvalidConfig,
			MaxVisionModelLength,
			len(c.VisionModel),
		)
	}

	// Check ImagePrompt
	if len(c.ImagePrompt) > MaxImagePromptLength {
		return fmt.Errorf(
			"%w: image_prompt must be at most %d characters, got: %d",
			plugin.ErrInvalidConfig,
			MaxImagePromptLength,
			len(c.ImagePrompt),
		)
	}

	// Check MaxInlineImageBytes. Only a nonsensical negative value is refused: it
	// means the operator wants a budget no image can satisfy, and silently treating
	// it as "no budget" would turn that intent into the opposite policy. This is a
	// config-validity check, NOT the per-message size guard: no value is compared
	// against bot.media.max_bytes here, because this plugin cannot see bot.media and
	// refusing to boot over it could take down an inert deployment.
	if c.MaxInlineImageBytes < 0 {
		return fmt.Errorf(
			"%w: max_inline_image_bytes must not be negative, got: %d",
			plugin.ErrInvalidConfig,
			c.MaxInlineImageBytes,
		)
	}

	return nil
}
