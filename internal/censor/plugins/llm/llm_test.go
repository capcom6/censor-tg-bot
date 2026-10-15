package llm_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/llm"
	"github.com/stretchr/testify/require"
)

// completion renders a chat completion whose single choice carries the given JSON
// content, which is exactly the strict schema response format the plugin asks for.
func completion(content string) string {
	body, err := json.Marshal(map[string]any{
		"id":      "cmpl-test",
		"object":  "chat.completion",
		"created": 0,
		"model":   "test-model",
		"choices": []any{map[string]any{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
	})
	if err != nil {
		panic(err)
	}

	return string(body)
}

// verdict renders the plugin's Response schema.
func verdict(inappropriate bool, confidence float64, reason string) string {
	return completion(fmt.Sprintf(
		`{"inappropriate":%t,"confidence":%v,"reason":%q}`,
		inappropriate, confidence, reason,
	))
}

// upstream is a local stand-in for the provider. It counts every chat completion it
// receives and keeps the decoded bodies so a test can assert on the request shape.
type upstream struct {
	*httptest.Server

	mu      sync.Mutex
	bodies  []map[string]any
	content string
	status  int
}

// newUpstream starts a server answering every chat completion with content. A non-zero
// status makes it answer with that HTTP status instead.
func newUpstream(t *testing.T, content string, status int) *upstream {
	t.Helper()

	u := &upstream{content: content, status: status}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		u.mu.Lock()
		u.bodies = append(u.bodies, body)
		u.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if u.status != 0 {
			w.WriteHeader(u.status)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream is unwell","type":"server_error"}}`))

			return
		}

		_, _ = w.Write([]byte(u.content))
	}))
	t.Cleanup(u.Close)

	return u
}

// calls reports how many chat completions the server received.
func (u *upstream) calls() int {
	u.mu.Lock()
	defer u.mu.Unlock()

	return len(u.bodies)
}

// request returns the decoded body of the nth call.
func (u *upstream) request(t *testing.T, n int) map[string]any {
	t.Helper()

	u.mu.Lock()
	defer u.mu.Unlock()

	require.Greater(t, len(u.bodies), n, "no such request")

	return u.bodies[n]
}

// parts returns the content parts of the first message of the nth request.
func (u *upstream) parts(t *testing.T, n int) []map[string]any {
	t.Helper()

	messages, ok := u.request(t, n)["messages"].([]any)
	require.True(t, ok, "the request carries no messages")
	require.Len(t, messages, 1)

	message, ok := messages[0].(map[string]any)
	require.True(t, ok)

	parts, ok := message["content"].([]any)
	require.True(t, ok, "expected a content-part array")

	decoded := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		asMap, isMap := part.(map[string]any)
		require.True(t, isMap)

		decoded = append(decoded, asMap)
	}

	return decoded
}

// contentOf returns the raw content field of the first message of the nth request.
func (u *upstream) contentOf(t *testing.T, n int) any {
	t.Helper()

	messages, ok := u.request(t, n)["messages"].([]any)
	require.True(t, ok, "the request carries no messages")
	require.Len(t, messages, 1)

	message, ok := messages[0].(map[string]any)
	require.True(t, ok)

	return message["content"]
}

// mediaOf builds a plugin.Media carrying the given decoded bytes.
func mediaOf(size int) *plugin.Media {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte('a' + i%26)
	}

	return &plugin.Media{
		Bytes:       payload,
		ContentType: "image/jpeg",
		SizeBytes:   len(payload),
		SHA256:      strings.Repeat("a", 64),
	}
}

// photoOfSize builds a caption-less photo of exactly size decoded bytes.
func photoOfSize(size int) plugin.Message {
	return plugin.Message{Media: mediaOf(size), UserID: 1, ChatID: 2, MessageID: 3}
}

// distinctPhoto builds a caption-less photo with its own content hash, so two of them
// are genuinely different images to the cache.
func distinctPhoto(name string) plugin.Message {
	payload := []byte(name)

	sum := sha256.Sum256(payload)

	return plugin.Message{
		Media: &plugin.Media{
			Bytes:       payload,
			ContentType: "image/jpeg",
			SizeBytes:   len(payload),
			SHA256:      hex.EncodeToString(sum[:]),
		},
		UserID:    1,
		ChatID:    2,
		MessageID: 3,
	}
}

// TestPluginEvaluate covers the whole Evaluate contract: reachability, the request
// shape, the two caches, the local skips and the fail-closed error path.
func TestPluginEvaluate(t *testing.T) {
	// visionConfig is the base configuration of every case: image moderation on and a
	// vision model chosen. textConfig turns only the image half off.
	visionConfig := func() llm.Config {
		cfg := llm.DefaultConfig()
		cfg.APIKey = "test-key"
		cfg.ImageEnabled = true
		cfg.VisionModel = "vision/test-model"

		return cfg
	}

	// blocking is a response that blocks at high confidence.
	blocking := verdict(true, 0.95, "explicit content")

	// textMessage builds an ordinary text message.
	textMessage := func(text string) plugin.Message {
		return plugin.Message{Text: text, UserID: 1, ChatID: 2, MessageID: 3}
	}

	// captionedPhoto builds a photo with a caption.
	captionedPhoto := func(caption string) plugin.Message {
		return plugin.Message{Caption: caption, Media: mediaOf(64), UserID: 1, ChatID: 2, MessageID: 3}
	}

	// captionlessPhoto builds a photo with no caption at all: the most common photo
	// case, and the one the removed early return used to kill.
	captionlessPhoto := func() plugin.Message {
		return plugin.Message{Media: mediaOf(64), UserID: 1, ChatID: 2, MessageID: 3}
	}

	// newPlugin builds the plugin against the local upstream.
	newPlugin := func(t *testing.T, cfg llm.Config) plugin.Plugin {
		t.Helper()

		p, err := llm.New(cfg)
		require.NoError(t, err)

		return p
	}

	t.Run("a caption-less photo reaches the vision branch exactly once", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 0.95, "explicit content"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), captionlessPhoto())

		require.NoError(t, err)
		require.Equal(t, 1, up.calls(), "exactly one vision request, no early return in the way")
		require.Equal(t, plugin.ActionBlock, result.Action)
		require.Equal(t, "explicit content", result.Reason)
		require.Equal(t, "image", result.Metadata["modality"])
		require.Equal(t, false, result.Metadata["cached"])
	})

	t.Run("a genuinely empty message still skips with the same reason", func(t *testing.T) {
		up := newUpstream(t, blocking, 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), plugin.Message{UserID: 1, ChatID: 2, MessageID: 3})

		require.NoError(t, err)
		require.Equal(t, plugin.ActionSkip, result.Action)
		require.Equal(t, "empty message", result.Reason, "the relocated skip keeps its reason")
		require.Zero(t, up.calls(), "there is nothing to send")
	})

	t.Run("an empty message with no media and no image moderation also skips", func(t *testing.T) {
		up := newUpstream(t, blocking, 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		cfg.ImageEnabled = false
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), plugin.Message{UserID: 1, ChatID: 2, MessageID: 3})

		require.NoError(t, err)
		require.Equal(t, plugin.ActionSkip, result.Action)
		require.Equal(t, "empty message", result.Reason)
		require.Zero(t, up.calls())
	})

	t.Run("the image branch is unreachable when image moderation is off", func(t *testing.T) {
		up := newUpstream(t, blocking, 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		// FX-05: another plugin may want the bytes - the duplicate plugin does - while
		// this feature is nominally off. No provider call may happen.
		cfg.ImageEnabled = false
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), captionlessPhoto())

		require.NoError(t, err)
		require.Zero(t, up.calls(), "photo moderation is off, so nothing leaves the host")
		require.Equal(t, plugin.ActionSkip, result.Action)
	})

	t.Run("the request is a data URL sent after the prompt with low detail", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 0.95, "explicit content"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		message := captionlessPhoto()
		_, err := p.Evaluate(context.Background(), message)

		require.NoError(t, err)
		require.Equal(t, 1, up.calls())
		require.Equal(t, cfg.VisionModel, up.request(t, 0)["model"])

		parts := up.parts(t, 0)
		require.Len(t, parts, 2)
		require.Equal(t, "text", parts[0]["type"], "the provider recommends the prompt first")
		require.Contains(t, parts[0]["text"], "photo")
		require.Equal(t, "image_url", parts[1]["type"])

		imageURL, ok := parts[1]["image_url"].(map[string]any)
		require.True(t, ok)
		url, isString := imageURL["url"].(string)
		require.True(t, isString)
		require.True(t, strings.HasPrefix(url, "data:image/jpeg;base64,"), "inline data URL, got %.40q", url)
		require.Equal(t, "low", imageURL["detail"])
	})

	t.Run("a text block is returned without calling the vision API", func(t *testing.T) {
		up := newUpstream(t, blocking, 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), captionedPhoto("buy now"))

		require.NoError(t, err)
		require.Equal(t, 1, up.calls(), "the cheap verdict is enough, so no photo is shipped")
		require.Equal(t, plugin.ActionBlock, result.Action)
		require.Equal(t, "text", result.Metadata["modality"])
	})

	t.Run("a text pass lets the image branch run", func(t *testing.T) {
		up := newUpstream(t, verdict(false, 0.1, "fine"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		// The response is benign, so the text branch does not block and the
		// second request must be the vision one.
		result, err := p.Evaluate(context.Background(), captionedPhoto("buy now"))

		require.NoError(t, err)
		require.Equal(t, 2, up.calls())
		require.Equal(t, plugin.ActionSkip, result.Action)
		require.Equal(t, "image", result.Metadata["modality"])
	})

	t.Run("an image above the local inline budget is skipped before any call", func(t *testing.T) {
		up := newUpstream(t, blocking, 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		cfg.MaxInlineImageBytes = 1024
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), photoOfSize(4096))

		require.NoError(t, err)
		require.Zero(t, up.calls(), "an oversized photo is never shipped")
		require.Equal(t, plugin.ActionSkip, result.Action)
		require.Equal(t, "image exceeds the local inline budget", result.Reason)
		require.Equal(t, "image", result.Metadata["modality"])
	})

	t.Run("a zero inline budget applies no limit", func(t *testing.T) {
		up := newUpstream(t, blocking, 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		require.Zero(t, llm.DefaultConfig().MaxInlineImageBytes, "the shipped default applies no limit")

		p := newPlugin(t, cfg)

		_, err := p.Evaluate(context.Background(), captionlessPhoto())

		require.NoError(t, err)
		require.Equal(t, 1, up.calls())
	})

	t.Run("an image inside the local inline budget is still examined", func(t *testing.T) {
		up := newUpstream(t, blocking, 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		cfg.MaxInlineImageBytes = 4096
		p := newPlugin(t, cfg)

		_, err := p.Evaluate(context.Background(), photoOfSize(1024))

		require.NoError(t, err)
		require.Equal(t, 1, up.calls())
	})

	t.Run("an image that cannot finish in the remaining budget is skipped", func(t *testing.T) {
		up := newUpstream(t, blocking, 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		// The service wraps every plugin in censor.Timeout; here the parent deadline is
		// deliberately almost spent. Starting a request that cannot finish would block
		// the message under the fail-closed rule.
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(500*time.Millisecond))
		defer cancel()

		result, err := p.Evaluate(ctx, captionlessPhoto())

		require.NoError(t, err)
		require.Zero(t, up.calls())
		require.Equal(t, plugin.ActionSkip, result.Action)
		require.Equal(t, "image exceeds the remaining moderation budget", result.Reason)
	})

	t.Run("an upstream failure propagates as an error with an empty result", func(t *testing.T) {
		up := newUpstream(t, "", http.StatusInternalServerError)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), captionlessPhoto())

		require.Error(t, err, "UR2: a vendor failure is never converted into a skip")
		require.Equal(t, plugin.Result{}, result, "no decision is invented from a failure")
	})

	t.Run("a malformed response propagates as an error", func(t *testing.T) {
		up := newUpstream(t, completion("this is not json"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), captionlessPhoto())

		require.Error(t, err)
		require.Equal(t, plugin.Result{}, result)
	})

	t.Run("an out-of-range confidence propagates as an error", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 1.7, "impossible"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), captionlessPhoto())

		require.Error(t, err)
		require.ErrorIs(t, err, llm.ErrInvalidConfidence)
		require.Equal(t, plugin.Result{}, result)
	})

	t.Run("a cached photo is answered without a second call", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 0.95, "explicit content"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		first, err := p.Evaluate(context.Background(), captionlessPhoto())
		require.NoError(t, err)
		require.Equal(t, false, first.Metadata["cached"])

		second, err := p.Evaluate(context.Background(), captionlessPhoto())

		require.NoError(t, err)
		require.Equal(t, 1, up.calls(), "the image cache serves the repeat")
		require.Equal(t, true, second.Metadata["cached"])
		require.Equal(t, plugin.ActionBlock, second.Action)
		require.Equal(t, "image", second.Metadata["modality"])
	})

	t.Run("a cached caption is answered without a second call", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 0.95, "explicit content"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		for range 2 {
			_, err := p.Evaluate(context.Background(), textMessage("buy now"))
			require.NoError(t, err)
		}

		require.Equal(t, 1, up.calls())
	})

	t.Run("photo traffic does not evict cached text", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 0.95, "explicit content"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		// One entry per cache: a shared LRU would make the two photos below evict the
		// caption, and the repeat would have to go upstream again.
		cfg.CacheMaxSize = 1
		p := newPlugin(t, cfg)

		_, err := p.Evaluate(context.Background(), textMessage("buy now"))
		require.NoError(t, err)
		require.Equal(t, 1, up.calls())

		repeat, err := p.Evaluate(context.Background(), distinctPhoto("first"))
		require.NoError(t, err)
		require.Equal(t, 2, up.calls())
		require.Equal(t, "image", repeat.Metadata["modality"])

		for _, name := range []string{"second", "third"} {
			_, evalErr := p.Evaluate(context.Background(), distinctPhoto(name))
			require.NoError(t, evalErr)
		}
		require.Equal(t, 4, up.calls())

		cached, err := p.Evaluate(context.Background(), textMessage("buy now"))

		require.NoError(t, err)
		require.Equal(t, 4, up.calls(), "the caption is still cached after two photos")
		require.Equal(t, true, cached.Metadata["cached"])
		require.Equal(t, "text", cached.Metadata["modality"])
	})

	t.Run("a text-only message never touches the image half", func(t *testing.T) {
		up := newUpstream(t, verdict(false, 0.05, "fine"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), textMessage("hello"))

		require.NoError(t, err)
		require.Equal(t, 1, up.calls())
		require.IsType(t, "", up.contentOf(t, 0), "the text branch sends a plain prompt, not content parts")
		require.Equal(t, "text", result.Metadata["modality"])
	})

	t.Run("cleanup leaves both modalities intact and expired entries stop being served", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 0.95, "explicit content"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		// llm.New does not re-validate, so the TTL is set directly to keep the test
		// fast. NewConfig would refuse anything under MinCacheTTL.
		cfg.CacheTTL = time.Millisecond
		p := newPlugin(t, cfg)

		_, err := p.Evaluate(context.Background(), textMessage("buy now"))
		require.NoError(t, err)
		_, err = p.Evaluate(context.Background(), distinctPhoto("one"))
		require.NoError(t, err)
		require.Equal(t, 2, up.calls())

		time.Sleep(20 * time.Millisecond)

		p.Cleanup(context.Background())

		_, err = p.Evaluate(context.Background(), textMessage("buy now"))
		require.NoError(t, err)
		_, err = p.Evaluate(context.Background(), distinctPhoto("one"))
		require.NoError(t, err)

		require.Equal(t, 4, up.calls(), "an expired entry is no longer served, for either modality")
	})

	t.Run("cleanup does not discard unexpired entries of either modality", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 0.95, "explicit content"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		// Set directly on the config, bypassing NewConfig, so the test stays fast; a
		// NewConfig-built deployment would refuse anything under MinCacheTTL.
		cfg.CacheTTL = time.Minute
		p := newPlugin(t, cfg)

		_, err := p.Evaluate(context.Background(), textMessage("buy now"))
		require.NoError(t, err)
		_, err = p.Evaluate(context.Background(), distinctPhoto("one"))
		require.NoError(t, err)
		require.Equal(t, 2, up.calls())

		p.Cleanup(context.Background())

		text, err := p.Evaluate(context.Background(), textMessage("buy now"))
		require.NoError(t, err)
		image, err := p.Evaluate(context.Background(), distinctPhoto("one"))

		require.NoError(t, err)
		require.Equal(t, 2, up.calls(), "cleanup is not a reset: live entries of both caches survive")
		require.Equal(t, true, text.Metadata["cached"])
		require.Equal(t, true, image.Metadata["cached"])
		require.Equal(t, "text", text.Metadata["modality"])
		require.Equal(t, "image", image.Metadata["modality"])
	})

	t.Run("needs media only while image moderation is on", func(t *testing.T) {
		on := llm.DefaultConfig()
		on.ImageEnabled = true
		onPlugin, err := llm.New(on)
		require.NoError(t, err)

		consumer, ok := onPlugin.(plugin.MediaConsumer)
		require.True(t, ok, "the plugin must be able to ask for media")
		require.True(t, consumer.NeedsMedia(plugin.Message{}))

		offPlugin, err := llm.New(llm.DefaultConfig())
		require.NoError(t, err)

		offConsumer, isConsumer := offPlugin.(plugin.MediaConsumer)
		require.True(t, isConsumer)
		require.False(t, offConsumer.NeedsMedia(plugin.Message{}))
	})

	t.Run("no reason carries a modality prefix", func(t *testing.T) {
		up := newUpstream(t, verdict(true, 0.95, "explicit content"), 0)
		cfg := visionConfig()
		cfg.BaseURL = up.URL
		p := newPlugin(t, cfg)

		result, err := p.Evaluate(context.Background(), captionlessPhoto())

		require.NoError(t, err)
		require.NotContains(t, result.Reason, "image:")
		require.NotContains(t, result.Reason, "text:")
		require.Equal(t, "explicit content", result.Reason, "the model reason reaches admins verbatim")
	})
}
