package decisions

import (
	"fmt"
	"strings"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	// DefaultBaseURL is the default base URL of the decisions API.
	DefaultBaseURL = "https://routerai.ru/api"
	// DefaultModel is the default decision model.
	DefaultModel = "~typesafe/jev-latest"
	// DefaultBlockThreshold is the default probability at or above which a
	// violation blocks the message.
	DefaultBlockThreshold = 0.7
	// DefaultTimeout is the default timeout for decision API calls.
	DefaultTimeout = 30 * time.Second

	// DefaultCacheTTL is the default TTL for cached answers.
	DefaultCacheTTL = 1 * time.Hour
	// MinCacheTTL is the minimum TTL for cached answers.
	MinCacheTTL = 1 * time.Minute
	// MaxCacheTTL is the maximum TTL for cached answers.
	MaxCacheTTL = 24 * time.Hour

	// MinBlockThreshold is the minimum block threshold.
	MinBlockThreshold = 0.0
	// MaxBlockThreshold is the maximum block threshold.
	MaxBlockThreshold = 1.0
	// MinTimeout is the minimum timeout for decision API calls.
	MinTimeout = 5 * time.Second
	// MaxTimeout is the maximum timeout for decision API calls.
	MaxTimeout = 5 * time.Minute
)

// Config represents the configuration for the decisions plugin.
type Config struct {
	BaseURL        string          // Base URL of the decisions API
	APIKey         string          `json:"-"` // API key for the decision API
	Model          string          // Decision model to use
	BlockThreshold float64         // Probability at or above which a violation blocks (0.0 - 1.0)
	Timeout        time.Duration   // Timeout for API calls
	Context        string          // Optional context added to the state sent to the model
	CacheTTL       time.Duration   // TTL for cached answers
	CacheEnabled   bool            // Whether caching is enabled
	Questions      QuestionsConfig // Text of the fixed three-question rubric
}

// QuestionsConfig holds the configurable text of the three rubric questions.
//
// Question IDs, types and option keys stay fixed in code; only the wording
// can be overridden.
type QuestionsConfig struct {
	Category CategoryQuestionConfig
	Spam     NoulQuestionConfig
	Abuse    NoulQuestionConfig
}

// CategoryQuestionConfig is the text of the category choice question.
type CategoryQuestionConfig struct {
	Instructions string
	Criteria     CategoryCriteriaConfig
}

// CategoryCriteriaConfig is the text of the four category descriptions.
type CategoryCriteriaConfig struct {
	Spam  string
	Scam  string
	Abuse string
	Other string
}

// NoulQuestionConfig is the text of one yes/no question.
type NoulQuestionConfig struct {
	Instructions string
	Criteria     NoulCriteriaConfig
}

// NoulCriteriaConfig is the text of the yes/no criteria.
type NoulCriteriaConfig struct {
	True  string
	False string
}

// NewConfig creates a new configuration from the provided map.
func NewConfig(config map[string]any) (Config, error) {
	var err error
	c := DefaultConfig()

	// Parse BaseURL
	if c.BaseURL, err = plugin.ConfigValue(config, "base_url", c.BaseURL); err != nil {
		return Config{}, fmt.Errorf("failed to parse base_url: %w", err)
	}

	// Parse APIKey
	if c.APIKey, err = plugin.ConfigValue(config, "api_key", c.APIKey); err != nil {
		return Config{}, fmt.Errorf("failed to parse api_key: %w", err)
	}

	// Parse Model
	if c.Model, err = plugin.ConfigValue(config, "model", c.Model); err != nil {
		return Config{}, fmt.Errorf("failed to parse model: %w", err)
	}

	// Parse BlockThreshold
	if c.BlockThreshold, err = plugin.ConfigValue(config, "block_threshold", c.BlockThreshold); err != nil {
		return Config{}, fmt.Errorf("failed to parse block_threshold: %w", err)
	}

	// Parse Timeout
	timeoutStr, err := plugin.ConfigValue(config, "timeout", c.Timeout.String())
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse timeout: %w", err)
	}

	if c.Timeout, err = time.ParseDuration(timeoutStr); err != nil {
		return Config{}, fmt.Errorf("%w: failed to parse timeout: %w", plugin.ErrInvalidConfig, err)
	}

	// Parse Context
	if c.Context, err = plugin.ConfigValue(config, "context", c.Context); err != nil {
		return Config{}, fmt.Errorf("failed to parse context: %w", err)
	}

	// Parse CacheTTL
	cacheTTLStr, err := plugin.ConfigValue(config, "cache_ttl", c.CacheTTL.String())
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse cache_ttl: %w", err)
	}

	if c.CacheTTL, err = time.ParseDuration(cacheTTLStr); err != nil {
		return Config{}, fmt.Errorf("%w: failed to parse cache_ttl: %w", plugin.ErrInvalidConfig, err)
	}

	// Parse CacheEnabled
	if c.CacheEnabled, err = plugin.ConfigValue(config, "cache_enabled", c.CacheEnabled); err != nil {
		return Config{}, fmt.Errorf("failed to parse cache_enabled: %w", err)
	}

	// Parse Questions
	if c.Questions, err = parseQuestions(config["questions"]); err != nil {
		return Config{}, err
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
		BaseURL:        DefaultBaseURL,
		APIKey:         "",
		Model:          DefaultModel,
		BlockThreshold: DefaultBlockThreshold,
		Timeout:        DefaultTimeout,
		Context:        "",
		CacheTTL:       DefaultCacheTTL,
		CacheEnabled:   true,
		Questions:      defaultQuestions(),
	}
}

// Validate checks if the configuration values are valid.
func (c Config) Validate() error {
	// Check Model
	if c.Model == "" {
		return fmt.Errorf("%w: model is required", plugin.ErrInvalidConfig)
	}

	// Check BlockThreshold
	if c.BlockThreshold < MinBlockThreshold || c.BlockThreshold > MaxBlockThreshold {
		return fmt.Errorf(
			"%w: block_threshold must be between %f and %f, got: %f",
			plugin.ErrInvalidConfig,
			MinBlockThreshold,
			MaxBlockThreshold,
			c.BlockThreshold,
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

	// Check Questions
	if err := validateQuestions(c.Questions); err != nil {
		return err
	}

	return nil
}

// defaultQuestions returns the built-in text of the three rubric questions.
func defaultQuestions() QuestionsConfig {
	return QuestionsConfig{
		Category: CategoryQuestionConfig{
			Instructions: "Which category best describes the main purpose of the message?",
			Criteria: CategoryCriteriaConfig{
				Spam:  "Unsolicited advertising, self-promotion, or promotional links; not ordinary conversation or a useful link",
				Scam:  "Phishing, fake giveaways, fake support agents, or requests for credentials and money",
				Abuse: "Insults, threats, hate speech, or harassment aimed at a person",
				Other: "Ordinary conversation, questions, reactions, or anything that fits none of the other categories",
			},
		},
		Spam: NoulQuestionConfig{
			Instructions: "Is this message unsolicited advertising or self-promotion?",
			Criteria: NoulCriteriaConfig{
				True:  "Unsolicited advertising, self-promotion, or a promotional link",
				False: "Legitimate conversation or a useful, on-topic link",
			},
		},
		Abuse: NoulQuestionConfig{
			Instructions: "Is this message insulting, threatening, or harassing someone?",
			Criteria: NoulCriteriaConfig{
				True:  "Insults, threats, slurs, or personal attacks against a person",
				False: "Friendly, neutral, or light banter with no target",
			},
		},
	}
}

// parseQuestions merges raw question overrides into the defaults.
//
// Missing questions keep their default text; unknown question names are rejected.
func parseQuestions(raw any) (QuestionsConfig, error) {
	questions := defaultQuestions()
	if raw == nil {
		return questions, nil
	}

	fields, err := asStringMap(raw)
	if err != nil {
		return QuestionsConfig{}, fmt.Errorf("%w: questions: %w", plugin.ErrInvalidConfig, err)
	}

	for name, value := range fields {
		switch name {
		case categoryQuestionID:
			questions.Category, err = parseCategoryQuestion(value, questions.Category)
		case spamQuestionID:
			questions.Spam, err = parseNoulQuestion(value, questions.Spam, "questions.spam")
		case abuseQuestionID:
			questions.Abuse, err = parseNoulQuestion(value, questions.Abuse, "questions.abuse")
		default:
			return QuestionsConfig{}, fmt.Errorf(
				"%w: unknown question %q (allowed: category, spam, abuse)",
				plugin.ErrInvalidConfig,
				name,
			)
		}
		if err != nil {
			return QuestionsConfig{}, err
		}
	}

	return questions, nil
}

// parseCategoryQuestion merges raw overrides into the current category question text.
func parseCategoryQuestion(raw any, current CategoryQuestionConfig) (CategoryQuestionConfig, error) {
	fields, err := asStringMap(raw)
	if err != nil {
		return CategoryQuestionConfig{}, fmt.Errorf(
			"%w: questions.category: %w",
			plugin.ErrInvalidConfig,
			err,
		)
	}

	for field, value := range fields {
		switch field {
		case "instructions":
			current.Instructions, err = requiredString(value, "questions.category.instructions")
		case "criteria":
			current.Criteria, err = parseCategoryCriteria(value, current.Criteria)
		default:
			return CategoryQuestionConfig{}, fmt.Errorf(
				"%w: unknown field %q in questions.category (allowed: instructions, criteria)",
				plugin.ErrInvalidConfig,
				field,
			)
		}
		if err != nil {
			return CategoryQuestionConfig{}, err
		}
	}

	return current, nil
}

// parseCategoryCriteria merges raw overrides into the current category criteria text.
func parseCategoryCriteria(raw any, current CategoryCriteriaConfig) (CategoryCriteriaConfig, error) {
	fields, err := asStringMap(raw)
	if err != nil {
		return CategoryCriteriaConfig{}, fmt.Errorf(
			"%w: questions.category.criteria: %w",
			plugin.ErrInvalidConfig,
			err,
		)
	}

	for key, value := range fields {
		var text string
		text, err = requiredString(value, "questions.category.criteria."+key)
		if err != nil {
			return CategoryCriteriaConfig{}, err
		}

		switch key {
		case criterionSpam:
			current.Spam = text
		case criterionScam:
			current.Scam = text
		case criterionAbuse:
			current.Abuse = text
		case criterionOther:
			current.Other = text
		default:
			return CategoryCriteriaConfig{}, fmt.Errorf(
				"%w: unknown criterion %q in questions.category.criteria (allowed: spam, scam, abuse, other)",
				plugin.ErrInvalidConfig,
				key,
			)
		}
	}

	return current, nil
}

// parseNoulQuestion merges raw overrides into the current yes/no question text.
func parseNoulQuestion(raw any, current NoulQuestionConfig, path string) (NoulQuestionConfig, error) {
	fields, err := asStringMap(raw)
	if err != nil {
		return NoulQuestionConfig{}, fmt.Errorf("%w: %s: %w", plugin.ErrInvalidConfig, path, err)
	}

	for field, value := range fields {
		switch field {
		case "instructions":
			current.Instructions, err = requiredString(value, path+".instructions")
		case "criteria":
			current.Criteria, err = parseNoulCriteria(value, current.Criteria, path+".criteria")
		default:
			return NoulQuestionConfig{}, fmt.Errorf(
				"%w: unknown field %q in %s (allowed: instructions, criteria)",
				plugin.ErrInvalidConfig,
				field,
				path,
			)
		}
		if err != nil {
			return NoulQuestionConfig{}, err
		}
	}

	return current, nil
}

// parseNoulCriteria merges raw overrides into the current yes/no criteria text.
func parseNoulCriteria(raw any, current NoulCriteriaConfig, path string) (NoulCriteriaConfig, error) {
	fields, err := asStringMap(raw)
	if err != nil {
		return NoulCriteriaConfig{}, fmt.Errorf("%w: %s: %w", plugin.ErrInvalidConfig, path, err)
	}

	for key, value := range fields {
		var text string
		text, err = requiredString(value, path+"."+key)
		if err != nil {
			return NoulCriteriaConfig{}, err
		}

		switch key {
		case criterionTrue:
			current.True = text
		case criterionFalse:
			current.False = text
		default:
			return NoulCriteriaConfig{}, fmt.Errorf(
				"%w: unknown criterion %q in %s (allowed: true, false)",
				plugin.ErrInvalidConfig,
				key,
				path,
			)
		}
	}

	return current, nil
}

// asStringMap normalizes a parsed YAML/JSON object to string keys.
//
// Unquoted YAML keys such as `true:` decode as booleans and yield map[any]any;
// both representations are accepted.
func asStringMap(value any) (map[string]any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, nil
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[fmt.Sprintf("%v", key)] = item
		}
		return normalized, nil
	default:
		return nil, fmt.Errorf("%w: expected an object, got %T", plugin.ErrInvalidConfig, value)
	}
}

// requiredString returns a non-empty string value or an error naming the path.
func requiredString(value any, path string) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: %s must be a string, got %T", plugin.ErrInvalidConfig, path, value)
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("%w: %s must not be empty", plugin.ErrInvalidConfig, path)
	}

	return text, nil
}

// validateQuestions checks that every rubric text is non-empty.
func validateQuestions(questions QuestionsConfig) error {
	texts := []struct {
		path  string
		value string
	}{
		{"questions.category.instructions", questions.Category.Instructions},
		{"questions.category.criteria.spam", questions.Category.Criteria.Spam},
		{"questions.category.criteria.scam", questions.Category.Criteria.Scam},
		{"questions.category.criteria.abuse", questions.Category.Criteria.Abuse},
		{"questions.category.criteria.other", questions.Category.Criteria.Other},
		{"questions.spam.instructions", questions.Spam.Instructions},
		{"questions.spam.criteria.true", questions.Spam.Criteria.True},
		{"questions.spam.criteria.false", questions.Spam.Criteria.False},
		{"questions.abuse.instructions", questions.Abuse.Instructions},
		{"questions.abuse.criteria.true", questions.Abuse.Criteria.True},
		{"questions.abuse.criteria.false", questions.Abuse.Criteria.False},
	}

	for _, item := range texts {
		if strings.TrimSpace(item.value) == "" {
			return fmt.Errorf("%w: %s must not be empty", plugin.ErrInvalidConfig, item.path)
		}
	}

	return nil
}
