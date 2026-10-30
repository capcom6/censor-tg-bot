# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### New Features

#### Image Processing

- **Photo moderation** — photos in messages are downloaded once per evaluation and passed to plugins that declare media support:
  - **Duplicate** compares photos by SHA-256 content digest, so the same image is detected even when its caption changes
  - **LLM** sends the image to the model next to the caption, so pictures are evaluated even without text
  - New download limits: `BOT__MEDIA__MAX_BYTES` (default `10485760` bytes / 10 MiB) and `BOT__MEDIA__TIMEOUT` (default `5s`) bound each photo download
  - A failed photo download no longer skips moderation — text-based checks still run on the message

## [0.13.0] - 2026-07-27

### New Features

#### LLM Plugin

- **Custom provider URL** — LLM plugin now supports a `base_url` config option, allowing you to use any OpenAI-compatible API provider (not only OpenRouter)
  ```yaml
  llm:
    config:
      base_url: "https://openrouter.ai/api/v1"
  ```
  When unset, defaults to `https://openrouter.ai/api/v1`.

## [0.12.0] - 2026-07-03

### New Features

- **Configurable Telegram API timeout** — set `TELEGRAM__TIMEOUT` (env) or `telegram.timeout` (YAML) to control the HTTP timeout for Telegram API requests (default `60s`)

## [0.11.0] - 2026-06-04

### New Features

- **Telegram API proxy** — route Bot API traffic through a SOCKS5 proxy via `TELEGRAM__PROXY_URL` (env) or `telegram.proxy_url` (YAML)
  ```bash
  TELEGRAM__PROXY_URL=socks5://user:pass@127.0.0.1:1080
  ```

### Bug Fixes

- Fixed a crash at startup caused by the message-evaluation counter registering the same metric name as the per-plugin counter (regression in 0.10.3) — message evaluations are again exported as `censor_evaluations_total`

## [0.10.0] - 2026-03-03

### New Features

#### Users Plugin

- **User blacklist/whitelist** — block or always-allow messages based on the sender's user ID; whitelisted users bypass all other checks
  ```yaml
  users:
    enabled: true
    config:
      blacklist: [123456789]
      whitelist: [987654321]
  ```

## [0.9.2] - 2026-02-02

### Behavior Changes

- **Allow now takes precedence over Block** — a message explicitly allowed by one plugin is no longer blocked by another plugin's block decision (previously the first block decided the outcome in sequential mode)

## [0.9.1] - 2026-01-17

### Bug Fixes

- Admin notifications now include photo captions (previously only `message.Text` was shown, so media messages had no content) and display the sender as a clickable name

## [0.9.0] - 2026-01-15

### New Features

#### LLM Plugin

- **Response caching** — LLM verdicts are cached in an LRU cache to reduce API calls and cost; configure with `cache_enabled`, `cache_ttl` (default `1h`), and `cache_max_size` (default `1000` entries)

## [0.8.0] - 2026-01-09

### New Features

#### LLM Plugin

- **LLM-based message analysis** — optionally block messages judged inappropriate by an external LLM API, with a configurable confidence threshold (default `0.8`)

## [0.7.0] - 2025-12-26

### New Features

#### Duplicate Plugin

- **Duplicate message detection** — blocks repeated messages from the same user within a configurable time window (default `5m`, allowed range `10s`–`24h`)

## [0.6.0] - 2025-12-22

### New Features

#### Forwarded Plugin

- **Forwarded message restriction** — blocks forwarded messages unless they come from an allowed user ID or chat ID

## [0.5.0] - 2025-12-19

### New Features

#### Plugin System

- **Plugin-based architecture** — filtering is now organized into plugins with their own priority and config; ships with **Keyword**, **Rate Limit**, and **Regex** plugins and supports `sequential` or `parallel` execution strategies
  ```yaml
  censor:
    strategy: parallel
    plugins:
      keyword:
        enabled: true
        priority: 10
        config:
          blacklist: [spam, scam]
  ```

## [0.4.0] - 2025-12-06

### New Features

#### Monitoring

- **Prometheus metrics** — the bot exports plugin and bot-action metrics on its HTTP endpoint (`/metrics`), with a pre-configured Grafana dashboard (`deployments/grafana/dashboard.json`) and Prometheus alert rules (`deployments/prometheus/alerts.yml`)

## [0.3.2] - 2025-11-27

### Bug Fixes

- Fixed a crash when processing messages that have no sender — such messages are now skipped instead of causing a nil-pointer panic

## [0.3.1] - 2024-11-08

### Behavior Changes

- **Sliding violation window** — violation counters now use a sliding window: the count resets only after the window elapses without new activity, instead of restarting on a fixed schedule from the first violation

## [0.3.0] - 2024-10-09

### New Features

- **Caption validation** — photo and document captions are now checked like regular message text

## [0.2.0] - 2024-10-07

### New Features

- **Channel post moderation** — the bot now processes channel posts in addition to group messages

## [0.1.0] - 2024-09-30

### New Features

- **Automatic user bans** — users are banned automatically after exceeding the configured violation threshold

## [0.0.1] - 2024-09-18

### New Features

- **Initial release** — keyword blacklist moderation for Telegram groups, packaged as a Docker image with CI publishing
