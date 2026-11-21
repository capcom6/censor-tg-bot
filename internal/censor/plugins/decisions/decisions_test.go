package decisions_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/decisions"
	"github.com/go-core-fx/cachefx/cache"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

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

// answerResponse builds a valid decisions response with the given answers.
func answerResponse(t *testing.T, choice string, choiceProb, spam, abuse float64) string {
	t.Helper()

	probabilities := map[string]float64{
		"spam":  (1 - choiceProb) / 3,
		"scam":  (1 - choiceProb) / 3,
		"abuse": (1 - choiceProb) / 3,
		"other": (1 - choiceProb) / 3,
	}
	probabilities[choice] = choiceProb

	resp := decisions.Response{
		Model: "test-model",
		Answers: map[string]decisions.Answer{
			"category": {
				Type:          "choice",
				Choice:        choice,
				Confidence:    0.9,
				Probabilities: probabilities,
			},
			"spam":  {Type: "noul", Noul: new(spam)},
			"abuse": {Type: "noul", Noul: new(abuse)},
		},
		Usage: decisions.Usage{InputTokens: 120, OutputTokens: 12},
	}

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	return string(body)
}

// newFakeProvider starts a provider that answers with the given body and
// records the request bodies.
func newFakeProvider(t *testing.T, status int, responseBody string) (*httptest.Server, *requestRecorder) {
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
		w.WriteHeader(status)
		_, _ = w.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)

	return server, recorder
}

// newPlugin builds a plugin pointed at the fake provider.
func newPlugin(t *testing.T, baseURL string, overrides map[string]any) plugin.Plugin {
	t.Helper()

	return newPluginWithDeps(t, baseURL, overrides, newDeps())
}

// newPluginWithDeps builds a plugin pointed at the fake provider over the
// given deps, so tests can share one cache between instances.
func newPluginWithDeps(t *testing.T, baseURL string, overrides map[string]any, deps plugin.Deps) plugin.Plugin {
	t.Helper()

	params := map[string]any{
		"base_url": baseURL,
		"api_key":  "test-key",
		"model":    "test-model",
	}
	maps.Copy(params, overrides)

	config, err := decisions.NewConfig(params)
	require.NoError(t, err)

	p, err := decisions.New(config, deps)
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

func TestPlugin_EvaluateBlocksOnCategory(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "spam", 0.95, 0.6, 0.1))
	p := newPlugin(t, server.URL, nil)

	result, err := p.Evaluate(context.Background(), plugin.Message{
		Text:   "Want more viewers? Buy 1000 followers",
		ChatID: 1,
	})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result.Action)
	require.Contains(t, result.Reason, "spam")
	require.Equal(t, "spam", result.Metadata["category"])
	require.Equal(t, false, result.Metadata["cached"])
	require.InDelta(t, 0.95, result.Metadata["category_probabilities"].(map[string]float64)["spam"], 1e-9)
	require.Equal(t, "decisions", result.Plugin)

	requests := recorder.all()
	require.Len(t, requests, 1)
	require.Contains(t, requests[0], `"model":"test-model"`)
	require.Contains(t, requests[0], "Buy 1000 followers")
	require.Contains(t, requests[0], `"type":"choice"`)
	require.Contains(t, requests[0], `"type":"noul"`)
	require.NotContains(t, requests[0], `"context"`)
}

func TestPlugin_EvaluateSkipsCleanMessage(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0.1, 0.1))
	p := newPlugin(t, server.URL, nil)

	result, err := p.Evaluate(context.Background(), plugin.Message{Text: "hi chat", ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)
	require.Contains(t, result.Reason, "appears appropriate")
	require.Equal(t, false, result.Metadata["cached"])
	require.Len(t, recorder.all(), 1)
}

func TestPlugin_EvaluateBlocksOnSpamNoul(t *testing.T) {
	t.Parallel()

	server, _ := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.6, 0.97, 0.1))
	p := newPlugin(t, server.URL, nil)

	result, err := p.Evaluate(context.Background(), plugin.Message{Text: "promo", ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result.Action)
	require.Contains(t, result.Reason, "unsolicited advertising")
}

func TestPlugin_EvaluateBlocksOnAbuseNoul(t *testing.T) {
	t.Parallel()

	server, _ := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.6, 0.1, 0.96))
	p := newPlugin(t, server.URL, nil)

	result, err := p.Evaluate(context.Background(), plugin.Message{Text: "you are trash", ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result.Action)
	require.Contains(t, result.Reason, "personal attack")
}

func TestPlugin_EvaluateThresholdBoundary(t *testing.T) {
	t.Parallel()

	t.Run("at threshold blocks", func(t *testing.T) {
		t.Parallel()

		server, _ := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.5, 0.7, 0.1))
		p := newPlugin(t, server.URL, nil)

		result, err := p.Evaluate(context.Background(), plugin.Message{Text: "promo", ChatID: 1})
		require.NoError(t, err)
		require.Equal(t, plugin.ActionBlock, result.Action)
	})

	t.Run("below threshold skips", func(t *testing.T) {
		t.Parallel()

		server, _ := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.5, 0.69, 0.1))
		p := newPlugin(t, server.URL, nil)

		result, err := p.Evaluate(context.Background(), plugin.Message{Text: "promo", ChatID: 1})
		require.NoError(t, err)
		require.Equal(t, plugin.ActionSkip, result.Action)
	})
}

func TestPlugin_EvaluateCachesAnswers(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "spam", 0.95, 0.1, 0.1))
	p := newPlugin(t, server.URL, nil)

	message := plugin.Message{Text: "Want followers?", ChatID: 1}

	first, err := p.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, first.Metadata["cached"])

	second, err := p.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, true, second.Metadata["cached"])
	require.Equal(t, first.Action, second.Action)
	require.Equal(t, first.Reason, second.Reason)

	require.Len(t, recorder.all(), 1)
}

func TestPlugin_EvaluateCachesPerContextAndModel(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0.1, 0.1))

	// All instances share one cache, so a hit proves the cache key itself
	// carries the varying configuration.
	deps := newDeps()
	plain := newPluginWithDeps(t, server.URL, nil, deps)
	withContext := newPluginWithDeps(t, server.URL, map[string]any{"context": "news group"}, deps)
	otherModel := newPluginWithDeps(t, server.URL, map[string]any{"model": "other-model"}, deps)

	message := plugin.Message{Text: "same text", ChatID: 1}

	first, err := plain.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, first.Metadata["cached"])

	second, err := plain.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, true, second.Metadata["cached"])

	// A changed context misses, then hits when it repeats.
	third, err := withContext.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, third.Metadata["cached"])

	fourth, err := withContext.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, true, fourth.Metadata["cached"])

	// A changed model misses, then hits when it repeats.
	fifth, err := otherModel.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, false, fifth.Metadata["cached"])

	sixth, err := otherModel.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, true, sixth.Metadata["cached"])

	require.Len(t, recorder.all(), 3)
}

func TestPlugin_EvaluateSkipsEmptyMessage(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0.1, 0.1))
	p := newPlugin(t, server.URL, nil)

	result, err := p.Evaluate(context.Background(), plugin.Message{ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)
	require.Contains(t, result.Reason, "empty message")

	require.Empty(t, recorder.all())
}

func TestPlugin_EvaluateSkipsMediaOnlyMessage(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0.1, 0.1))
	p := newPlugin(t, server.URL, nil)

	result, err := p.Evaluate(context.Background(), plugin.Message{
		ChatID: 1,
		Media: &plugin.Media{
			Bytes:       []byte("image content"),
			ContentType: "image/jpeg",
			SizeBytes:   len("image content"),
			SHA256:      "d58b3fa7",
		},
	})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)
	require.Contains(t, result.Reason, "media-only")

	require.Empty(t, recorder.all())
}

func TestPlugin_EvaluateUsesCaptionWhenTextEmpty(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0.1, 0.1))
	p := newPlugin(t, server.URL, nil)

	result, err := p.Evaluate(context.Background(), plugin.Message{Caption: "a caption", ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)

	requests := recorder.all()
	require.Len(t, requests, 1)
	require.Contains(t, requests[0], "a caption")
}

func TestPlugin_EvaluateRequestCarriesQuestionText(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0.1, 0.1))
	p := newPlugin(t, server.URL, map[string]any{
		"questions": map[string]any{
			"category": map[string]any{
				"instructions": "Pick the message kind",
				"criteria": map[string]any{
					"spam": "custom spam criteria",
				},
			},
			"spam": map[string]any{
				"instructions": "Is this a custom ad?",
			},
		},
	})

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.NoError(t, err)

	requests := recorder.all()
	require.Len(t, requests, 1)
	require.Contains(t, requests[0], "Pick the message kind")
	require.Contains(t, requests[0], "custom spam criteria")
	require.Contains(t, requests[0], "Is this a custom ad?")
	// Untouched question keeps its default text.
	require.Contains(t, requests[0], "Is this message insulting, threatening, or harassing someone?")
}

func TestPlugin_EvaluateSendsContextInState(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0.1, 0.1))
	p := newPlugin(t, server.URL, map[string]any{"context": "a group about cats"})

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.NoError(t, err)

	requests := recorder.all()
	require.Len(t, requests, 1)
	require.Contains(t, requests[0], `"context":"a group about cats"`)
}

func TestPlugin_EvaluateHTTPError(t *testing.T) {
	t.Parallel()

	server, _ := newFakeProvider(t, http.StatusUnauthorized, `{"detail":"invalid api key"}`)
	p := newPlugin(t, server.URL, nil)

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.Error(t, err)
	require.ErrorIs(t, err, decisions.ErrRequestFailed)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "invalid api key")
}

func TestPlugin_EvaluateDecodeError(t *testing.T) {
	t.Parallel()

	server, _ := newFakeProvider(t, http.StatusOK, `not json`)
	p := newPlugin(t, server.URL, nil)

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.ErrorIs(t, err, decisions.ErrRequestFailed)
}

func TestPlugin_EvaluateMissingAnswer(t *testing.T) {
	t.Parallel()

	response := `{
		"model": "test-model",
		"answers": {
			"category": {"type":"choice","choice":"other","confidence":0.9,"probabilities":{"other":0.9,"spam":0.1}},
			"spam": {"type":"noul","noul":0.1}
		},
		"usage": {"input_tokens":10,"output_tokens":5}
	}`
	server, _ := newFakeProvider(t, http.StatusOK, response)
	p := newPlugin(t, server.URL, nil)

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.ErrorIs(t, err, decisions.ErrInvalidAnswer)
	require.Contains(t, err.Error(), `"abuse"`)
}

func TestPlugin_EvaluateWrongAnswerType(t *testing.T) {
	t.Parallel()

	response := `{
		"model": "test-model",
		"answers": {
			"category": {"type":"choice","choice":"other","confidence":0.9,"probabilities":{"other":0.9,"spam":0.1}},
			"spam": {"type":"noul","noul":0.1},
			"abuse": {"type":"choice","choice":"spam","confidence":0.9,"probabilities":{"spam":1.0}}
		},
		"usage": {"input_tokens":10,"output_tokens":5}
	}`
	server, _ := newFakeProvider(t, http.StatusOK, response)
	p := newPlugin(t, server.URL, nil)

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.ErrorIs(t, err, decisions.ErrInvalidAnswer)
	require.Contains(t, err.Error(), "want")
}

func TestPlugin_EvaluateInvalidProbability(t *testing.T) {
	t.Parallel()

	server, _ := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 1.5, 0.1))
	p := newPlugin(t, server.URL, nil)

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.ErrorIs(t, err, decisions.ErrInvalidProbability)
}

func TestPlugin_EvaluateMissingCategoryProbability(t *testing.T) {
	t.Parallel()

	response := `{
		"model": "test-model",
		"answers": {
			"category": {"type":"choice","choice":"scam","confidence":0.9,"probabilities":{"other":0.9,"spam":0.1}},
			"spam": {"type":"noul","noul":0.1},
			"abuse": {"type":"noul","noul":0.1}
		},
		"usage": {"input_tokens":10,"output_tokens":5}
	}`
	server, _ := newFakeProvider(t, http.StatusOK, response)
	p := newPlugin(t, server.URL, nil)

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.ErrorIs(t, err, decisions.ErrInvalidAnswer)
	require.Contains(t, err.Error(), "scam")
}

func TestPlugin_EvaluateMissingNoulProbability(t *testing.T) {
	t.Parallel()

	t.Run("missing field", func(t *testing.T) {
		t.Parallel()

		response := `{
			"model": "test-model",
			"answers": {
				"category": {"type":"choice","choice":"other","confidence":0.9,"probabilities":{"other":0.9,"spam":0.1}},
				"spam": {"type":"noul"},
				"abuse": {"type":"noul","noul":0.1}
			},
			"usage": {"input_tokens":10,"output_tokens":5}
		}`
		server, _ := newFakeProvider(t, http.StatusOK, response)
		p := newPlugin(t, server.URL, nil)

		_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
		require.ErrorIs(t, err, decisions.ErrInvalidAnswer)
		require.Contains(t, err.Error(), `"spam"`)
	})

	t.Run("null value", func(t *testing.T) {
		t.Parallel()

		response := `{
			"model": "test-model",
			"answers": {
				"category": {"type":"choice","choice":"other","confidence":0.9,"probabilities":{"other":0.9,"spam":0.1}},
				"spam": {"type":"noul","noul":0.1},
				"abuse": {"type":"noul","noul":null}
			},
			"usage": {"input_tokens":10,"output_tokens":5}
		}`
		server, _ := newFakeProvider(t, http.StatusOK, response)
		p := newPlugin(t, server.URL, nil)

		_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
		require.ErrorIs(t, err, decisions.ErrInvalidAnswer)
		require.Contains(t, err.Error(), `"abuse"`)
	})

	t.Run("zero probability is valid", func(t *testing.T) {
		t.Parallel()

		server, _ := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0, 0))
		p := newPlugin(t, server.URL, nil)

		result, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
		require.NoError(t, err)
		require.Equal(t, plugin.ActionSkip, result.Action)
	})
}

func TestPlugin_EvaluateRejectsUnknownCategoryChoice(t *testing.T) {
	t.Parallel()

	response := `{
		"model": "test-model",
		"answers": {
			"category": {"type":"choice","choice":"unexpected","confidence":0.9,"probabilities":{"unexpected":1.0}},
			"spam": {"type":"noul","noul":0.1},
			"abuse": {"type":"noul","noul":0.1}
		},
		"usage": {"input_tokens":10,"output_tokens":5}
	}`
	server, _ := newFakeProvider(t, http.StatusOK, response)
	p := newPlugin(t, server.URL, nil)

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.ErrorIs(t, err, decisions.ErrInvalidAnswer)
	require.Contains(t, err.Error(), `"unexpected"`)
}

func TestPlugin_EvaluateDoesNotCacheInvalidResponses(t *testing.T) {
	t.Parallel()

	response := `{
		"model": "test-model",
		"answers": {
			"category": {"type":"choice","choice":"other","confidence":0.9,"probabilities":{"other":0.9,"spam":0.1}},
			"spam": {"type":"noul","noul":0.1}
		},
		"usage": {"input_tokens":10,"output_tokens":5}
	}`
	server, recorder := newFakeProvider(t, http.StatusOK, response)
	p := newPlugin(t, server.URL, nil)

	_, err := p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.ErrorIs(t, err, decisions.ErrInvalidAnswer)

	// The invalid response was not cached: the second call re-queries the API
	// instead of replaying the cached failure.
	_, err = p.Evaluate(context.Background(), plugin.Message{Text: "hello", ChatID: 1})
	require.ErrorIs(t, err, decisions.ErrInvalidAnswer)
	require.Len(t, recorder.all(), 2)
}

func TestPlugin_PriorityAndCapabilities(t *testing.T) {
	t.Parallel()

	server, _ := newFakeProvider(t, http.StatusOK, answerResponse(t, "other", 0.9, 0.1, 0.1))
	p := newPlugin(t, server.URL, nil)

	require.Equal(t, "decisions", p.Name())
	require.Equal(t, plugin.CapabilityText, p.Capabilities())
	require.Equal(t, 250, p.Priority())
}

// failingCache fails every operation, exercising the degrade-to-miss path.
type failingCache struct{}

func (failingCache) Set(_ context.Context, _ string, _ []byte, _ ...cache.Option) error {
	return errors.New("cache unavailable")
}

func (failingCache) SetOrFail(_ context.Context, _ string, _ []byte, _ ...cache.Option) error {
	return errors.New("cache unavailable")
}

func (failingCache) Increment(_ context.Context, _ string, _ int64, _ ...cache.Option) (int64, error) {
	return 0, errors.New("cache unavailable")
}

func (failingCache) Get(_ context.Context, _ string, _ ...cache.GetOption) ([]byte, error) {
	return nil, errors.New("cache unavailable")
}

func (failingCache) GetAndDelete(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("cache unavailable")
}

func (failingCache) Delete(_ context.Context, _ string) error {
	return errors.New("cache unavailable")
}

func (failingCache) Cleanup(_ context.Context) error {
	return errors.New("cache unavailable")
}

func (failingCache) Drain(_ context.Context) (map[string][]byte, error) {
	return nil, errors.New("cache unavailable")
}

func (failingCache) Close() error {
	return nil
}

func TestPlugin_EvaluateDegradesOnCacheFailure(t *testing.T) {
	t.Parallel()

	server, recorder := newFakeProvider(t, http.StatusOK, answerResponse(t, "spam", 0.95, 0.1, 0.1))
	config, err := decisions.NewConfig(map[string]any{
		"base_url": server.URL,
		"api_key":  "test-key",
		"model":    "test-model",
	})
	require.NoError(t, err)

	core, observed := observer.New(zap.WarnLevel)
	p, err := decisions.New(config, plugin.Deps{Cache: failingCache{}, Logger: zap.New(core)})
	require.NoError(t, err)

	result, err := p.Evaluate(context.Background(), plugin.Message{Text: "promo", ChatID: 1})

	// A cache failure never fails the evaluation; the decision comes from the API.
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result.Action)
	require.Equal(t, false, result.Metadata["cached"])
	require.Len(t, recorder.all(), 1)

	// Both cache failures are visible as Warn logs, not swallowed.
	messages := make([]string, 0, len(observed.All()))
	for _, entry := range observed.All() {
		messages = append(messages, entry.Message)
	}
	require.Contains(t, messages, "failed to read decisions cache")
	require.Contains(t, messages, "failed to write decisions cache")
}
