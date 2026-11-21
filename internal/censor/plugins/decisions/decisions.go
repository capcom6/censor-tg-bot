package decisions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"go.uber.org/zap"
)

const (
	// categoryQuestionID is the id of the category choice question.
	categoryQuestionID = "category"
	// spamQuestionID is the id of the unsolicited-advertising yes/no question.
	spamQuestionID = "spam"
	// abuseQuestionID is the id of the personal-attack yes/no question.
	abuseQuestionID = "abuse"

	// Fixed criterion keys of the rubric questions.
	criterionSpam  = "spam"
	criterionScam  = "scam"
	criterionAbuse = "abuse"
	criterionOther = "other"
	criterionTrue  = "true"
	criterionFalse = "false"

	// pluginName is the plugin identifier.
	pluginName = "decisions"

	// defaultPriority runs the plugin after the cheap checks and before the
	// heavier LLM plugin: a decisions call is fast and cheap, a chat
	// completion is neither.
	defaultPriority = 250
)

// Metadata returns the plugin registration.
func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: pluginName,
		Factory: func(params map[string]any, deps plugin.Deps) (plugin.Plugin, error) {
			config, err := NewConfig(params)
			if err != nil {
				return nil, err
			}

			return New(config, deps)
		},
	}
}

// Plugin evaluates messages with a decisions model.
type Plugin struct {
	config        Config
	client        *http.Client
	endpoint      string
	questions     map[string]question
	questionsJSON string
	cache         *Storage
	logger        *zap.Logger
}

// New creates the plugin from a validated configuration and shared deps.
//
// deps.Cache is guaranteed by the wiring (one app-scoped cache per plugin name).
func New(config Config, deps plugin.Deps) (plugin.Plugin, error) {
	questions := buildQuestions(config.Questions)

	questionsJSON, err := json.Marshal(questions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal questions: %w", err)
	}

	return &Plugin{
		config:        config,
		client:        &http.Client{},
		endpoint:      strings.TrimRight(config.BaseURL, "/") + apiPath,
		questions:     questions,
		questionsJSON: string(questionsJSON),
		cache:         NewStorage(deps.Cache, config.CacheTTL),
		logger:        deps.Logger,
	}, nil
}

func (p *Plugin) Name() string {
	return pluginName
}

func (p *Plugin) Capabilities() plugin.Capability {
	return plugin.CapabilityText
}

func (p *Plugin) Priority() int {
	return defaultPriority
}

// Evaluate asks the fixed three-question rubric about the message and maps the
// answers to a plugin action.
func (p *Plugin) Evaluate(ctx context.Context, msg plugin.Message) (plugin.Result, error) {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	if text == "" {
		if msg.Media != nil {
			return skipResult("media-only message (not evaluated)"), nil
		}

		return skipResult("empty message"), nil
	}

	// Check cache first; a cache failure degrades to a miss and is logged,
	// never fails the evaluation.
	if p.config.CacheEnabled {
		cachedResp, found, cacheErr := p.cache.GetBy(ctx, text, p.config.Model, p.config.Context, p.questionsJSON)
		if cacheErr != nil {
			p.logger.Warn("failed to read decisions cache", zap.Error(cacheErr))
		}

		if found {
			result, err := p.evaluateResponse(cachedResp)
			if err != nil {
				return plugin.Result{}, err
			}
			result.Metadata["cached"] = true

			return result, nil
		}
	}

	// Cache miss - call API
	resp, err := p.ask(ctx, p.buildRequest(text))
	if err != nil {
		return plugin.Result{}, err
	}

	result, err := p.evaluateResponse(resp)
	if err != nil {
		return plugin.Result{}, err
	}

	if p.config.CacheEnabled {
		cacheErr := p.cache.SetBy(ctx, text, p.config.Model, p.config.Context, p.questionsJSON, resp)
		if cacheErr != nil {
			p.logger.Warn("failed to write decisions cache", zap.Error(cacheErr))
		}
	}
	result.Metadata["cached"] = false

	return result, nil
}

// Cleanup is a no-op: expired cache entries are reclaimed by the cachefx
// sweeper and evicted lazily on Get.
func (p *Plugin) Cleanup(context.Context) {
}

// buildRequest assembles the decisions request for one message.
func (p *Plugin) buildRequest(text string) decisionsRequest {
	return decisionsRequest{
		Model: p.config.Model,
		State: decisionsState{
			Context: p.config.Context,
			Message: stateMessage{Text: text},
		},
		Questions: p.questions,
	}
}

// evaluateResponse validates the answers and maps them to a plugin action.
func (p *Plugin) evaluateResponse(resp *Response) (plugin.Result, error) {
	category, err := answerOfType(resp.Answers, categoryQuestionID, questionTypeChoice)
	if err != nil {
		return plugin.Result{}, err
	}
	spam, err := answerOfType(resp.Answers, spamQuestionID, questionTypeNoul)
	if err != nil {
		return plugin.Result{}, err
	}
	abuse, err := answerOfType(resp.Answers, abuseQuestionID, questionTypeNoul)
	if err != nil {
		return plugin.Result{}, err
	}
	spamProb, err := noulValue(spamQuestionID, spam)
	if err != nil {
		return plugin.Result{}, err
	}
	abuseProb, err := noulValue(abuseQuestionID, abuse)
	if err != nil {
		return plugin.Result{}, err
	}

	if category.Choice == "" || len(category.Probabilities) == 0 {
		return plugin.Result{}, fmt.Errorf("%w: category answer carries no choice", ErrInvalidAnswer)
	}
	if !isRubricCategory(category.Choice) {
		return plugin.Result{}, fmt.Errorf(
			"%w: category choice %q is outside the rubric",
			ErrInvalidAnswer,
			category.Choice,
		)
	}
	if _, ok := category.Probabilities[category.Choice]; !ok {
		return plugin.Result{}, fmt.Errorf(
			"%w: category probabilities miss %q",
			ErrInvalidAnswer,
			category.Choice,
		)
	}
	for choice, value := range category.Probabilities {
		if err = checkProbability(categoryQuestionID+"."+choice, value); err != nil {
			return plugin.Result{}, err
		}
	}
	if err = checkProbability(spamQuestionID, spamProb); err != nil {
		return plugin.Result{}, err
	}
	if err = checkProbability(abuseQuestionID, abuseProb); err != nil {
		return plugin.Result{}, err
	}

	metadata := map[string]any{
		"category":               category.Choice,
		"category_probabilities": category.Probabilities,
		"spam_probability":       spamProb,
		"abuse_probability":      abuseProb,
		"model":                  resp.Model,
		"input_tokens":           resp.Usage.InputTokens,
		"output_tokens":          resp.Usage.OutputTokens,
	}

	categoryProb := category.Probabilities[category.Choice]
	reason, violated := p.firstViolation(category.Choice, categoryProb, spamProb, abuseProb)
	if violated {
		return plugin.Result{
			Action:   plugin.ActionBlock,
			Reason:   reason,
			Metadata: metadata,
			Plugin:   p.Name(),
		}, nil
	}

	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "message appears appropriate",
		Metadata: metadata,
		Plugin:   p.Name(),
	}, nil
}

// firstViolation returns the reason of the first rule that blocks, if any.
func (p *Plugin) firstViolation(category string, categoryProb float64, spam float64, abuse float64) (string, bool) {
	threshold := p.config.BlockThreshold

	if isViolationCategory(category) && categoryProb >= threshold {
		return fmt.Sprintf("%s (p=%.2f)", category, categoryProb), true
	}
	if spam >= threshold {
		return fmt.Sprintf("unsolicited advertising (p=%.2f)", spam), true
	}
	if abuse >= threshold {
		return fmt.Sprintf("personal attack (p=%.2f)", abuse), true
	}

	return "", false
}

// answerOfType returns the answer for id, checking it carries wantType.
func answerOfType(answers map[string]Answer, id string, wantType string) (Answer, error) {
	answer, ok := answers[id]
	if !ok {
		return Answer{}, fmt.Errorf("%w: missing %q", ErrInvalidAnswer, id)
	}
	if answer.Type != wantType {
		return Answer{}, fmt.Errorf(
			"%w: %q has type %q, want %q",
			ErrInvalidAnswer,
			id,
			answer.Type,
			wantType,
		)
	}

	return answer, nil
}

// noulValue returns the probability of a noul answer, rejecting a missing or
// null value while accepting a valid zero.
func noulValue(id string, answer Answer) (float64, error) {
	if answer.Noul == nil {
		return 0, fmt.Errorf("%w: %q carries no probability", ErrInvalidAnswer, id)
	}

	return *answer.Noul, nil
}

// checkProbability rejects values outside the 0.0 - 1.0 range.
func checkProbability(path string, value float64) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("%w: %s = %f", ErrInvalidProbability, path, value)
	}

	return nil
}

// isRubricCategory reports whether choice is one of the fixed rubric categories.
func isRubricCategory(choice string) bool {
	return choice == criterionSpam || choice == criterionScam ||
		choice == criterionAbuse || choice == criterionOther
}

// isViolationCategory reports whether a category answer is a blocking category.
func isViolationCategory(choice string) bool {
	return choice == criterionSpam || choice == criterionScam || choice == criterionAbuse
}

// skipResult builds a skip result with no opinion on the message.
func skipResult(reason string) plugin.Result {
	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   reason,
		Metadata: nil,
		Plugin:   pluginName,
	}
}

// buildQuestions converts the configured rubric text into the request payload.
func buildQuestions(cfg QuestionsConfig) map[string]question {
	return map[string]question{
		categoryQuestionID: {
			Type:         questionTypeChoice,
			Instructions: cfg.Category.Instructions,
			Criteria: map[string]string{
				criterionSpam:  cfg.Category.Criteria.Spam,
				criterionScam:  cfg.Category.Criteria.Scam,
				criterionAbuse: cfg.Category.Criteria.Abuse,
				criterionOther: cfg.Category.Criteria.Other,
			},
		},
		spamQuestionID: {
			Type:         questionTypeNoul,
			Instructions: cfg.Spam.Instructions,
			Criteria: map[string]string{
				criterionTrue:  cfg.Spam.Criteria.True,
				criterionFalse: cfg.Spam.Criteria.False,
			},
		},
		abuseQuestionID: {
			Type:         questionTypeNoul,
			Instructions: cfg.Abuse.Instructions,
			Criteria: map[string]string{
				criterionTrue:  cfg.Abuse.Criteria.True,
				criterionFalse: cfg.Abuse.Criteria.False,
			},
		},
	}
}
