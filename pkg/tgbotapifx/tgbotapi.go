package tgbotapifx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
)

type Handler func(ctx context.Context, bot *Bot, update tgbotapi.Update) error

type Bot struct {
	*tgbotapi.BotAPI

	config Config

	handler Handler

	logger *zap.Logger
}

func New(config Config, logger *zap.Logger) (*Bot, error) {
	var api *tgbotapi.BotAPI

	client, err := newClient(config.ProxyURL, config.Timeout)
	if err != nil {
		return nil, err
	}

	api, err = tgbotapi.NewBotAPIWithClient(config.Token, tgbotapi.APIEndpoint, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	api.Debug = config.Debug

	return &Bot{
		BotAPI:  api,
		config:  config,
		handler: nil,
		logger:  logger,
	}, nil
}

func newClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	if proxyURL == "" {
		return &http.Client{
			Timeout: timeout,
		}, nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse proxy URL: %w", ErrInvalidConfig, err)
	}

	dialer, err := proxy.FromURL(u, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create proxy dialer: %w", ErrInvalidConfig, err)
	}

	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("%w: proxy dialer does not support context", ErrInvalidConfig)
	}

	transport := &http.Transport{
		DialContext: contextDialer.DialContext,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

func (b *Bot) SetDefaultHandler(handler Handler) {
	b.handler = handler
}

// DownloadFile returns the bytes of a Telegram file, downloading at most maxBytes.
//
// The call costs exactly one Bot API round trip (GetFile) plus one GET of the link
// Telegram returned. Both go through b.Client, the same proxy-aware [http.Client] the
// rest of the bot uses, so telegram.proxy_url covers media downloads without a
// second transport. Because that link embeds the bot token, every error leaves this
// function redacted: neither the link, nor the token, nor a raw [url.Error] is ever
// returned or logged.
//
// The body is read through a LimitReader of maxBytes+1, so at most maxBytes+1 bytes
// are pulled off the network. A file larger than maxBytes yields ErrFileTooLarge, and
// a transport failure, a non-200 response or an unreadable body yields
// ErrDownloadFailed.
func (b *Bot) DownloadFile(ctx context.Context, fileID string, maxBytes int64) ([]byte, error) {
	file, err := b.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, fmt.Errorf("%w: get file: %w", ErrDownloadFailed, RedactURLError(err))
	}

	// maxReadBytes is one byte past the limit, which is what makes the size check exact.
	const maxReadBytes = 1

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, file.Link(b.Token), nil)
	if reqErr != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrDownloadFailed, RedactURLError(reqErr))
	}

	resp, respErr := b.Client.Do(req)
	if respErr != nil {
		return nil, fmt.Errorf("%w: get file content: %w", ErrDownloadFailed, RedactURLError(respErr))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status code %d", ErrDownloadFailed, resp.StatusCode)
	}

	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBytes+maxReadBytes))
	if readErr != nil {
		return nil, fmt.Errorf("%w: read body: %w", ErrDownloadFailed, RedactURLError(readErr))
	}

	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrFileTooLarge, maxBytes)
	}

	return data, nil
}

// RedactURLError strips the request URL from an error before it can be logged.
//
// Telegram puts the bot token in every request URL
// (https://api.telegram.org/bot<TOKEN>/...), and both [http.Client] and
// tgbotapi.MakeRequest surface transport failures as a [url.Error] whose Error
// repeats that URL verbatim. Replacing such an error with its cause keeps the
// transport reason and drops the token. The loop repeats the conversion so a nested
// [url.Error], which a redirect would produce, cannot leak through either.
//
// It is exported because callers outside this package log their own Telegram
// failures, and they must strip the token before doing so, not after.
func RedactURLError(err error) error {
	for {
		var urlErr *url.Error
		if !errors.As(err, &urlErr) || urlErr.Err == nil {
			return err
		}

		err = urlErr.Err
	}
}

func (b *Bot) Run(ctx context.Context) {
	updates := b.GetUpdatesChan(tgbotapi.UpdateConfig{
		Timeout: int(b.config.LongPollTimeout.Seconds()),
	})
	defer b.StopReceivingUpdates()

	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				b.logger.Warn("updates channel closed")
				return
			}
			if err := b.handleUpdate(ctx, update); err != nil {
				b.logger.Error("error handling update", zap.Error(err))
			}
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if b.handler == nil {
		b.logger.Warn("no handler set")
		return nil
	}

	return b.handler(ctx, b, update)
}
