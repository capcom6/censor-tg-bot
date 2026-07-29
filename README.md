<div align="center">

# Censor Telegram Bot

**A plugin-based antispam bot for Telegram groups, written in Go.**

[Report Bug](https://github.com/capcom6/censor-tg-bot/issues) ·
[Request Feature](https://github.com/capcom6/censor-tg-bot/issues)

</div>

## Table of Contents

- [Censor Telegram Bot](#censor-telegram-bot)
  - [Table of Contents](#table-of-contents)
  - [About The Project](#about-the-project)
  - [Features](#features)
  - [Built With](#built-with)
  - [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Installation](#installation)
      - [Docker (Recommended)](#docker-recommended)
      - [Docker Compose](#docker-compose)
      - [From Source](#from-source)
  - [Usage](#usage)
  - [Configuration](#configuration)
    - [Environment Variables](#environment-variables)
    - [YAML Configuration](#yaml-configuration)
    - [Plugin Configuration](#plugin-configuration)
      - [Keyword Plugin](#keyword-plugin)
      - [Rate Limit Plugin](#rate-limit-plugin)
      - [Regex Plugin](#regex-plugin)
      - [Forwarded Plugin](#forwarded-plugin)
      - [Duplicate Plugin](#duplicate-plugin)
      - [Users Plugin](#users-plugin)
      - [LLM Plugin](#llm-plugin)
  - [Execution Strategies](#execution-strategies)
    - [Sequential (Default)](#sequential-default)
    - [Parallel](#parallel)
  - [Monitoring](#monitoring)
    - [Prometheus Metrics](#prometheus-metrics)
    - [Grafana Dashboard](#grafana-dashboard)
    - [Prometheus Alerts](#prometheus-alerts)
  - [Creating Custom Plugins](#creating-custom-plugins)
  - [Roadmap](#roadmap)
  - [Contributing](#contributing)
    - [Development Setup](#development-setup)
  - [License](#license)
  - [Contact](#contact)
  - [Acknowledgments](#acknowledgments)

## About The Project

Censor Telegram Bot helps group administrators automatically detect and remove unwanted messages — spam, abuse, scams, flooding, forwarded content, and other policy violations.

A plugin system lets you choose which checks to run, in what order, and whether to execute them sequentially or in parallel. Built-in Prometheus metrics and a pre-configured Grafana dashboard give visibility into filtering effectiveness and plugin performance.

## Features

- **Plugin-based architecture** — extend with custom filtering logic
- **7 built-in plugins**:
  - **Keyword** — block messages containing blacklisted words (case-insensitive, Unicode-normalized)
  - **Regex** — block messages matching regular expression patterns
  - **Rate Limit** — limit messages per user within a time window
  - **Forwarded** — restrict forwarded messages by allowed user/chat IDs
  - **Duplicate** — detect and block repeated messages from a user
  - **Users** — blacklist/whitelist specific user IDs
  - **LLM** — analyze message content via an external LLM API
- **Sequential or parallel** execution strategies
- **Automatic user banning** after configurable violation threshold
- **Admin notifications** — real-time alerts with plugin details for blocked messages and bans
- **Prometheus metrics** — plugin evaluations, durations, errors, and bot actions
- **Grafana dashboard** — pre-configured monitoring dashboard
- **Dockerized** — ready for containerized deployment
- **Cross-platform builds** — Linux, macOS, Windows via GoReleaser

## Built With

- [Go](https://go.dev/)
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) — Telegram Bot API wrapper
- [fx](https://uber-go.github.io/fx/) — dependency injection
- [Fiber](https://gofiber.io/) — HTTP server for metrics endpoint
- [Prometheus](https://prometheus.io/) — metrics and monitoring
- [Grafana](https://grafana.com/) — monitoring dashboards
- [Koanf](https://github.com/knadh/koanf) — configuration management

## Getting Started

### Prerequisites

- Go 1.25+ (for building from source)
- Docker (for containerized deployment)
- Prometheus & Grafana (optional, for monitoring)

### Installation

#### Docker (Recommended)

```sh
docker run -d --name censor-tg-bot \
  --env-file .env \
  ghcr.io/capcom6/censor-tg-bot
```

#### Docker Compose

```yaml
services:
  bot:
    image: ghcr.io/capcom6/censor-tg-bot
    env_file: .env
    restart: unless-stopped
```

#### From Source

```sh
git clone https://github.com/capcom6/censor-tg-bot.git
cd censor-tg-bot
go build -o censor-tg-bot .
./censor-tg-bot
```

## Usage

Create a `.env` file with your bot token and admin ID, then run the bot.

The bot will process messages in groups where it has admin permissions. Blocked messages are deleted, the admin is notified with plugin details, and repeat offenders are automatically banned after exceeding the configured threshold.

```sh
# Run with hot reload during development
air

# Run tests
go test ./...

# Build
make build
```

## Configuration

The application loads configuration from environment variables (via `.env` file) and optionally from a YAML file specified by the `CONFIG_PATH` environment variable.

### Environment Variables

| Variable               | Required | Default                   | Description                                               |
| ---------------------- | -------- | ------------------------- | --------------------------------------------------------- |
| `BOT__ADMIN_ID`        | **Yes**  | —                         | Telegram user ID of the administrator                     |
| `TELEGRAM__TOKEN`      | **Yes**  | —                         | Bot token from [@BotFather](https://t.me/BotFather)       |
| `BOT__BAN_THRESHOLD`   | No       | `3`                       | Number of violations before automatic ban                 |
| `CENSOR__BLACKLIST`    | No       | —                         | Comma-separated blacklist (deprecated, use YAML)          |
| `CENSOR__ENABLED_ONLY` | No       | `true`                    | Only run plugins marked as enabled                        |
| `CENSOR__ERROR_ACTION` | No       | `block`                   | Default action when a plugin errors (`block` or `allow`)  |
| `CENSOR__SKIP_ACTION`  | No       | `allow`                   | Default action when all plugins skip (`block` or `allow`) |
| `CENSOR__STRATEGY`     | No       | `sequential`              | Execution strategy (`sequential` or `parallel`)           |
| `CENSOR__TIMEOUT`      | No       | `30s`                     | Per-plugin execution timeout                              |
| `CONFIG_PATH`          | No       | —                         | Path to YAML configuration file                           |
| `HTTP__ADDRESS`        | No       | `127.0.0.1:3000`          | Metrics endpoint address                                  |
| `HTTP__PROXIES`        | No       | —                         | Comma-separated list of trusted proxy IPs                 |
| `HTTP__PROXY_HEADER`   | No       | `X-Forwarded-For`         | Proxy header for trusted proxies                          |
| `STORAGE__URL`         | No       | `memory://storage?ttl=5m` | Storage backend URL (in-memory with TTL)                  |
| `TELEGRAM__PROXY_URL`  | No       | —                         | SOCKS5 proxy URL                                          |
| `TELEGRAM__TIMEOUT`    | No       | `60s`                     | Timeout for Telegram API requests                         |

### YAML Configuration

Create a `config.yml` file and set the `CONFIG_PATH` environment variable to load it:

```yaml
telegram:
  proxy_url: "socks5://user:pass@127.0.0.1:1080"
  timeout: "30s"

censor:
  strategy: sequential
  timeout: 30s
  enabled_only: true
  error_action: block
  skip_action: allow

http:
  address: "127.0.0.1:3000"
  proxy_header: "X-Forwarded-For"
  proxies: []
```

### Plugin Configuration

Plugins are configured under `censor.plugins` in YAML:

```yaml
censor:
  plugins:
    keyword:
      enabled: true
      priority: 10
      config:
        blacklist:
          - spam
          - scam
          - phishing

    ratelimit:
      enabled: true
      priority: 5
      config:
        max_messages: 5
        window: "1m"

    regex:
      enabled: false
      priority: 20
      config:
        patterns:
          - 'https?://[\w\-\.]+\.xyz'
          - '\b\d{16}\b'

    forwarded:
      enabled: false
      priority: 15
      config:
        allowed_user_ids:
          - 123456789
        allowed_chat_ids:
          - -1001234567890

    duplicate:
      enabled: true
      priority: 150
      config:
        max_duplicates: 1
        window: "5m"

    users:
      enabled: true
      priority: 5
      config:
        blacklist: []
        whitelist: []

    llm:
      enabled: false
      priority: 250
      config:
        api_key: "<your-api-key>"
        model: "nvidia/nemotron-nano-9b-v2:free"
        confidence_threshold: 0.8
        timeout: 30s
        prompt: "Analyze the following message..."
        temperature: 0.1
```

**Note:** Provide your API key through an environment variable (e.g., `CENSOR__PLUGINS__LLM__CONFIG__API_KEY`) or an uncommitted local config file to avoid committing secrets.

#### Keyword Plugin

Blocks messages containing blacklisted keywords with case-insensitive matching and Unicode normalization.

| Config Key  | Type       | Default | Description       |
| ----------- | ---------- | ------- | ----------------- |
| `blacklist` | `[]string` | —       | Keywords to block |

**Use Cases:** Blocking profanity, filtering promotional keywords, preventing specific terminology.

---

#### Rate Limit Plugin

Prevents spam by limiting the number of messages a user can send within a time window.

| Config Key     | Type     | Default | Description                           |
| -------------- | -------- | ------- | ------------------------------------- |
| `max_messages` | `int`    | `5`     | Maximum messages allowed per window   |
| `window`       | `string` | `"1m"`  | Time window (e.g., `30s`, `5m`, `1h`) |

**Use Cases:** Preventing message flooding, limiting bot abuse, protecting against rapid-fire spam.

---

#### Regex Plugin

Blocks messages matching regular expression patterns.

| Config Key | Type       | Default | Description             |
| ---------- | ---------- | ------- | ----------------------- |
| `patterns` | `[]string` | —       | Array of regex patterns |

**Use Cases:** Blocking URL patterns, detecting credit card numbers, filtering complex patterns.

---

#### Forwarded Plugin

Blocks forwarded messages from non-exception sources. Only messages forwarded from allowed user IDs or chat IDs pass through.

| Config Key         | Type      | Default | Description                                 |
| ------------------ | --------- | ------- | ------------------------------------------- |
| `allowed_user_ids` | `[]int64` | `[]`    | User IDs whose forwards are allowed         |
| `allowed_chat_ids` | `[]int64` | `[]`    | Chat/channel IDs whose forwards are allowed |

**Use Cases:** Preventing unauthorized message forwarding, controlling content flow from specific channels.

---

#### Duplicate Plugin

Detects and blocks repetitive messages from a user within a time window. Messages are normalized (lowercased, whitespace-collapsed) before comparison.

| Config Key       | Type     | Default | Valid Range   | Description                                    |
| ---------------- | -------- | ------- | ------------- | ---------------------------------------------- |
| `max_duplicates` | `int`    | `1`     | `>= 0`        | Max duplicate messages allowed before blocking |
| `window`         | `string` | `"5m"`  | `10s` – `24h` | Time window for duplicate detection            |

**Use Cases:** Preventing copy-paste spam, blocking repeated promotional messages, reducing channel noise.

---

#### Users Plugin

Blocks or allows messages based on user IDs. Whitelisted users are always allowed; blacklisted users are always blocked; users in neither list are skipped.

| Config Key  | Type    | Default | Description                                    |
| ----------- | ------- | ------- | ---------------------------------------------- |
| `blacklist` | `[]int` | `[]`    | User IDs to block                              |
| `whitelist` | `[]int` | `[]`    | User IDs to always allow (overrides blacklist) |

**Use Cases:** Blocking known spammers, creating VIP lists, implementing user-based access control.

---

#### LLM Plugin

Blocks messages containing potentially inappropriate content, spam, or violations by analyzing the message with an external LLM API. Supports response caching (LRU eviction) to reduce API costs.

| Config Key             | Type     | Default                             | Valid Range   | Description                       |
| ---------------------- | -------- | ----------------------------------- | ------------- | --------------------------------- |
| `base_url`             | `string` | `"https://openrouter.ai/api/v1"`    | —             | LLM API base URL                  |
| `api_key`              | `string` | `""`                                | —             | API key (required)                |
| `model`                | `string` | `"nvidia/nemotron-nano-9b-v2:free"` | —             | Model identifier                  |
| `confidence_threshold` | `float`  | `0.8`                               | `0.0` – `1.0` | Minimum confidence to block       |
| `timeout`              | `string` | `"30s"`                             | `5s` – `5m`   | API call timeout                  |
| `prompt`               | `string` | *see default*                       | —             | LLM prompt                        |
| `temperature`          | `float`  | `0.1`                               | `0.0` – `2.0` | LLM sampling temperature          |
| `cache_enabled`        | `bool`   | `true`                              | —             | Enable response caching           |
| `cache_ttl`            | `string` | `"1h"`                              | `1m` – `24h`  | Cache entry TTL                   |
| `cache_max_size`       | `int`    | `1000`                              | `> 0`         | Max cached entries (LRU eviction) |

**Use Cases:** Filtering semantic spam that evades keyword matching, multi-language moderation, context-sensitive content filtering.

## Execution Strategies

The censor service supports two strategies:

### Sequential (Default)

Plugins execute in priority order (lower number = earlier execution).

- Each plugin runs sequentially
- If a plugin returns `ActionAllow`, execution stops and the message is allowed
- If a plugin returns `ActionBlock`, execution continues — later plugins may still `ActionAllow`
- If all plugins skip, the configured `skip_action` is applied
- If no plugin explicitly allows but at least one blocks, the message is blocked

**Best for:** Most use cases, efficient resource usage.

```yaml
censor:
  strategy: sequential
```

### Parallel

All plugins execute concurrently, results are aggregated.

- All enabled plugins run simultaneously
- If any plugin returns `ActionAllow`, the message is allowed
- If no allow is found but any plugin returns `ActionBlock`, the message is blocked
- If all plugins skip, the configured `skip_action` is applied

**Best for:** When all plugins must evaluate every message, high-throughput scenarios.

```yaml
censor:
  strategy: parallel
```

## Monitoring

### Prometheus Metrics

The bot exposes metrics at `http://localhost:3000/metrics`:

| Metric                               | Type      | Description                                        |
| ------------------------------------ | --------- | -------------------------------------------------- |
| `censor_plugin_evaluations_total`    | Counter   | Plugin evaluation counts by action                 |
| `censor_plugin_duration_seconds`     | Histogram | Plugin execution duration                          |
| `censor_plugin_errors_total`         | Counter   | Plugin error counts                                |
| `censor_bot_processed_actions_total` | Counter   | Bot action counts (message processed, deletions, bans, notifications) |

### Grafana Dashboard

Import the pre-configured dashboard from [`deployments/grafana/dashboard.json`](deployments/grafana/dashboard.json):

1. Open Grafana
2. Go to Dashboards → Import
3. Upload `deployments/grafana/dashboard.json`
4. Select your Prometheus datasource

**Dashboard panels:** bot action distribution, message filter rate over time, plugin performance metrics, HTTP request statistics.

### Prometheus Alerts

Configure alerts using [`deployments/prometheus/alerts.yml`](deployments/prometheus/alerts.yml):

**Bot Alerts:**

- `HighBotActionFailureRate` — >10% of bot actions fail over 5 minutes
- `BotActionFailures` — >5 bot actions fail in 5 minutes (critical)

**Plugin Alerts:**

- `HighPluginEvaluationFailureRate` — >10% of plugin evaluations fail (warning)
- `HighPluginEvaluationFailures` — >5 plugin evaluations fail in 5 minutes (critical)

**Server Alerts:**

- `HighHTTPErrorRate` — >5% of HTTP requests return 5xx (warning)
- `HighHTTPRequestLatency` — 95th percentile latency exceeds 1 second (warning)
- `HighHTTPThroughput` — request rate exceeds 100 req/s (warning)

## Creating Custom Plugins

The plugin architecture allows you to implement custom filtering logic by implementing the `Plugin` interface. See the [Creating Custom Plugins](docs/creating-custom-plugins.md) guide for details.

## Roadmap

See the [open issues](https://github.com/capcom6/censor-tg-bot/issues) for a full list of proposed features and known issues.

## Contributing

Contributions are welcome!

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Implement your changes with tests
4. Ensure all tests pass (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Development Setup

```sh
go mod download
go test ./...
air                     # hot reload
make build              # build binary
make lint               # run golangci-lint
make coverage           # test with coverage
```

## License

Distributed under the Apache-2.0 License. See [`LICENSE`](./LICENSE) for more information.

## Contact

Project maintainer: [capcom6](https://github.com/capcom6)

Project link: [https://github.com/capcom6/censor-tg-bot](https://github.com/capcom6/censor-tg-bot)

## Acknowledgments

- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) — Telegram Bot API wrapper
- [fx](https://uber-go.github.io/fx/) — Dependency injection framework
- [Fiber](https://gofiber.io/) — HTTP framework
- [Prometheus](https://prometheus.io/) — Monitoring and metrics
- [Grafana](https://grafana.com/) — Monitoring dashboards
- [Best-README-Template](https://github.com/othneildrew/Best-README-Template) — README structure inspiration
