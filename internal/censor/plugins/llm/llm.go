package llm

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/invopop/jsonschema"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"go.uber.org/zap"
)

const (
	// mediaOnlyText is the text of a message that carries only media.
	mediaOnlyText = "(image only, no text)"
	// imageDetailAuto lets the provider choose the fidelity of the image.
	imageDetailAuto = "auto"
	// defaultMediaContentType is used when the media carries no content type.
	defaultMediaContentType = "image/jpeg"
)

type Response struct {
	Inappropriate bool    `json:"inappropriate" description:"Whether the message is inappropriate" required:"true"`
	Confidence    float64 `json:"confidence"    description:"Confidence level of the response"     required:"true"`
	Reason        string  `json:"reason"        description:"Reason for the response"              required:"true"`
}

// Marshal implements [cache.Item] so responses can be cached.
func (r *Response) Marshal() ([]byte, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return data, nil
}

// Unmarshal implements [cache.Item] so responses can be cached.
func (r *Response) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, r); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: "llm",
		Factory: func(params map[string]any, deps plugin.Deps) (plugin.Plugin, error) {
			config, err := NewConfig(params)
			if err != nil {
				return nil, err
			}

			return New(config, deps)
		},
	}
}

type Plugin struct {
	config         Config
	client         openai.Client
	responseSchema map[string]any
	baseURL        string
	cache          *Storage
	logger         *zap.Logger
}

// New creates the plugin from a validated configuration and shared deps.
//
// deps.Cache is guaranteed by the wiring (one app-scoped cache per plugin name).
func New(config Config, deps plugin.Deps) (plugin.Plugin, error) {
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
		baseURL:        baseURL,
		cache:          NewStorage(deps.Cache, config.CacheTTL),
		logger:         deps.Logger,
	}, nil
}

func (p *Plugin) Name() string {
	return "llm"
}

func (p *Plugin) Capabilities() plugin.Capability {
	return plugin.CapabilityText | plugin.CapabilityMedia
}

func (p *Plugin) Priority() int {
	const priority = 260
	return priority
}

func (p *Plugin) Evaluate(ctx context.Context, msg plugin.Message) (plugin.Result, error) {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	media := msg.Media
	if text == "" && media == nil {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "empty message",
			Metadata: nil,
			Plugin:   p.Name(),
		}, nil
	}

	if text == "" {
		text = mediaOnlyText
	}

	// Prepare message for LLM analysis
	prompt := p.buildPrompt(text)

	// The media identity is part of the cache key, so the same caption with a
	// different image is a different request.
	mediaHash := mediaDigest(media)

	// Check cache first; a cache failure degrades to a miss and is logged,
	// never fails the evaluation.
	if p.config.CacheEnabled {
		cachedResp, found, cacheErr := p.cache.GetBy(
			ctx, text, p.config.Model, p.config.Prompt, mediaHash, p.baseURL, p.config.Temperature,
		)
		if cacheErr != nil {
			p.logger.Warn("failed to read llm cache", zap.Error(cacheErr))
		}

		if found {
			result := p.evaluateResponse(cachedResp)
			result.Metadata["cached"] = true
			return result, nil
		}
	}

	// Cache miss - call API
	llmResponse, err := p.callLLMAPI(ctx, prompt, media)
	if err != nil {
		return plugin.Result{}, err
	}

	// Store in cache
	if p.config.CacheEnabled {
		cacheErr := p.cache.SetBy(
			ctx, text, p.config.Model, p.config.Prompt, mediaHash, p.baseURL, p.config.Temperature, llmResponse,
		)
		if cacheErr != nil {
			p.logger.Warn("failed to write llm cache", zap.Error(cacheErr))
		}
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

func (p *Plugin) callLLMAPI(ctx context.Context, prompt string, media *plugin.Media) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	var description param.Opt[string]

	res, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: p.config.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			buildUserMessage(prompt, media),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				Type: "json_schema",
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        "response",
					Description: description,
					Schema:      p.responseSchema,
					Strict:      openai.Bool(true),
				},
			},
		},
		Temperature: openai.Float(p.config.Temperature),
	})
	if err != nil {
		if apiErr, ok := errors.AsType[*openai.Error](err); ok {
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

// buildUserMessage builds the user message of the request.
//
// A message that carries media attaches the image as a data URL next to the
// prompt, so the model evaluates the image and not only the caption.
func buildUserMessage(prompt string, media *plugin.Media) openai.ChatCompletionMessageParamUnion {
	if media == nil || len(media.Bytes) == 0 {
		return openai.UserMessage(prompt)
	}

	return openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
		openai.TextContentPart(prompt),
		openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL:    mediaDataURL(media),
			Detail: imageDetailAuto,
		}),
	})
}

// mediaDataURL encodes the media content as a base64 data URL.
func mediaDataURL(media *plugin.Media) string {
	contentType := media.ContentType
	if contentType == "" {
		contentType = defaultMediaContentType
	}

	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(media.Bytes)
}

// mediaDigest returns the identity of the media content, or an empty string when
// the message carries no media.
func mediaDigest(media *plugin.Media) string {
	if media == nil {
		return ""
	}

	if media.SHA256 != "" {
		return media.SHA256
	}

	sum := sha256.Sum256(media.Bytes)

	return hex.EncodeToString(sum[:])
}

// Cleanup is a no-op: expired cache entries are reclaimed by the cachefx
// sweeper and evicted lazily on Get.
func (p *Plugin) Cleanup(context.Context) {
}
