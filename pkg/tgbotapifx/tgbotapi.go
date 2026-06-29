package tgbotapifx

import (
	"context"
	"fmt"
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

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if b.handler == nil {
		b.logger.Warn("no handler set")
		return nil
	}

	return b.handler(ctx, b, update)
}
