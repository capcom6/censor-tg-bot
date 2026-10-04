package bot_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// testToken is a syntactically valid but fake bot token. No real credential may ever
// appear in this file.
const testToken = "1234567890:TEST-TOKEN-NOT-REAL"

// jpegPayload is a payload [http.DetectContentType] recognises as image/jpeg, so the
// content-type gate passes without a real encoder.
const jpegPayload = "\xff\xd8\xff\xe0 stub jpeg payload"

const (
	metricMediaDownloads = "censor_bot_media_downloads_total"
	metricMediaBytes     = "censor_bot_media_download_bytes_total"
	metricProcessed      = "censor_bot_processed_actions_total"

	resultOK          = "ok"
	resultTooLarge    = "too_large"
	resultNotImage    = "not_image"
	resultFetchFailed = "fetch_failed"

	telegramFilePath  = "photos/file_0.jpg"
	telegramFetchPath = "/bot" + testToken + "/getFile"
	telegramFileRoute = "/file/bot" + testToken + "/" + telegramFilePath
)

// jpegBytes is the payload served for a successful download.
func jpegBytes() []byte {
	return []byte(jpegPayload)
}

// recordingPlugin records every message it receives and decides from its own
// configuration whether it needs media and what to answer, exactly like a real plugin
// does. reason doubles as a stand-in for a model-authored string, which is how the
// escaping path is exercised without an LLM.
type recordingPlugin struct {
	needsMedia bool
	block      bool
	reason     string

	calls int
	last  plugin.Message
}

func (p *recordingPlugin) Name() string { return "recorder" }

func (p *recordingPlugin) Priority() int { return 1 }

func (p *recordingPlugin) Cleanup(context.Context) {}

func (p *recordingPlugin) NeedsMedia(plugin.Message) bool { return p.needsMedia }

func (p *recordingPlugin) Evaluate(_ context.Context, msg plugin.Message) (plugin.Result, error) {
	p.calls++
	p.last = msg

	action := plugin.ActionSkip
	if p.block {
		action = plugin.ActionBlock
	}

	reason := p.reason
	if reason == "" {
		reason = "recorded"
	}

	return plugin.Result{
		Action:   action,
		Reason:   reason,
		Metadata: nil,
		Plugin:   p.Name(),
	}, nil
}

// telegramStub redirects every Telegram request to a local httptest server, because
// Telegram builds file links against the hard-coded api.telegram.org host. Recording
// the paths proves the download reuses the injected client, which is what makes
// telegram.proxy_url apply to media downloads.
type telegramStub struct {
	target *url.URL
	client *http.Client

	paths []string
}

func (s *telegramStub) Do(req *http.Request) (*http.Response, error) {
	s.paths = append(s.paths, req.URL.Path)

	rewritten := req.Clone(req.Context())
	rewritten.URL.Scheme = s.target.Scheme
	rewritten.URL.Host = s.target.Host

	return s.client.Do(rewritten)
}

func (s *telegramStub) sawPath(path string) bool {
	return slices.Contains(s.paths, path)
}

// failTransport fails the requests for one path and passes every other one through.
type failTransport struct {
	path string
}

func (t failTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == t.path {
		return nil, errors.New("stubbed transport failure")
	}

	return http.DefaultTransport.RoundTrip(req)
}

// telegram is a local stand-in for the Telegram API covering getMe, getFile, the file
// download, deleteMessage and sendMessage.
type telegram struct {
	stub *telegramStub
	api  *tgbotapi.BotAPI

	payload []byte
	deleted int
	sent    []string

	// parseErr, when set, makes the sendMessage endpoint reject an HTML send with
	// that description, which is how a Telegram markup rejection is reproduced
	// without a network. alwaysReject rejects every attempt.
	parseErr     error
	alwaysReject bool

	sends []sendAttempt
	bans  int
}

// sendAttempt records one sendMessage call exactly as Telegram received it.
type sendAttempt struct {
	text      string
	parseMode string
}

// newTelegram starts the stub. failPath makes exactly one request path fail at the
// transport level, which is how a download failure is simulated without a network.
func newTelegram(t *testing.T, failPath string) *telegram {
	t.Helper()

	tg := &telegram{payload: jpegBytes()}

	mux := http.NewServeMux()
	mux.HandleFunc("/bot"+testToken+"/getMe", func(w http.ResponseWriter, _ *http.Request) {
		body := `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"bot","username":"bot"}}`
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc(telegramFetchPath, func(w http.ResponseWriter, _ *http.Request) {
		body := fmt.Sprintf(
			`{"ok":true,"result":{"file_id":"photo-id","file_unique_id":"unique","file_size":%d,"file_path":%q}}`,
			len(tg.payload), telegramFilePath,
		)
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc(telegramFileRoute, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tg.payload)
	})
	mux.HandleFunc("/bot"+testToken+"/deleteMessage", func(w http.ResponseWriter, _ *http.Request) {
		tg.deleted++
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})
	mux.HandleFunc("/bot"+testToken+"/banChatMember", func(w http.ResponseWriter, _ *http.Request) {
		tg.bans++
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})
	mux.HandleFunc("/bot"+testToken+"/sendMessage", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		tg.sent = append(tg.sent, r.Form.Get("text"))
		tg.sends = append(tg.sends, sendAttempt{
			text:      r.Form.Get("text"),
			parseMode: r.Form.Get("parse_mode"),
		})

		// Only a send that claims HTML can be rejected for its markup, which is
		// exactly the condition the plain-text retry is meant to recover from.
		if tg.parseErr != nil && (tg.alwaysReject || r.Form.Get("parse_mode") != "") {
			body := fmt.Sprintf(`{"ok":false,"error_code":400,"description":%q}`, tg.parseErr.Error())
			_, _ = w.Write([]byte(body))

			return
		}

		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	require.NoError(t, err)

	tg.stub = &telegramStub{target: target, client: &http.Client{Transport: failTransport{path: failPath}}}
	tg.api, err = tgbotapi.NewBotAPIWithClient(testToken, tgbotapi.APIEndpoint, tg.stub)
	require.NoError(t, err)

	return tg
}

func (tg *telegram) bot() *tgbotapifx.Bot {
	return &tgbotapifx.Bot{BotAPI: tg.api}
}

// counterValue reads one counter out of the default registry, which promauto fills
// process-globally. A label combination that was never incremented reads as 0.
func counterValue(t *testing.T, name string, labels map[string]string) int64 {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != name {
			continue
		}

		for _, metric := range family.GetMetric() {
			got := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				got[label.GetName()] = label.GetValue()
			}

			if maps.Equal(got, labels) {
				return int64(metric.GetCounter().GetValue())
			}
		}
	}

	return 0
}

// mediaCounters snapshots every media counter. promauto registers process-globally and
// `go test -shuffle` randomises the case order, so a case must assert on the change
// it caused rather than on an absolute value.
func mediaCounters(t *testing.T) map[string]int64 {
	t.Helper()

	return map[string]int64{
		resultOK:          counterValue(t, metricMediaDownloads, map[string]string{"result": resultOK}),
		resultTooLarge:    counterValue(t, metricMediaDownloads, map[string]string{"result": resultTooLarge}),
		resultNotImage:    counterValue(t, metricMediaDownloads, map[string]string{"result": resultNotImage}),
		resultFetchFailed: counterValue(t, metricMediaDownloads, map[string]string{"result": resultFetchFailed}),
		metricMediaBytes:  counterValue(t, metricMediaBytes, map[string]string{}),
	}
}

// requireCounterDelta pins the change every media counter saw.
func requireCounterDelta(t *testing.T, before, after map[string]int64, want map[string]int64) {
	t.Helper()

	for name, wantValue := range want {
		require.Equal(t, wantValue, after[name]-before[name], "unexpected change of %s", name)
	}

	for name, startValue := range before {
		if _, expected := want[name]; expected {
			continue
		}

		require.Equal(t, startValue, after[name], "counter %s moved unexpectedly", name)
	}
}

func TestSelectPhotoSize(t *testing.T) {
	const (
		// maxBytes is larger than every reported size in the table unless a case names
		// its own limit, so the byte filter is exercised deliberately.
		maxBytes = 60_000

		dimAboveMedia = 900 // clears minDimensionMedia on both axes
		dimBelowMedia = 400 // clears the floor only
		dimAboveFloor = 300
		dimBelowFloor = 200 // clears nothing

		smallSize = 1_000
		bigSize   = 50_000
	)

	photo := func(id string, width, height, size int) tgbotapi.PhotoSize {
		return tgbotapi.PhotoSize{
			FileID:       id,
			FileUniqueID: id + "-unique",
			Width:        width,
			Height:       height,
			FileSize:     size,
		}
	}

	cases := []struct {
		name     string
		photos   []tgbotapi.PhotoSize
		limit    int64
		wantFile string
	}{
		{
			name: "prefers the smallest variant above the media minimum",
			photos: []tgbotapi.PhotoSize{
				photo("large", dimAboveMedia*2, dimAboveMedia*2, bigSize),
				photo("medium", dimAboveMedia, dimAboveMedia, smallSize),
				photo("medium-larger", dimAboveMedia+50, dimAboveMedia+50, smallSize+1),
			},
			wantFile: "medium",
		},
		{
			name: "filters by size before applying the dimension preference",
			photos: []tgbotapi.PhotoSize{
				photo("huge", 4000, 4000, bigSize),
				photo("floor-only-but-eligible", dimAboveFloor, dimAboveFloor, smallSize),
			},
			limit:    10_000,
			wantFile: "floor-only-but-eligible",
		},
		{
			name: "ignores a floor-clearing variant that is too large",
			photos: []tgbotapi.PhotoSize{
				photo("big", dimAboveMedia, dimAboveMedia, bigSize),
				photo("smaller-but-oversized", 2000, dimAboveFloor, maxBytes+1),
			},
			wantFile: "big",
		},
		{
			name: "panorama falls back to the smallest variant clearing the floor",
			photos: []tgbotapi.PhotoSize{
				photo("wide", 4000, dimAboveFloor, smallSize),
				photo("wider", 4000, dimBelowMedia, smallSize+1),
			},
			wantFile: "wide",
		},
		{
			name: "treats an unknown size as eligible",
			photos: []tgbotapi.PhotoSize{
				photo("unknown", dimBelowMedia, dimBelowMedia, 0),
			},
			limit:    smallSize,
			wantFile: "unknown",
		},
		{
			name: "returns nothing when no variant clears the floor",
			photos: []tgbotapi.PhotoSize{
				photo("pano", 4000, dimBelowFloor, smallSize),
				photo("pano-small", 2000, dimBelowFloor, smallSize),
			},
		},
		{
			name: "returns nothing when every variant is too large",
			photos: []tgbotapi.PhotoSize{
				photo("big", dimAboveMedia, dimAboveMedia, bigSize),
			},
			limit: smallSize,
		},
		{
			name: "returns nothing for a message without a photo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limit := tc.limit
			if limit == 0 {
				limit = maxBytes
			}

			got := bot.SelectPhotoSizeForTest(tc.photos, limit)

			if tc.wantFile == "" {
				require.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			require.Equal(t, tc.wantFile, got.FileID)
		})
	}
}

func TestEvaluateMessage(t *testing.T) {
	const (
		adminID  = 4242
		userID   = 777
		chatID   = 555
		maxBytes = 60_000
	)

	// promauto registers process-globally, so each metrics type is built exactly once
	// per test binary and every case below shares it.
	botMetrics := bot.NewMetrics()
	censorMetrics := censor.NewMetrics()
	logger := zap.NewNop()

	store, err := storage.New(storage.Config{URL: "memory://storage?ttl=5m"})
	require.NoError(t, err)

	photoMessage := func(fileSize int) *tgbotapi.Message {
		return &tgbotapi.Message{
			MessageID: 100,
			From:      &tgbotapi.User{ID: userID, UserName: "spammer"},
			Chat:      &tgbotapi.Chat{ID: chatID},
			Caption:   "look at this",
			Photo: []tgbotapi.PhotoSize{{
				FileID:       "photo-id",
				FileUniqueID: "unique",
				Width:        900,
				Height:       900,
				FileSize:     fileSize,
			}},
		}
	}

	newBot := func(rec *recordingPlugin) *bot.Bot {
		censorSvc := censor.New([]plugin.Plugin{rec}, censor.Config{
			Strategy:    censor.StrategySequential,
			Timeout:     5 * time.Second,
			Plugins:     map[string]censor.PluginConfig{rec.Name(): {Enabled: true, Priority: 1}},
			EnabledOnly: true,
			ErrorAction: plugin.ActionBlock,
			SkipAction:  plugin.ActionSkip,
		}, censorMetrics, logger)

		b, buildErr := bot.New(bot.Config{
			AdminID:      adminID,
			BanThreshold: 3,
			Media: bot.MediaConfig{
				MaxBytes: maxBytes,
				Timeout:  5 * time.Second,
			},
		}, censorSvc, store, botMetrics, logger)
		require.NoError(t, buildErr)

		return b
	}

	evaluate := func(rec *recordingPlugin, tg *telegram, message *tgbotapi.Message) (plugin.Result, string) {
		t.Helper()

		return bot.EvaluateMessageForTest(context.Background(), newBot(rec), tg.bot(), message)
	}

	t.Run("hands the downloaded image to the plugins", func(t *testing.T) {
		tg := newTelegram(t, "")
		rec := &recordingPlugin{needsMedia: true}
		before := mediaCounters(t)

		result, note := evaluate(rec, tg, photoMessage(len(jpegBytes())))

		require.Equal(t, "fetched", note)
		require.Equal(t, plugin.ActionSkip, result.Action)
		require.Equal(t, 1, rec.calls)
		require.True(t, tg.stub.sawPath(telegramFetchPath))
		require.True(t, tg.stub.sawPath(telegramFileRoute))

		require.NotNil(t, rec.last.Media)
		require.Equal(t, jpegBytes(), rec.last.Media.Bytes)
		require.Equal(t, "image/jpeg", rec.last.Media.ContentType)
		require.Equal(t, len(jpegBytes()), rec.last.Media.SizeBytes)

		sum := sha256.Sum256(jpegBytes())
		require.Equal(t, hex.EncodeToString(sum[:]), rec.last.Media.SHA256)

		requireCounterDelta(t, before, mediaCounters(t), map[string]int64{
			resultOK:         1,
			metricMediaBytes: int64(len(jpegBytes())),
		})
	})

	t.Run("downloads nothing when no plugin needs media", func(t *testing.T) {
		tg := newTelegram(t, "")
		rec := &recordingPlugin{needsMedia: false}
		before := mediaCounters(t)

		result, note := evaluate(rec, tg, photoMessage(len(jpegBytes())))

		require.Empty(t, note)
		require.Equal(t, plugin.ActionSkip, result.Action)
		require.Equal(t, 1, rec.calls)
		require.Nil(t, rec.last.Media)
		require.False(t, tg.stub.sawPath(telegramFetchPath), "no media-capable plugin means no round trip")
		requireCounterDelta(t, before, mediaCounters(t), map[string]int64{})
	})

	t.Run("downloads nothing from the admin", func(t *testing.T) {
		tg := newTelegram(t, "")
		rec := &recordingPlugin{needsMedia: true}
		message := photoMessage(len(jpegBytes()))
		message.From.ID = adminID
		before := mediaCounters(t)

		result, note := evaluate(rec, tg, message)

		require.Equal(t, plugin.ActionAllow, result.Action)
		require.Empty(t, note)
		require.Zero(t, rec.calls)
		require.False(t, tg.stub.sawPath(telegramFetchPath))
		requireCounterDelta(t, before, mediaCounters(t), map[string]int64{})
	})

	t.Run("evaluates a message without a photo as text", func(t *testing.T) {
		tg := newTelegram(t, "")
		rec := &recordingPlugin{needsMedia: true}

		message := photoMessage(len(jpegBytes()))
		message.Photo = nil
		message.Text = "hello"
		before := mediaCounters(t)

		result, note := evaluate(rec, tg, message)

		require.Empty(t, note)
		require.Equal(t, plugin.ActionSkip, result.Action)
		require.Equal(t, 1, rec.calls)
		require.Nil(t, rec.last.Media)
		require.False(t, tg.stub.sawPath(telegramFetchPath))
		requireCounterDelta(t, before, mediaCounters(t), map[string]int64{})
	})

	t.Run("skips an image whose every variant is too large", func(t *testing.T) {
		tg := newTelegram(t, "")
		rec := &recordingPlugin{needsMedia: true}
		before := mediaCounters(t)

		result, note := evaluate(rec, tg, photoMessage(maxBytes+1))

		require.Equal(t, resultTooLarge, note)
		require.NotEqual(t, plugin.ActionBlock, result.Action)
		require.Equal(t, 1, rec.calls)
		require.Nil(t, rec.last.Media)
		require.False(t, tg.stub.sawPath(telegramFetchPath))
		requireCounterDelta(t, before, mediaCounters(t), map[string]int64{
			resultTooLarge: 1,
		})
	})

	t.Run("skips bytes that are not an image", func(t *testing.T) {
		tg := newTelegram(t, "")
		tg.payload = []byte("this is plain text, not an image at all")
		rec := &recordingPlugin{needsMedia: true}
		before := mediaCounters(t)

		result, note := evaluate(rec, tg, photoMessage(len(jpegBytes())))

		require.Equal(t, resultNotImage, note)
		require.NotEqual(t, plugin.ActionBlock, result.Action)
		require.Equal(t, 1, rec.calls)
		require.Nil(t, rec.last.Media)
		requireCounterDelta(t, before, mediaCounters(t), map[string]int64{
			resultNotImage: 1,
		})
	})

	t.Run("blocks with a redacted reason when the download fails", func(t *testing.T) {
		tg := newTelegram(t, telegramFileRoute)
		rec := &recordingPlugin{needsMedia: true}
		before := mediaCounters(t)

		result, note := evaluate(rec, tg, photoMessage(len(jpegBytes())))

		require.Equal(t, resultFetchFailed, note)
		require.Equal(t, plugin.ActionBlock, result.Action)
		require.Equal(t, "bot", result.Plugin)
		require.NotContains(t, result.Reason, testToken)
		require.Equal(t, resultFetchFailed, result.Metadata["media"])
		require.NotContains(t, result.Metadata["error"].(string), testToken)
		require.Zero(t, rec.calls, "censor.Evaluate must not run when the image cannot be fetched")
		requireCounterDelta(t, before, mediaCounters(t), map[string]int64{
			resultFetchFailed: 1,
		})
	})

	t.Run("deletes and notifies through the existing chain on a fetch failure", func(t *testing.T) {
		tg := newTelegram(t, telegramFileRoute)
		rec := &recordingPlugin{needsMedia: true}
		before := mediaCounters(t)

		b := newBot(rec)
		handleErr := b.Handler(context.Background(), tg.bot(), tgbotapi.Update{Message: photoMessage(len(jpegBytes()))})

		require.NoError(t, handleErr)
		require.Equal(t, 1, tg.deleted, "the message must be deleted")
		require.Len(t, tg.sent, 1, "the admin must be notified")
		require.Contains(t, tg.sent[0], "telegram photo download failed")
		require.NotContains(t, tg.sent[0], testToken)
		require.Zero(t, rec.calls)
		requireCounterDelta(t, before, mediaCounters(t), map[string]int64{
			resultFetchFailed: 1,
		})
	})

	t.Run("exposes exactly the four media outcomes", func(t *testing.T) {
		require.Equal(t, resultOK, string(bot.MetricLabelMediaResultOK))
		require.Equal(t, resultTooLarge, string(bot.MetricLabelMediaResultTooLarge))
		require.Equal(t, resultNotImage, string(bot.MetricLabelMediaResultNotImage))
		require.Equal(t, resultFetchFailed, string(bot.MetricLabelMediaResultFetchFailed))
	})

	t.Run("registers no unsupported counter and no duration histogram", func(t *testing.T) {
		families, gatherErr := prometheus.DefaultGatherer.Gather()
		require.NoError(t, gatherErr)

		var names []string
		for _, family := range families {
			if strings.HasPrefix(family.GetName(), "censor_bot_") {
				names = append(names, family.GetName())
			}
		}

		require.ElementsMatch(t, []string{
			metricProcessed,
			metricMediaDownloads,
			metricMediaBytes,
		}, names)
	})

	// promauto registers process-globally, so the metrics built above are the only
	// ones this test binary may build. The chain suite therefore runs from here
	// rather than as its own top-level test.
	t.Run("deletion, notification and ban chain", func(t *testing.T) {
		testProcessMessageChain(t, botMetrics, censorMetrics)
	})
}

// captureLogs builds an in-memory zap core plus an accessor for everything written to
// it, so a test can assert that a secret never reaches a log line.
func captureLogs(t *testing.T) (zapcore.Core, func() []string) {
	t.Helper()

	core, recorded := observer.New(zap.WarnLevel)

	entries := func() []string {
		logged := recorded.AllUntimed()
		lines := make([]string, 0, len(logged))
		for _, entry := range logged {
			lines = append(lines, fmt.Sprint(entry.Message, entry.ContextMap()))
		}

		return lines
	}

	return core, entries
}

func TestEscapeTelegramHTML(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "leaves an apostrophe literal",
			in:   "a child's photo",
			want: "a child's photo",
		},
		{
			name: "escapes the four documented named entities",
			in:   `<b>&"'</b>`,
			want: "&lt;b&gt;&amp;&quot;'&lt;/b&gt;",
		},
		{
			name: "never emits a numeric character reference",
			in:   `'"&<>` + "\n\t",
			want: `'&quot;&amp;&lt;&gt;` + "\n\t",
		},
		{
			name: "does not double-escape an existing entity",
			in:   "&amp;",
			want: "&amp;amp;",
		},
		{
			name: "leaves plain text untouched",
			in:   "look at this",
			want: "look at this",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bot.EscapeTelegramHTMLForTest(tc.in)
			require.Equal(t, tc.want, got)
			require.NotContains(t, got, "&#")
		})
	}
}

func TestMessageToString(t *testing.T) {
	const caption = "child's <b>photo</b>"

	cases := []struct {
		name    string
		message *tgbotapi.Message
		note    string
		want    string
	}{
		{
			name:    "a fetched photo is labelled and keeps its escaped caption",
			message: &tgbotapi.Message{Caption: caption},
			note:    "fetched",
			want:    "[photo] child's &lt;b&gt;photo&lt;/b&gt;",
		},
		{
			name:    "a fetched photo without a caption is labelled alone",
			message: &tgbotapi.Message{},
			note:    "fetched",
			want:    "[photo]",
		},
		{
			name:    "an oversized photo says it was not examined",
			message: &tgbotapi.Message{Caption: caption},
			note:    "too_large",
			want:    "[photo not examined: too large]",
		},
		{
			name:    "an unrecognised photo says it was not examined",
			message: &tgbotapi.Message{Caption: caption},
			note:    "not_image",
			want:    "[photo not examined: unrecognized format]",
		},
		{
			name:    "a failed download says it was not examined",
			message: &tgbotapi.Message{Caption: caption},
			note:    "fetch_failed",
			want:    "[photo not examined: download failed]",
		},
		{
			name:    "no note keeps the escaped text of a text message",
			message: &tgbotapi.Message{Text: caption},
			note:    "",
			want:    "child's &lt;b&gt;photo&lt;/b&gt;",
		},
		{
			name:    "no note keeps the escaped caption of a media message",
			message: &tgbotapi.Message{Caption: caption},
			note:    "",
			want:    "child's &lt;b&gt;photo&lt;/b&gt;",
		},
		{
			name:    "a message with nothing to show renders empty",
			message: &tgbotapi.Message{},
			note:    "",
			want:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, bot.MessageToStringForTest(tc.message, tc.note))
		})
	}
}

// testProcessMessageChain drives the whole delete -> notify -> count -> ban chain
// through Handler against the stubbed Telegram API. It is a plain function, not a
// Test, so it shares the process-global metrics TestEvaluateMessage already built.
func testProcessMessageChain(t *testing.T, botMetrics *bot.Metrics, censorMetrics *censor.Metrics) {
	t.Helper()

	const (
		adminID      = 4242
		userID       = 777
		chatID       = 555
		banThreshold = 2
		mediaTimeout = 5 * time.Second
		mediaMax     = 60_000
	)

	logger := zap.NewNop()

	store, err := storage.New(storage.Config{URL: "memory://storage?ttl=30m"})
	require.NoError(t, err)

	// A model writing about a photo emits an apostrophe and raw angle brackets. Those
	// are exactly the characters that break Telegram's HTML parse mode if they reach
	// the sink unescaped, which is why they appear in nearly every case below.
	const modelReason = "a child's photo & <b>tag</b>"

	// newBot wires a bot whose only plugin is rec, through the real censor.Service. The
	// logger is a parameter so a case can capture what reaches the log.
	newBot := func(lg *zap.Logger, rec *recordingPlugin, maxBytes int64) *bot.Bot {
		censorSvc := censor.New([]plugin.Plugin{rec}, censor.Config{
			Strategy:    censor.StrategySequential,
			Timeout:     5 * time.Second,
			Plugins:     map[string]censor.PluginConfig{rec.Name(): {Enabled: true, Priority: 1}},
			EnabledOnly: true,
			ErrorAction: plugin.ActionBlock,
			SkipAction:  plugin.ActionSkip,
		}, censorMetrics, lg)

		b, buildErr := bot.New(bot.Config{
			AdminID:      adminID,
			BanThreshold: banThreshold,
			Media:        bot.MediaConfig{MaxBytes: maxBytes, Timeout: mediaTimeout},
		}, censorSvc, store, botMetrics, lg)
		require.NoError(t, buildErr)

		return b
	}

	// textMessage builds a plain offending text message.
	textMessage := func(id int, from int64) *tgbotapi.Message {
		return &tgbotapi.Message{
			MessageID: id,
			From:      &tgbotapi.User{ID: from, UserName: "spammer"},
			Chat:      &tgbotapi.Chat{ID: chatID},
			Text:      "spam",
		}
	}

	// removals returns only the removal notifications, leaving out the separate
	// "Banned ..." message so a retry is countable on its own.
	removals := func(tg *telegram) []sendAttempt {
		t.Helper()

		var found []sendAttempt
		for _, attempt := range tg.sends {
			if strings.HasPrefix(attempt.text, "Removed message") {
				found = append(found, attempt)
			}
		}

		return found
	}

	// handle runs one update and fails the test if the chain reports an error.
	handle := func(t *testing.T, b *bot.Bot, tg *telegram, message *tgbotapi.Message) {
		t.Helper()

		require.NoError(t, b.Handler(context.Background(), tg.bot(), tgbotapi.Update{Message: message}))
	}

	t.Run("three evaluations of one messageID never reach the ban threshold", func(t *testing.T) {
		tg := newTelegram(t, "")
		b := newBot(logger, &recordingPlugin{block: true}, mediaMax)
		message := textMessage(100, userID)

		// Handler has no IsEdit gate, so an edit is evaluated again. Without the
		// one-violation-per-message rule these three evaluations would count three
		// violations and ban on the second.
		for range 3 {
			handle(t, b, tg, message)
		}

		require.Equal(t, 3, tg.deleted, "every evaluation deletes the message")
		require.Zero(t, tg.bans, "edit spam alone must not escalate to a ban")
	})

	t.Run("distinct messages do escalate to a ban", func(t *testing.T) {
		tg := newTelegram(t, "")
		b := newBot(logger, &recordingPlugin{block: true}, mediaMax)
		const offender = 779

		for id := range banThreshold {
			handle(t, b, tg, textMessage(200+id, offender))
		}

		require.Equal(t, 1, tg.bans, "the threshold of two must be reached by two distinct messages")
	})

	t.Run("a rejected HTML notification is retried once as plain text and the ban still runs", func(t *testing.T) {
		tg := newTelegram(t, "")
		tg.parseErr = errors.New("Bad Request: can't parse entities: Unsupported start tag \"b\"")
		b := newBot(logger, &recordingPlugin{block: true, reason: modelReason}, mediaMax)

		for id := range banThreshold {
			handle(t, b, tg, textMessage(1000+id, 780))
		}

		sent := removals(tg)
		require.Len(t, sent, 2*banThreshold, "each removal is retried exactly once")
		for i, attempt := range sent {
			if i%2 == 0 {
				require.Equal(t, "HTML", attempt.parseMode, "attempt %d asks for HTML", i)

				continue
			}
			require.Empty(t, attempt.parseMode, "attempt %d must carry no parse mode", i)
		}

		require.Equal(t, 1, tg.bans, "a notification failure must never skip the ban")
	})

	t.Run("a notification that fails every attempt still never aborts the chain", func(t *testing.T) {
		tg := newTelegram(t, "")
		tg.parseErr = errors.New("Bad Request: can't parse entities: Unsupported start tag \"b\"")
		tg.alwaysReject = true
		b := newBot(logger, &recordingPlugin{block: true, reason: modelReason}, mediaMax)

		for id := range banThreshold {
			handle(t, b, tg, textMessage(2000+id, 781))
		}

		require.Len(t, removals(tg), 2*banThreshold, "exactly one retry per notification")
		require.Equal(t, 1, tg.bans, "the ban survives a notification that never succeeds")
	})

	t.Run("a non-markup failure is not retried but the ban still runs", func(t *testing.T) {
		tg := newTelegram(t, "")
		tg.parseErr = errors.New("Bad Request: chat not found")
		tg.alwaysReject = true
		b := newBot(logger, &recordingPlugin{block: true, reason: modelReason}, mediaMax)

		for id := range banThreshold {
			handle(t, b, tg, textMessage(3000+id, 782))
		}

		require.Len(t, removals(tg), banThreshold, "a non-markup failure is not worth a second send")
		require.Equal(t, 1, tg.bans)
	})

	t.Run("an unescaped reason never reaches the notification sink raw", func(t *testing.T) {
		tg := newTelegram(t, "")
		b := newBot(logger, &recordingPlugin{block: true, reason: modelReason}, mediaMax)

		handle(t, b, tg, textMessage(4000, 783))

		require.NotEmpty(t, tg.sends)
		require.Contains(t, tg.sends[0].text, "a child's photo &amp; &lt;b&gt;tag&lt;/b&gt;")
		require.NotContains(t, tg.sends[0].text, "<b>tag</b>")
		require.NotContains(t, tg.sends[0].text, "&#")
	})

	t.Run("an unescaped message body never reaches the notification sink raw", func(t *testing.T) {
		tg := newTelegram(t, "")
		b := newBot(logger, &recordingPlugin{block: true, reason: "spam"}, mediaMax)

		message := textMessage(4001, 784)
		message.Text = "a child's <b>photo</b>"

		handle(t, b, tg, message)

		require.NotEmpty(t, tg.sends)
		require.Contains(t, tg.sends[0].text, "a child's &lt;b&gt;photo&lt;/b&gt;")
		require.NotContains(t, tg.sends[0].text, "<b>photo</b>")
	})

	t.Run("a fetched photo is notified as examined", func(t *testing.T) {
		tg := newTelegram(t, "")
		rec := &recordingPlugin{block: true, reason: "explicit content", needsMedia: true}
		b := newBot(logger, rec, mediaMax)

		handle(t, b, tg, &tgbotapi.Message{
			MessageID: 4002,
			From:      &tgbotapi.User{ID: 785, UserName: "photospammer"},
			Chat:      &tgbotapi.Chat{ID: chatID},
			Caption:   "look",
			Photo: []tgbotapi.PhotoSize{{
				FileID:       "photo-id",
				FileUniqueID: "unique",
				Width:        900,
				Height:       900,
			}},
		})

		require.NotEmpty(t, tg.sent)
		require.Contains(t, tg.sent[0], "[photo] look")
		require.NotContains(t, tg.sent[0], "not examined")
	})

	t.Run("a photo too large to download is notified as not examined", func(t *testing.T) {
		tg := newTelegram(t, "")
		rec := &recordingPlugin{block: true, reason: "spam", needsMedia: true}
		b := newBot(logger, rec, 10)

		handle(t, b, tg, &tgbotapi.Message{
			MessageID: 4003,
			From:      &tgbotapi.User{ID: 786, UserName: "bigphotospammer"},
			Chat:      &tgbotapi.Chat{ID: chatID},
			Photo: []tgbotapi.PhotoSize{{
				FileID:       "photo-id",
				FileUniqueID: "unique",
				Width:        900,
				Height:       900,
				FileSize:     50_000,
			}},
		})

		require.NotEmpty(t, tg.sent)
		require.Contains(t, tg.sent[0], "[photo not examined: too large]")
		require.NotContains(t, tg.sent[0], "[photo]")
	})

	t.Run("a failed download is notified as not examined", func(t *testing.T) {
		tg := newTelegram(t, telegramFileRoute)
		rec := &recordingPlugin{block: true, reason: "spam", needsMedia: true}
		b := newBot(logger, rec, mediaMax)

		handle(t, b, tg, &tgbotapi.Message{
			MessageID: 4004,
			From:      &tgbotapi.User{ID: 787, UserName: "failedphoto"},
			Chat:      &tgbotapi.Chat{ID: chatID},
			Photo: []tgbotapi.PhotoSize{{
				FileID:       "photo-id",
				FileUniqueID: "unique",
				Width:        900,
				Height:       900,
			}},
		})

		require.NotEmpty(t, tg.sent)
		require.Contains(t, tg.sent[0], "[photo not examined: download failed]")
		require.NotContains(t, tg.sent[0], "[photo]")
	})

	t.Run("a transport failure while notifying never puts the token in the log", func(t *testing.T) {
		// The send goes through the same token-bearing transport as every other
		// Telegram call, and the stub rewrites the host, so its *url.Error carries the
		// full https://api.telegram.org/bot<TOKEN>/... URL.
		core, logs := captureLogs(t)

		tg := newTelegram(t, "/bot"+testToken+"/sendMessage")
		b := newBot(zap.New(core), &recordingPlugin{block: true, reason: "spam"}, mediaMax)

		handle(t, b, tg, textMessage(4006, 789))

		require.Zero(t, tg.bans, "the threshold of two is not reached by one message")
		require.NotEmpty(t, logs())
		for _, entry := range logs() {
			require.NotContains(t, entry, testToken, "no bot token may reach the log")
		}
	})

	t.Run("the admin-notified counter records a failed send as failed", func(t *testing.T) {
		labels := map[string]string{"action": "admin_notified", "status": "failed"}
		before := counterValue(t, metricProcessed, labels)

		tg := newTelegram(t, "")
		tg.parseErr = errors.New("Bad Request: chat not found")
		tg.alwaysReject = true
		b := newBot(logger, &recordingPlugin{block: true, reason: "spam"}, mediaMax)

		handle(t, b, tg, textMessage(4005, 788))

		require.Equal(t, int64(1), counterValue(t, metricProcessed, labels)-before)
	})
}
