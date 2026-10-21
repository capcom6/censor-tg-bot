package tgbotapifx_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

// testToken is the token of the fake bot. It must never appear in an error.
const testToken = "123456:test-TOKEN"

// telegramTransport sends the API calls to the fake Telegram server, keeping the
// path of the original request, and fails the file download when asked to.
type telegramTransport struct {
	base         *url.URL
	next         http.RoundTripper
	failDownload bool
	err          error
}

func (t *telegramTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.failDownload && strings.Contains(req.URL.Path, "/file/") {
		return nil, t.err
	}

	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.base.Scheme
	clone.URL.Host = t.base.Host
	clone.Host = t.base.Host

	return t.next.RoundTrip(clone)
}

// newFakeTelegram starts a fake Telegram API that serves a file of the given
// size and returns a bot bound to it.
func newFakeTelegram(t *testing.T, fileSize int, contentLength int, failDownload bool) *tgbotapifx.Bot {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,` +
				`"first_name":"test","username":"test_bot"}}`))
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			_, _ = fmt.Fprintf(w,
				`{"ok":true,"result":{"file_id":"file-id","file_unique_id":"unique",`+
					`"file_path":"photos/file_1.jpg","file_size":%d}}`,
				fileSize,
			)
		default:
			body := bytes.Repeat([]byte("a"), fileSize)
			if contentLength >= 0 {
				w.Header().Set("Content-Length", strconv.Itoa(contentLength))
			}

			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(server.Close)

	base, err := url.Parse(server.URL)
	require.NoError(t, err)

	client := &http.Client{
		Transport: &telegramTransport{
			base:         base,
			next:         http.DefaultTransport,
			failDownload: failDownload,
			err:          errors.New("connection refused"),
		},
	}

	api, err := tgbotapi.NewBotAPIWithClient(testToken, tgbotapi.APIEndpoint, client)
	require.NoError(t, err)

	return &tgbotapifx.Bot{BotAPI: api}
}

func TestBot_DownloadFile(t *testing.T) {
	t.Parallel()

	bot := newFakeTelegram(t, 10, -1, false)

	dst := &bytes.Buffer{}
	require.NoError(t, bot.DownloadFile(context.Background(), "file-id", dst, 1024))

	require.Equal(t, 10, dst.Len())
}

func TestBot_DownloadFileRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	// A body of more than 2048 bytes is sent chunked, so the client does not
	// know the size and has to enforce the limit while reading.
	bot := newFakeTelegram(t, 4096, -1, false)

	dst := &bytes.Buffer{}
	err := bot.DownloadFile(context.Background(), "file-id", dst, 1024)

	require.ErrorIs(t, err, tgbotapifx.ErrDownloadFailed)
	require.Contains(t, err.Error(), "exceeds the limit")
	require.LessOrEqual(t, dst.Len(), 1025)
}

func TestBot_DownloadFileRejectsOversizedContentLength(t *testing.T) {
	t.Parallel()

	bot := newFakeTelegram(t, 2048, 2048, false)

	dst := &bytes.Buffer{}
	err := bot.DownloadFile(context.Background(), "file-id", dst, 1024)

	require.ErrorIs(t, err, tgbotapifx.ErrDownloadFailed)
	require.Contains(t, err.Error(), "exceeds the limit")
	require.Empty(t, dst.Bytes())
}

func TestBot_DownloadFileErrorKeepsTheTokenSecret(t *testing.T) {
	t.Parallel()

	bot := newFakeTelegram(t, 10, -1, true)

	err := bot.DownloadFile(context.Background(), "file-id", &bytes.Buffer{}, 1024)
	require.Error(t, err)
	require.NotContains(t, err.Error(), testToken)
}
