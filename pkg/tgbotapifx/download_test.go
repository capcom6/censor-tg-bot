package tgbotapifx_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

// testToken is a syntactically valid but fake bot token. No real credential may ever
// appear in this file.
const testToken = "1234567890:TEST-TOKEN-NOT-REAL"

// telegramStub is a tgbotapi.HTTPClient that redirects every Telegram request to a
// local httptest server.
//
// Telegram builds file links against the hard-coded api.telegram.org host
// (tgbotapi.FileEndpoint), so going through the client interface is the only way to
// observe a download without reaching the real API. Recording the paths also proves
// that the download reuses the injected client, which is what makes telegram.proxy_url
// apply to media downloads.
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

func TestDownloadFile(t *testing.T) {
	const (
		fileID     = "photo-file-id"
		filePath   = "photos/file_0.jpg"
		fetchPath  = "/bot" + testToken + "/getFile"
		download   = "/file/bot" + testToken + "/" + filePath
		limitBytes = 4096
	)

	fileBytes := []byte("\xff\xd8\xff\xe0 stub jpeg payload")

	// The knobs a subtest turns before the request. The server goroutine only reads
	// them, and every write happens before the request that reads it.
	var (
		getFileBody  string
		downloadBody []byte
		downloadCode int
	)

	newStub := func(t *testing.T, failPath string) *telegramStub {
		t.Helper()

		mux := http.NewServeMux()
		mux.HandleFunc("/bot"+testToken+"/getMe", func(w http.ResponseWriter, _ *http.Request) {
			body := `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"bot","username":"bot"}}`
			_, _ = w.Write([]byte(body))
		})
		mux.HandleFunc(fetchPath, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(getFileBody))
		})
		mux.HandleFunc(download, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(downloadCode)
			_, _ = w.Write(downloadBody)
		})

		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		target, err := url.Parse(server.URL)
		require.NoError(t, err)

		return &telegramStub{
			target: target,
			client: &http.Client{Transport: failTransport{path: failPath}},
			paths:  nil,
		}
	}

	newBot := func(t *testing.T, stub *telegramStub) *tgbotapifx.Bot {
		t.Helper()

		api, err := tgbotapi.NewBotAPIWithClient(testToken, tgbotapi.APIEndpoint, stub)
		require.NoError(t, err)

		// An unexported field may be left out of a keyed literal, so the download can
		// be driven without the proxy-aware constructor.
		return &tgbotapifx.Bot{BotAPI: api}
	}

	okGetFileBody := fmt.Sprintf(
		`{"ok":true,"result":{"file_id":%q,"file_unique_id":"unique","file_size":%d,"file_path":%q}}`,
		fileID, len(fileBytes), filePath,
	)

	t.Run("downloads the file through the injected client", func(t *testing.T) {
		getFileBody, downloadBody, downloadCode = okGetFileBody, fileBytes, http.StatusOK
		stub := newStub(t, "")
		b := newBot(t, stub)

		data, err := b.DownloadFile(context.Background(), fileID, limitBytes)

		require.NoError(t, err)
		require.Equal(t, fileBytes, data)
		require.True(t, stub.sawPath(fetchPath), "GetFile must go through the injected client")
		require.True(t, stub.sawPath(download), "the file GET must go through the injected client")
	})

	t.Run("accepts a file exactly at the limit", func(t *testing.T) {
		getFileBody, downloadBody, downloadCode = okGetFileBody, fileBytes, http.StatusOK
		b := newBot(t, newStub(t, ""))

		data, err := b.DownloadFile(context.Background(), fileID, int64(len(fileBytes)))

		require.NoError(t, err)
		require.Equal(t, fileBytes, data)
	})

	t.Run("reports a file larger than the limit", func(t *testing.T) {
		getFileBody, downloadBody, downloadCode = okGetFileBody, fileBytes, http.StatusOK
		b := newBot(t, newStub(t, ""))

		data, err := b.DownloadFile(context.Background(), fileID, int64(len(fileBytes))-1)

		require.ErrorIs(t, err, tgbotapifx.ErrFileTooLarge)
		require.Nil(t, data)
	})

	t.Run("reports a non-200 response without the token", func(t *testing.T) {
		getFileBody, downloadBody, downloadCode = okGetFileBody, nil, http.StatusInternalServerError
		b := newBot(t, newStub(t, ""))

		data, err := b.DownloadFile(context.Background(), fileID, limitBytes)

		require.ErrorIs(t, err, tgbotapifx.ErrDownloadFailed)
		require.Contains(t, err.Error(), "500")
		require.NotContains(t, err.Error(), testToken)
		require.Nil(t, data)
	})

	t.Run("redacts the token when the file GET fails", func(t *testing.T) {
		getFileBody, downloadBody, downloadCode = okGetFileBody, fileBytes, http.StatusOK
		stub := newStub(t, download)
		b := newBot(t, stub)

		// The link Telegram hands out really does carry the token, so the assertion
		// below is not vacuous: the raw transport error names it.
		fileURL := fmt.Sprintf(tgbotapi.FileEndpoint, testToken, filePath)
		rawReq, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, fileURL, nil)
		require.NoError(t, reqErr)
		_, rawErr := stub.Do(rawReq)
		require.Error(t, rawErr)
		require.Contains(t, rawErr.Error(), testToken)

		data, err := b.DownloadFile(context.Background(), fileID, limitBytes)

		require.ErrorIs(t, err, tgbotapifx.ErrDownloadFailed)
		require.NotContains(t, err.Error(), testToken)
		require.NotContains(t, err.Error(), "api.telegram.org")
		require.Nil(t, data)
	})

	t.Run("redacts the token when GetFile fails", func(t *testing.T) {
		getFileBody, downloadBody, downloadCode = okGetFileBody, fileBytes, http.StatusOK
		b := newBot(t, newStub(t, fetchPath))

		data, err := b.DownloadFile(context.Background(), fileID, limitBytes)

		require.ErrorIs(t, err, tgbotapifx.ErrDownloadFailed)
		require.NotContains(t, err.Error(), testToken)
		require.Nil(t, data)
	})

	t.Run("reports a Bot API error without the token", func(t *testing.T) {
		getFileBody = `{"ok":false,"error_code":400,"description":"Bad Request: invalid file_id"}`
		downloadBody, downloadCode = fileBytes, http.StatusOK
		b := newBot(t, newStub(t, ""))

		data, err := b.DownloadFile(context.Background(), "unknown-file-id", limitBytes)

		require.ErrorIs(t, err, tgbotapifx.ErrDownloadFailed)
		require.NotContains(t, err.Error(), testToken)
		require.Nil(t, data)
	})
}
