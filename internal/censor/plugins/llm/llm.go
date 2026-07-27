package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/invopop/jsonschema"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Response struct {
	Inappropriate bool    `json:"inappropriate" description:"Whether the message is inappropriate" required:"true"`
	Confidence    float64 `json:"confidence"    description:"Confidence level of the response"     required:"true"`
	Reason        string  `json:"reason"        description:"Reason for the response"              required:"true"`
}

type Cache interface {
	SetBy(text, model, prompt string, resp *Response)
	GetBy(text, model, prompt string) (*Response, bool)
	Cleanup()
}

func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: "llm",
		Factory: func(params map[string]any) (plugin.Plugin, error) {
			config, err := NewConfig(params)
			if err != nil {
				return nil, err
			}

			return New(config)
		},
	}
}

type Plugin struct {
	config         Config
	client         openai.Client
	responseSchema map[string]any
	cache          Cache
}

func New(config Config) (plugin.Plugin, error) {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	schema := reflector.Reflect(new(Response))
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response schema: %w", err)
	}

	var responseSchema map[string]any
	if err = json.Unmarshal(schemaBytes, &responseSchema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response schema: %w", err)
	}

	baseURL := DefaultBaseURL
	if config.BaseURL != "" {
		baseURL = strings.TrimRight(config.BaseURL, "/")
	}

	client := openai.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(baseURL),
		option.WithHeader("HTTP-Referer", "https://t.me/NeoCensorBot"),
		option.WithHeader("X-Title", "NeoCensorBot"),
	)

	return &Plugin{
		config:         config,
		client:         client,
		responseSchema: responseSchema,
		cache:          NewStorage(config.CacheTTL, config.CacheMaxSize),
	}, nil
}

func (p *Plugin) Name() string {
	return "llm"
}

func (p *Plugin) Priority() int {
	const priority = 250
	return priority
}

func (p *Plugin) Evaluate(ctx context.Context, msg plugin.Message) (plugin.Result, error) {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	if text == "" {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "empty message",
			Metadata: nil,
			Plugin:   p.Name(),
		}, nil
	}

	// Prepare message for LLM analysis
	prompt := p.buildPrompt(text)

	// Check cache first
	if p.config.CacheEnabled {
		if cachedResp, found := p.cache.GetBy(text, p.config.Model, p.config.Prompt); found {
			result := p.evaluateResponse(cachedResp)
			result.Metadata["cached"] = true
			return result, nil
		}
	}

	// Cache miss - call API
	llmResponse, err := p.callLLMAPI(ctx, prompt)
	if err != nil {
		return plugin.Result{}, err
	}

	// Store in cache
	if p.config.CacheEnabled {
		p.cache.SetBy(text, p.config.Model, p.config.Prompt, llmResponse)
	}

	result := p.evaluateResponse(llmResponse)
	result.Metadata["cached"] = false

	return result, nil
}

func (p *Plugin) evaluateResponse(response *Response) plugin.Result {
	if response.Inappropriate && response.Confidence >= p.config.ConfidenceThreshold {
		return plugin.Result{
			Action: plugin.ActionBlock,
			Reason: response.Reason,
			Metadata: map[string]any{
				"confidence": response.Confidence,
			},
			Plugin: p.Name(),
		}
	}

	return plugin.Result{
		Action: plugin.ActionSkip,
		Reason: "message appears appropriate",
		Metadata: map[string]any{
			"confidence": response.Confidence,
		},
		Plugin: p.Name(),
	}
}

func (p *Plugin) callLLMAPI(ctx context.Context, prompt string) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	res, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: p.config.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "response",
					Schema: p.responseSchema,
					Strict: openai.Bool(true),
				},
			},
		},
		Temperature: openai.Float(p.config.Temperature),
	})
	if err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("failed to call LLM API (HTTP %d): %w", apiErr.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to call LLM API: %w", err)
	}

	if len(res.Choices) != 1 {
		return nil, fmt.Errorf("%w: expected 1, got %d", ErrUnexpectedResponseCount, len(res.Choices))
	}

	response := new(Response)

	if jsonErr := json.Unmarshal([]byte(res.Choices[0].Message.Content), response); jsonErr != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", jsonErr)
	}

	// Validate the response
	if response.Confidence < 0.0 || response.Confidence > 1.0 {
		return nil, fmt.Errorf("%w: %f (must be between 0.0 and 1.0)", ErrInvalidConfidence, response.Confidence)
	}

	return response, nil
}

func (p *Plugin) buildPrompt(text string) string {
	return fmt.Sprintf("%s\n\nMessage to analyze:\n%q", p.config.Prompt, text)
}

func (p *Plugin) Cleanup(_ context.Context) {
	p.cache.Cleanup()
}
