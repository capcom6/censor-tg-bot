package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/invopop/jsonschema"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// minImageBudget is the smallest amount of the parent deadline an image request must
// still have. The request runs under censor.Timeout and the update pump is a single
// serial loop, so a request that cannot finish inside the remaining budget would time
// out - and under the fail-closed rule a timeout blocks the message. Skipping is the
// only outcome that does not punish a photo for the server being slow.
const minImageBudget = 2 * time.Second

const (
	// imageDetail is the detail level sent with every photo. "low" is inside the enum
	// openai-go declares (auto, low, high) and keeps the request small, which is the
	// right trade for a yes/no moderation verdict.
	imageDetail = "low"

	// The Result.Metadata keys.
	metadataKeyModality   = "modality"
	metadataKeyCached     = "cached"
	metadataKeyConfidence = "confidence"

	// The two modality values. This is the ONLY place the modality of a decision is
	// recorded: the reason is never prefixed (N8), so one mechanism serves both
	// plugins.
	modalityText  = "text"
	modalityImage = "image"

	// imageCacheKeyPrefix namespaces image entries. The generated cache key already
	// hashes the components together, but the prefix makes the key readable in a
	// dump and keeps the two modalities obviously distinct.
	imageCacheKeyPrefix = "image:"

	// dataURLPrefix and dataURLSeparator build the inline data URL.
	dataURLPrefix    = "data:"
	dataURLSeparator = ";base64,"

	// The two LOCAL reasons the image branch skips before any provider call. Every
	// other outcome is either a decision or an error.
	imageSkipBudgetReason = "image exceeds the remaining moderation budget"
	imageSkipSizeReason   = "image exceeds the local inline budget"
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

	// textCache and imageCache are separate storages on purpose. An image key is
	// unique per photo while a text key repeats, so one shared LRU would let photo
	// traffic evict exactly the text entries that keep the text path cheap and cause
	// MORE upstream calls, not fewer. The cost of two caches is that
	// cache_max_size now applies per modality.
	textCache  Cache
	imageCache Cache
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
		textCache:      NewStorage(config.CacheTTL, config.CacheMaxSize),
		imageCache:     NewStorage(config.CacheTTL, config.CacheMaxSize),
	}, nil
}

func (p *Plugin) Name() string {
	return "llm"
}

func (p *Plugin) Priority() int {
	const priority = 250
	return priority
}

// NeedsMedia reports whether the plugin needs the photo bytes of a message.
//
// It is configuration-only on purpose: the service asks before any media exists, so
// there is nothing about the message to decide on yet. A plugin turned off here never
// causes a Telegram download, which is what keeps image moderation from becoming a
// third-party photo upload the operator did not ask for.
func (p *Plugin) NeedsMedia(_ plugin.Message) bool {
	return p.config.ImageEnabled
}

// Evaluate decides what happens to a message, looking at its text, its caption and -
// when image moderation is enabled - its photo.
//
// N1: the empty-message Skip is RELOCATED, not deleted. It used to run before every
// other check, so every CAPTION-LESS PHOTO skipped vision moderation entirely: the
// most common photo case, and a silent no-op for the whole feature. The Skip now
// applies only when there is neither text nor an image to examine, which leaves a
// genuinely empty message exactly as it was. For every message with non-empty text
// the decision is unchanged; the one behavior change is that a message with neither
// text nor caption now reaches the image branch.
func (p *Plugin) Evaluate(ctx context.Context, msg plugin.Message) (plugin.Result, error) {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	if text != "" {
		result, err := p.evaluateText(ctx, text)
		if err != nil {
			return plugin.Result{}, err
		}

		// Cheapest first: a text block is the most specific reason available, and
		// spending a vision call on an already-blocked message would cost provider
		// money and ship somebody's photo for nothing.
		if result.Action == plugin.ActionBlock || !p.examinesImage(msg) {
			return result, nil
		}

		return p.evaluateImage(ctx, msg.Media)
	}

	if !p.examinesImage(msg) {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "empty message",
			Metadata: nil,
			Plugin:   p.Name(),
		}, nil
	}

	return p.evaluateImage(ctx, msg.Media)
}

// examinesImage is the gate on the image branch: it runs only when the operator turned
// image moderation on AND the message really carries downloaded bytes.
//
// Without the ImageEnabled check, enabling images in the duplicate plugin alone would
// still send every photo to the provider with the feature nominally off, which is
// provider spend and a third-party photo upload nobody asked for.
func (p *Plugin) examinesImage(msg plugin.Message) bool {
	return p.config.ImageEnabled && msg.Media != nil
}

// evaluateText judges the message text or caption with the configured text model.
func (p *Plugin) evaluateText(ctx context.Context, text string) (plugin.Result, error) {
	prompt := p.buildPrompt(text)

	return p.evaluateCached(ctx, p.textCache, modalityText, text, p.config.Model, prompt,
		func(callCtx context.Context) (*Response, error) {
			return p.callLLMAPI(callCtx, prompt)
		})
}

// evaluateImage judges one downloaded photo with the configured vision model. media is
// non-nil: examinesImage guarantees it.
//
// Every skip here is LOCAL and happens before any request is issued. Every vendor
// failure - transport error, upstream HTTP status, malformed JSON, out-of-range
// confidence, an expired deadline - leaves as a non-nil error with an empty Result,
// exactly like the text branch. There is no blanket catch, no degradation allowlist
// and no ActionSkip on an API failure.
func (p *Plugin) evaluateImage(ctx context.Context, media *plugin.Media) (plugin.Result, error) {
	if skipped, isSkipped := p.localImageSkip(ctx, media); isSkipped {
		return skipped, nil
	}

	dataURL := imageDataURL(media)
	key := imageCacheKeyPrefix + media.SHA256

	return p.evaluateCached(ctx, p.imageCache, modalityImage, key, p.config.VisionModel, p.config.ImagePrompt,
		func(callCtx context.Context) (*Response, error) {
			return p.callVisionAPI(callCtx, dataURL)
		})
}

// evaluateCached is the ONE cache-and-decide sequence both branches run: read the
// cache, issue the call on a miss through the branch's own closure, store the response
// and turn it into a decision.
//
// This is a ruling, not lint insurance. Both modalities must share one error semantic,
// so the cache handling, the "cached" metadata flag and the threshold decision must
// not be free to drift apart: a bug fixed in one branch has to be fixed in both.
func (p *Plugin) evaluateCached(
	ctx context.Context,
	cache Cache,
	modality string,
	key string,
	model string,
	prompt string,
	call func(context.Context) (*Response, error),
) (plugin.Result, error) {
	if p.config.CacheEnabled {
		if cachedResp, found := cache.GetBy(key, model, prompt); found {
			result := p.evaluateResponse(cachedResp, modality)
			result.Metadata[metadataKeyCached] = true

			return result, nil
		}
	}

	// Fail-closed: any vendor failure leaves here as an empty Result.
	response, err := call(ctx)
	if err != nil {
		return plugin.Result{}, err
	}

	if p.config.CacheEnabled {
		cache.SetBy(key, model, prompt, response)
	}

	result := p.evaluateResponse(response, modality)
	result.Metadata[metadataKeyCached] = false

	return result, nil
}

// localImageSkip reports whether this photo must be skipped without asking the
// provider, returning the skip result when it must. Both conditions are local policy,
// decided before a request exists.
func (p *Plugin) localImageSkip(ctx context.Context, media *plugin.Media) (plugin.Result, bool) {
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline && time.Until(deadline) < minImageBudget {
		return p.skipResult(imageSkipBudgetReason), true
	}

	// A zero budget means the guard does not run and bot.media.max_bytes is the only
	// bound. See Config.MaxInlineImageBytes for why there is no default and why no
	// value here is compared against bot.media.max_bytes.
	if p.config.MaxInlineImageBytes > 0 && int64(media.SizeBytes) > p.config.MaxInlineImageBytes {
		return p.skipResult(imageSkipSizeReason), true
	}

	var undecided plugin.Result

	return undecided, false
}

// skipResult builds the Skip both local guards return. It carries the modality so the
// decision stays identifiable.
func (p *Plugin) skipResult(reason string) plugin.Result {
	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   reason,
		Metadata: map[string]any{metadataKeyModality: modalityImage},
		Plugin:   p.Name(),
	}
}

func (p *Plugin) evaluateResponse(response *Response, modality string) plugin.Result {
	if response.Inappropriate && response.Confidence >= p.config.ConfidenceThreshold {
		return plugin.Result{
			Action: plugin.ActionBlock,
			Reason: response.Reason,
			Metadata: map[string]any{
				metadataKeyModality:   modality,
				metadataKeyConfidence: response.Confidence,
			},
			Plugin: p.Name(),
		}
	}

	return plugin.Result{
		Action: plugin.ActionSkip,
		Reason: "message appears appropriate",
		Metadata: map[string]any{
			metadataKeyModality:   modality,
			metadataKeyConfidence: response.Confidence,
		},
		Plugin: p.Name(),
	}
}

func (p *Plugin) callLLMAPI(ctx context.Context, prompt string) (*Response, error) {
	return p.complete(ctx, openai.ChatCompletionNewParams{
		Model: p.config.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		ResponseFormat: p.responseFormat(),
		Temperature:    openai.Float(p.config.Temperature),
	})
}

// callVisionAPI sends the photo to the configured vision model as an inline data URL.
//
// The text part comes FIRST because that is the order the provider recommends: its
// content parser is documented to want the prompt before the images.
func (p *Plugin) callVisionAPI(ctx context.Context, dataURL string) (*Response, error) {
	return p.complete(ctx, openai.ChatCompletionNewParams{
		Model: p.config.VisionModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
				openai.TextContentPart(p.config.ImagePrompt),
				openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL:    dataURL,
					Detail: imageDetail,
				}),
			}),
		},
		ResponseFormat: p.responseFormat(),
		Temperature:    openai.Float(p.config.Temperature),
	})
}

// complete issues one chat completion and turns it into a Response.
//
// Both branches build their own params and hand them here, so the timeout, the error
// mapping and every response validation are literally the same code on both paths.
// That is what makes the fail-closed guarantee hold on the image path by construction
// rather than by remembering to copy it.
func (p *Plugin) complete(ctx context.Context, params openai.ChatCompletionNewParams) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	res, err := p.client.Chat.Completions.New(ctx, params)
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

// responseFormat is the strict JSON-schema response format, built in New from the one
// reflected schema and shared by both modalities.
func (p *Plugin) responseFormat() openai.ChatCompletionNewParamsResponseFormatUnion {
	return openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
			JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "response",
				Schema: p.responseSchema,
				Strict: openai.Bool(true),
			},
		},
	}
}

// imageDataURL builds the inline data URL: "data:<mime>;base64,<payload>".
//
// [base64.StdEncoding] inflates the payload by 4/3, which is why the local budget in
// Config.MaxInlineImageBytes is measured in DECODED bytes rather than in the bytes
// that actually travel.
func imageDataURL(media *plugin.Media) string {
	return dataURLPrefix + media.ContentType + dataURLSeparator + base64.StdEncoding.EncodeToString(media.Bytes)
}

func (p *Plugin) buildPrompt(text string) string {
	return fmt.Sprintf("%s\n\nMessage to analyze:\n%q", p.config.Prompt, text)
}

// Cleanup expires both caches. The service drives it from a ticker, so leaving one of
// the two alone would leak its entries until the process ends.
func (p *Plugin) Cleanup(_ context.Context) {
	p.textCache.Cleanup()
	p.imageCache.Cleanup()
}
