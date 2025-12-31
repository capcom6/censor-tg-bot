package llm

import (
	"fmt"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	// DefaultModel is the default LLM model.
	DefaultModel = "nvidia/nemotron-nano-9b-v2:free"
	// DefaultConfidenceThreshold is the default confidence threshold for blocking messages.
	DefaultConfidenceThreshold = 0.8
	// DefaultTimeout is the default timeout for LLM API calls.
	DefaultTimeout = 30 * time.Second
	// DefaultTemperature is the default temperature for the LLM.
	DefaultTemperature = 0.1

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
)

// Config represents the configuration for the LLM plugin.
type Config struct {
	APIKey              string        // API key for the LLM service
	Model               string        // LLM model to use
	ConfidenceThreshold float64       // Confidence threshold for blocking (0.0 - 1.0)
	Timeout             time.Duration // Timeout for API calls
	Prompt              string        // Custom prompt for the LLM
	Temperature         float64       // Temperature for the LLM
}

// NewConfig creates a new configuration from the provided map.
func NewConfig(config map[string]any) (Config, error) {
	var err error
	c := DefaultConfig()

	// Parse APIKey
	if c.APIKey, err = plugin.ConfigValue(config, "api_key", c.APIKey); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse Model
	if c.Model, err = plugin.ConfigValue(config, "model", c.Model); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	// Parse ConfidenceThreshold
	if c.ConfidenceThreshold, err = plugin.ConfigValue(config, "confidence_threshold", c.ConfidenceThreshold); err != nil {
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

	// Validate the configuration
	if validErr := c.Validate(); validErr != nil {
		return Config{}, validErr
	}

	return c, nil
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		APIKey:              "",
		ConfidenceThreshold: DefaultConfidenceThreshold,
		Timeout:             DefaultTimeout,
		Model:               DefaultModel,
		Prompt:              "Analyze the following message for inappropriate content, spam, or violations. Respond with JSON: {\"inappropriate\": boolean, \"confidence\": float, \"reason\": string}",
		Temperature:         DefaultTemperature,
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

	return nil
}
