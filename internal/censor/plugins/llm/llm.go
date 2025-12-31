package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/revrost/go-openrouter"
	"github.com/revrost/go-openrouter/jsonschema"
)

type response struct {
	Inappropriate bool    `json:"inappropriate" description:"Whether the message is inappropriate" required:"true"`
	Confidence    float64 `json:"confidence"    description:"Confidence level of the response"     required:"true"`
	Reason        string  `json:"reason"        description:"Reason for the response"              required:"true"`
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
	client         *openrouter.Client
	responseSchema *jsonschema.Definition
}

func New(config Config) (plugin.Plugin, error) {
	responseSchema, err := jsonschema.GenerateSchemaForType(new(response))
	if err != nil {
		return nil, fmt.Errorf("failed to generate response schema: %w", err)
	}

	return &Plugin{
		config: config,
		client: openrouter.NewClient(
			config.APIKey,
			openrouter.WithXTitle("NeoCensorBot"),
			openrouter.WithHTTPReferer("https://t.me/NeoCensorBot"),
		),
		responseSchema: responseSchema,
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

	// Call LLM API
	llmResponse, err := p.callLLMAPI(ctx, prompt)
	if err != nil {
		return plugin.Result{}, err
	}

	// Evaluate response and determine action
	result := p.evaluateResponse(llmResponse)

	return result, nil
}

func (p *Plugin) evaluateResponse(response *response) plugin.Result {
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

func (p *Plugin) callLLMAPI(ctx context.Context, prompt string) (*response, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	request := openrouter.ChatCompletionRequest{
		Model: p.config.Model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.UserMessage(prompt),
		},
		ResponseFormat: &openrouter.ChatCompletionResponseFormat{
			Type: openrouter.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openrouter.ChatCompletionResponseFormatJSONSchema{
				Name:        "response",
				Schema:      p.responseSchema,
				Strict:      true,
				Description: "",
			},
		},
		Temperature: float32(p.config.Temperature),
	}

	res, err := p.client.CreateChatCompletion(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM API: %w", err)
	}

	if len(res.Choices) != 1 {
		return nil, fmt.Errorf("%w: expected 1, got %d", ErrUnexpectedResponseCount, len(res.Choices))
	}

	response := new(response)

	if jsonErr := json.Unmarshal([]byte(res.Choices[0].Message.Content.Text), response); jsonErr != nil {
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

// Cleanup implements plugin.Plugin.
func (p *Plugin) Cleanup(_ context.Context) {
	// no-op
}
