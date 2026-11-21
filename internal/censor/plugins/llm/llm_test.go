package llm_test

import (
	"context"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/llm"
	"github.com/go-core-fx/cachefx/cache"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// completionResponse is the answer of the fake provider.
const completionResponse = `{
	"id": "1",
	"object": "chat.completion",
	"created": 0,
	"model": "test-model",
	"choices": [
		{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "{\"inappropriate\": false, \"confidence\": 0.1, \"reason\": \"fine\"}"
			},
			"finish_reason": "stop"
		}
	]
}`

// requestRecorder collects the bodies of the requests sent to the fake provider.
type requestRecorder struct {
	mu       sync.Mutex
	requests []string
}

func (r *requestRecorder) add(body string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests = append(r.requests, body)
}

func (r *requestRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.requests)
}

// newFakeProvider starts a provider that answers every request with the same
// completion and records the request bodies.
func newFakeProvider(t *testing.T) (*httptest.Server, *requestRecorder) {
	t.Helper()

	recorder := &requestRecorder{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)

			return
		}

		recorder.add(string(body))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(completionResponse))
	}))
	t.Cleanup(server.Close)

	return server, recorder
}

func newPlugin(t *testing.T, baseURL string) plugin.Plugin {
	t.Helper()

	return newPluginWithDeps(t, baseURL, newDeps())
}

// newPluginWithDeps builds a plugin pointed at the fake provider over the
// given deps, so tests can share one cache between instances.
func newPluginWithDeps(t *testing.T, baseURL string, deps plugin.Deps) plugin.Plugin {
	t.Helper()

	return newPluginWithConfig(t, baseURL, deps, nil)
}

// newPluginWithConfig builds a plugin with config overrides applied on top of
// the test defaults, allowing tests to vary one config value at a time.
func newPluginWithConfig(t *testing.T, baseURL string, deps plugin.Deps, overrides map[string]any) plugin.Plugin {
	t.Helper()

	configMap := map[string]any{
		"base_url":             baseURL,
		"api_key":              "test-key",
		"model":                "test-model",
		"prompt":               "Analyze the message",
		"confidence_threshold": 0.8,
		"cache_enabled":        true,
	}
	maps.Copy(configMap, overrides)

	config, err := llm.NewConfig(configMap)
	require.NoError(t, err)

	p, err := llm.New(config, deps)
	require.NoError(t, err)

	return p
}

// newDeps builds plugin deps backed by a real in-memory cache.
func newDeps() plugin.Deps {
	return plugin.Deps{
		Cache:  cache.NewMemory(time.Hour),
		Logger: zap.NewNop(),
	}
}

func testMedia(digest string) *plugin.Media {
	return &plugin.Media{
		Bytes:       []byte("image content"),
		ContentType: "image/jpeg",
		SizeBytes:   len("image content"),
		SHA256:      digest,
	}
}

func TestPlugin_EvaluateAttachesMedia(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t)
	p := newPlugin(t, server.URL)

	result, err := p.Evaluate(context.Background(), plugin.Message{
		Caption: "a caption",
		ChatID:  1,
		Media:   testMedia("d58b3fa7"),
	})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)

	requests := recorder.all()
	require.Len(t, requests, 1)
	require.Contains(t, requests[0], `"type":"image_url"`)
	require.Contains(t, requests[0], "data:image/jpeg;base64,")
	require.Contains(t, requests[0], `"type":"text"`)
}

func TestPlugin_EvaluateMediaWithoutCaption(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t)
	p := newPlugin(t, server.URL)

	result, err := p.Evaluate(context.Background(), plugin.Message{
		ChatID: 1,
		Media:  testMedia("d58b3fa7"),
	})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)

	require.Len(t, recorder.all(), 1)
}

func TestPlugin_EvaluateTextOnlyHasNoImage(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t)
	p := newPlugin(t, server.URL)

	result, err := p.Evaluate(context.Background(), plugin.Message{
		Text:   "a text",
		ChatID: 1,
	})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)

	requests := recorder.all()
	require.Len(t, requests, 1)
	require.NotContains(t, requests[0], "image_url")
}

func TestPlugin_EvaluateCachesPerMedia(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t)
	p := newPlugin(t, server.URL)

	message := plugin.Message{
		Caption: "a caption",
		ChatID:  1,
		Media:   testMedia("d58b3fa7"),
	}

	first, err := p.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, first.Metadata["cached"])

	second, err := p.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, true, second.Metadata["cached"])

	// The same caption with a different image is a different request.
	other := message
	other.Media = testMedia("95a0db0d")

	third, err := p.Evaluate(context.Background(), other)
	require.NoError(t, err)
	require.Equal(t, false, third.Metadata["cached"])

	require.Len(t, recorder.all(), 2)
}

func TestPlugin_EvaluateCachesPerEndpoint(t *testing.T) {
	t.Parallel()

	serverA, recorderA := newFakeProvider(t)
	serverB, recorderB := newFakeProvider(t)

	// Both plugins share one cache, so a hit proves the endpoint is part of
	// the key on both the read and the write path.
	deps := newDeps()
	pA := newPluginWithDeps(t, serverA.URL, deps)
	pB := newPluginWithDeps(t, serverB.URL, deps)

	message := plugin.Message{Text: "hello", ChatID: 1}

	first, err := pA.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, first.Metadata["cached"])

	second, err := pA.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, true, second.Metadata["cached"])

	// The same text against a different endpoint must not hit the cache.
	third, err := pB.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, third.Metadata["cached"])

	require.Len(t, recorderA.all(), 1)
	require.Len(t, recorderB.all(), 1)
}

func TestPlugin_EvaluateCachesPerTemperature(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t)

	// Both plugins share one cache, so a hit proves temperature is part of
	// the key on both the read and the write path.
	deps := newDeps()
	low := newPluginWithConfig(t, server.URL, deps, map[string]any{"temperature": 0.1})
	high := newPluginWithConfig(t, server.URL, deps, map[string]any{"temperature": 1.0})

	message := plugin.Message{Text: "hello", ChatID: 1}

	first, err := low.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, first.Metadata["cached"])

	second, err := low.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, true, second.Metadata["cached"])

	// The same text at a different temperature must not hit the cache.
	third, err := high.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, third.Metadata["cached"])

	fourth, err := high.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, true, fourth.Metadata["cached"])

	requests := recorder.all()
	require.Len(t, requests, 2)
	require.Contains(t, requests[0], `"temperature":0.1`)
	require.Contains(t, requests[1], `"temperature":1`)
}

func TestPlugin_EvaluateSkipsEmptyMessage(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t)
	p := newPlugin(t, server.URL)

	result, err := p.Evaluate(context.Background(), plugin.Message{ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)
	require.Contains(t, result.Reason, "empty message")

	require.Empty(t, recorder.all())
}
