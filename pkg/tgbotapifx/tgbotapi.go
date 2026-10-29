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

// DownloadFile downloads a Telegram file into dst.
//
// The download is bounded by maxBytes: a larger file is rejected instead of
// being written to dst. The returned error never contains the request URL,
// because the Telegram file URL carries the bot token.
func (b *Bot) DownloadFile(ctx context.Context, fileID string, dst io.Writer, maxBytes int64) error {
	file, err := b.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return fmt.Errorf("%w: get file: %w", ErrDownloadFailed, redactURLError(err))
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, file.Link(b.Token), nil)
	if reqErr != nil {
		return fmt.Errorf("%w: build request: %w", ErrDownloadFailed, redactURLError(reqErr))
	}

	resp, respErr := b.Client.Do(req)
	if respErr != nil {
		return fmt.Errorf("%w: get file content: %w", ErrDownloadFailed, redactURLError(respErr))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: unexpected status code %d", ErrDownloadFailed, resp.StatusCode)
	}

	if resp.ContentLength > maxBytes {
		return fmt.Errorf(
			"%w: file of %d bytes exceeds the limit of %d bytes",
			ErrDownloadFailed,
			resp.ContentLength,
			maxBytes,
		)
	}

	// Read one byte more than the limit, so an oversized body is detected
	// instead of being buffered in full.
	written, copyErr := io.Copy(dst, io.LimitReader(resp.Body, maxBytes+1))
	if copyErr != nil {
		return fmt.Errorf("%w: copy file content: %w", ErrDownloadFailed, redactURLError(copyErr))
	}

	if written > maxBytes {
		return fmt.Errorf("%w: file exceeds the limit of %d bytes", ErrDownloadFailed, maxBytes)
	}

	return nil
}

// redactURLError removes the request URL from a transport error.
//
// The HTTP client returns a [url.Error], whose message contains the URL of the
// failed request. The Telegram file URL carries the bot token, so the error is
// unwrapped before it can reach a log. The timeout and status information of
// the underlying error is preserved.
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}

	return err
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if b.handler == nil {
		b.logger.Warn("no handler set")
		return nil
	}

	return b.handler(ctx, b, update)
}
