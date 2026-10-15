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
      - [Photo Moderation](#photo-moderation)
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
  - **Duplicate** — detect and block repeated messages from a user, and optionally repeated photos by content hash
  - **Users** — blacklist/whitelist specific user IDs
  - **LLM** — analyze message content via an external LLM API; optionally moderate photos with a vision model (opt-in, off by default)
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
| `BOT__MEDIA__MAX_BYTES`| No       | `5242880`                 | Largest Telegram photo (in bytes) the bot downloads       |
| `BOT__MEDIA__TIMEOUT`  | No       | `5s`                      | Deadline for one photo download                          |
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

bot:
  media:
    max_bytes: 5242880
    timeout: 5s

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

`bot.media` only matters when a media-capable plugin is enabled — see [Photo Moderation](#photo-moderation).

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
        image_enabled: false # see "Photo Moderation"

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
        image_enabled: false # see "Photo Moderation"; needs vision_model
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
| `image_enabled`  | `bool`   | `false` | —             | Also deduplicate photos by SHA-256 content hash |

**Use Cases:** Preventing copy-paste spam, blocking repeated promotional messages, reducing channel noise.

Text and photo candidates are counted **independently** against the same `window` and `max_duplicates`, so a repeated caption and a repeated photo never consume one shared budget. See [Photo Moderation](#photo-moderation) for what photo matching does and does not catch.

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

| Config Key             | Type     | Default                             | Valid Range         | Description                       |
| ---------------------- | -------- | ----------------------------------- | ------------------- | --------------------------------- |
| `base_url`             | `string` | `"https://openrouter.ai/api/v1"`    | —                   | LLM API base URL                  |
| `api_key`              | `string` | `""`                                | —                   | API key (required)                |
| `model`                | `string` | `"nvidia/nemotron-nano-9b-v2:free"` | —                   | Model identifier                  |
| `confidence_threshold` | `float`  | `0.8`                               | `0.0` – `1.0`       | Minimum confidence to block       |
| `timeout`              | `string` | `"30s"`                             | `5s` – `5m`         | API call timeout                  |
| `prompt`               | `string` | *see default*                       | —                   | LLM prompt                        |
| `temperature`          | `float`  | `0.1`                               | `0.0` – `2.0`       | LLM sampling temperature          |
| `cache_enabled`        | `bool`   | `true`                              | —                   | Enable response caching           |
| `cache_ttl`            | `string` | `"1h"`                              | `1m` – `24h`        | Cache entry TTL                   |
| `cache_max_size`       | `int`    | `1000`                              | `> 0`               | Max cached entries **per modality** (LRU eviction) |
| `image_enabled`        | `bool`   | `false`                             | —                   | Moderate photos with a vision model |
| `vision_model`         | `string` | `""` (no default)                   | `1` – `256` chars   | Vision model for photos; required when `image_enabled` is `true` |
| `image_prompt`         | `string` | *see default*                       | `<= 4096` chars     | Instruction sent with every photo |
| `max_inline_image_bytes`| `int64`  | `0` (no additional limit)           | `>= 0`              | Local budget for shipping one photo to the provider, in decoded image bytes |

**Use Cases:** Filtering semantic spam that evades keyword matching, multi-language moderation, context-sensitive content filtering.

**Notes:**

- `cache_max_size` now applies **per modality**: the plugin holds two independent caches, one for text and one for photos, each bounded by `cache_max_size`. This is intentional — photo keys are unique per image while text keys repeat, so one shared LRU would let photo traffic evict exactly the text entries that keep the text path cheap, causing **more** upstream calls. Do not "optimize" the two caches back into one. The photo cache therefore holds up to `cache_max_size` (1000 by default) near-never-hit entries per TTL, which is bounded wasted memory, not a leak.
- There is deliberately **no default `vision_model`**. The default `model` is a small text model, and no vision model has been verified as dependable enough to delete messages and ban users on. Setting `image_enabled: true` without a `vision_model` fails at startup rather than silently sending photos to the text model.
- `max_inline_image_bytes` is **local policy, not a provider limit**. `0` means the guard does not run and `bot.media.max_bytes` is the only bound. It is measured in **decoded image bytes** — the same unit as `bot.media.max_bytes` — and deliberately *not* in base64-inflated or HTTP request-body bytes (base64 expands by 4/3). It only has an effect when set at or below `bot.media.max_bytes`, which is already enforced twice over (a `FileSize` pre-filter and a hard read limit). Use it to say "never ship large photos to a third party". The provider publishes no inline-image size limit at all, so there is no provider number to compare against.

---

#### Photo Moderation

Two independent, **opt-in** photo features. Both are off by default; with both off, no photo bytes ever leave Telegram and no existing deployment changes behavior.

Offending photos are **deleted**, exactly like any other offending message. The bot never edits, blurs, redacts or re-uploads a photo, and the sender is banned at the usual threshold.

Only Telegram **photos** are handled. Stickers, documents, videos and animations are out of scope: they carry no photo, nothing is downloaded, nothing is deleted, and both image paths skip.

##### Image deduplication

Independent of any LLM and free of any provider cost. It reuses the Duplicate plugin and needs one key:

```yaml
censor:
  plugins:
    duplicate:
      enabled: true
      priority: 150
      config:
        max_duplicates: 1
        window: "5m"
        image_enabled: true
```

Duplicate image hashing matches only byte-identical Telegram-hosted images (re-posts, forwards). It does **not** catch re-uploads or re-encodes of the same picture; perceptual hashing is out of scope for v1.

##### Vision-LLM photo moderation

Runs in the LLM plugin and sends each photo to the vision model you name:

```yaml
censor:
  plugins:
    llm:
      enabled: true
      priority: 250
      config:
        api_key: "<your-api-key>"
        image_enabled: true
        vision_model: "<your-vision-model-id>" # no default; required when image_enabled is true
        # image_prompt: "..."                  # optional, has a default
        # max_inline_image_bytes: 1048576      # optional local budget, 0 = no additional limit
```

A caption is analyzed by the text model first. Only if that does not block the message, and only when the photo was really downloaded, is the vision model called. Photo messages with **no caption at all** are still examined.

If `duplicate.image_enabled` is true but `llm.image_enabled` is false, photos are hashed for deduplication and never sent to any provider.

##### What gets downloaded

The bot picks the cheapest Telegram-hosted variant that fits `bot.media.max_bytes`: the smallest one at least 512×512, falling back to the smallest one still at least 256×256. The lower floor exists for panoramas, which have no large-enough variant on their short axis. No image library is involved and nothing is re-encoded — Telegram's own downscaled variant is downloaded as-is and hashed once with SHA-256; that digest is what both features use.

A photo is **skipped, never blocked**, when:

- no variant fits `bot.media.max_bytes` (`censor_bot_media_downloads_total{result="too_large"}`)
- the downloaded bytes are not an image (`result="not_image"`)
- fewer than 2 seconds remain of `censor.timeout`, so a request that could not finish anyway is never started
- `max_inline_image_bytes` is set and the photo's decoded size exceeds it. The download still happened (`result="ok"`); only the provider call was skipped.

##### Privacy: photos go to a third party

**If `llm.image_enabled` is true, every incoming photo is base64-encoded and sent to the provider named by `llm.base_url` (OpenRouter by default), and potentially onward to that provider's own model host.** Photos in a group are the personal content of people who did not consent to being uploaded elsewhere. The photo is uploaded to obtain a moderation verdict; this bot does not store it, but the provider's handling of it is outside this project's control.

- The feature is **off by default** and requires an explicit `image_enabled: true` plus a `vision_model` you chose.
- Supply your provider API key through an environment variable (e.g. `CENSOR__PLUGINS__LLM__CONFIG__API_KEY`) or an uncommitted local config file. Never put a real key, bot token or admin ID in a committed file.
- `bot.media.max_bytes` bounds how large a photo can be before it is uploaded at all; `max_inline_image_bytes` can lower that further.

##### Cost: there is no local spend ceiling

**No local cost-limiting mechanism was implemented.** `censor.timeout` bounds how long a single call may take, not how many calls happen, and the pre-existing text LLM path has no ceiling either.

- **Do not enable `image_enabled` in a large group without an upstream provider spend limit.** Every uncached photo is a billable request, and edits of the same message are re-moderated, so spend scales with photo volume rather than with violations.
- **Free-tier vision is not dependable for this job.** OpenRouter's own documentation says free models are "usually not suitable for production use", with **50 requests/day** unless you have purchased at least 10 credits, and **1000 requests/day** once you have. Free variants may also have different rate limits or availability than paid ones. Hitting the rate limit produces request failures, and request failures on this path block and ban — see the accepted risks below.
- This is one reason the bot ships **no default vision model**: you must choose one and accept its cost and its reliability.

##### Accepted risks

These are deliberate design decisions, not bugs. Read them before enabling `image_enabled` in a group with real users.

> **Vendor outage — ACCEPTED:** with image moderation enabled, any non-admin message whose vision call fails is deleted and its sender banned once it reaches `BOT__BAN_THRESHOLD` (default 3 within 5 minutes) — including completely benign, caption-less photos from first-time senders. During a vendor outage this is a mass-ban event.

> **Telegram download failure — ACCEPTED:** a Telegram file-download failure **also** deletes and bans. Any non-admin photo message whose image download fails is deleted and its sender banned at `BOT__BAN_THRESHOLD`, including completely benign photos. A Telegram-side or network incident is a mass-ban event for photo messages.

The plugin path is **fail-closed** by design: a vendor error becomes a block, never a skip, so a broken or hostile provider cannot be used to wave content through. The cost of that choice is that a provider-side failure is indistinguishable from a violation. If that trade is wrong for your group, leave `image_enabled` off.

A failed admin notification is *not* part of that chain: `notifyAdmins` returns nothing and is called as a bare statement, so a notification that cannot be sent can never skip the violation count or the ban. The worst case is a silent admin.

##### Known limitations

- **Duplicate image hashing is byte-identical only.** Re-uploads and re-encodes of the same picture are not detected; perceptual hashing is out of scope for v1.
- **Edits are re-downloaded and re-moderated.** The update handler processes edited messages with no edit gate, so editing a photo triggers another download and another vision call, costing another provider request. The damage is capped: a violation is counted **at most once per (chat, message)**, so repeated edits of one message cannot escalate a ban on their own. Repeated vision spend on edits is accepted.
- **`cache_max_size` is per modality.** See the [LLM Plugin](#llm-plugin) notes. The photo cache holds up to `cache_max_size` near-never-hit entries per TTL — bounded wasted memory, not a leak, because an expired entry self-evicts on read whether or not the periodic cleanup reaches it. That self-eviction is also why "cleanup clears both caches" is not machine-verifiable: no black-box test can distinguish a running cleanup from an absent one, and the worst case is exactly that bounded memory.
- **The update pump is serial.** One update is handled at a time from a 100-buffered channel, so per-message latency is `bot.media.timeout` + `censor.timeout` (5s + 30s by default) and backpressure is invisible — there is no queue-depth metric.
- **`go test -count=2` (or higher) panics** in `internal/bot` and `internal/censor` on duplicate Prometheus metric registration. `promauto` registers process-globally, and the usual `sync.OnceValue` fix is unavailable because `gochecknoglobals` is enforced for `_test.go`. Use `-count=1`, which is what `make test` runs.

##### Metrics

`censor_bot_media_downloads_total{result="ok|too_large|not_image|fetch_failed"}` and `censor_bot_media_download_bytes_total` are documented in [Prometheus Metrics](#prometheus-metrics).

Watch `{result="fetch_failed"}` as the early signal for Telegram-side download problems, which is the trigger for the second accepted risk above.

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
| `censor_bot_media_downloads_total`   | Counter   | Telegram photo download attempts by outcome (`ok`, `too_large`, `not_image`, `fetch_failed`) |
| `censor_bot_media_download_bytes_total` | Counter | Total image bytes downloaded from Telegram |

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
