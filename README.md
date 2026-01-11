# Censor Telegram Bot

A flexible, plugin-based antispam bot for Telegram, written in Go. Features a modular architecture for message filtering with built-in Prometheus metrics and Grafana dashboards.

## Features

- **Plugin-Based Architecture**: Extensible modular design for custom filtering logic
- **Built-in Plugins**:
  - **Keyword**: Block messages containing blacklisted keywords
  - **Rate Limit**: Prevent spam by limiting messages per user per time window
  - **Regex**: Block messages matching regex patterns
- **Execution Strategies**: Sequential or parallel plugin execution
- **Prometheus Metrics**: Track plugin performance and message filtering statistics
- **Grafana Dashboards**: Pre-configured dashboards for monitoring
- **Automatic User Banning**: Ban users after threshold violations
- **Admin Notifications**: Real-time alerts for blocked messages
- **Dockerized Deployment**: Easy containerized deployment

## Prerequisites

- Go 1.24+ (for building from source)
- Docker (for containerized deployment)
- Prometheus & Grafana (optional, for monitoring)

## Configuration

### Basic Configuration

The app uses environment variables or a configuration file. Create a `.env` file:

```sh
# Telegram Bot Settings
TELEGRAM__TOKEN=xxx:yyyyy              # Bot token from @BotFather
BOT__ADMIN_ID=123456789                # Your Telegram user ID
BOT__BAN_THRESHOLD=3                   # Number of violations before ban

# Censor Settings
CENSOR__STRATEGY=sequential            # sequential|parallel
CENSOR__TIMEOUT=30s                    # Plugin execution timeout
CENSOR__ENABLED_ONLY=true              # Only run enabled plugins

# Storage (for violation tracking)
STORAGE__URL=memory://storage?ttl=5m   # In-memory storage with 5-minute TTL

# HTTP Server
HTTP__ADDRESS=127.0.0.1:3000           # Metrics endpoint address
```

### Plugin Configuration

Configure plugins using environment variables or YAML configuration:

#### Environment Variables (Deprecated, Keyword only)
```sh
CENSOR__BLACKLIST='spam,scam,phishing' # Comma-separated blacklist
```

#### YAML Configuration (Recommended)
Create a `config.yml` file:

```yaml
censor:
  strategy: sequential
  timeout: 30s
  enabled_only: true
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
          - 'https?://[\w\-\.]+\.xyz'  # Block .xyz domains
          - '\b\d{16}\b'               # Block credit card numbers
```

Pass `CONFIG_PATH` environment variable to specify a path to a YAML configuration file.

## Plugin Reference

### Keyword Plugin
Blocks messages containing blacklisted keywords with case-insensitive matching and Unicode normalization.

**Configuration:**
```yaml
keyword:
  enabled: true
  priority: 10  # Lower number = earlier execution
  config:
    blacklist:
      - spam
      - scam
      - phishing
```

**Use Cases:**
- Blocking profanity
- Filtering promotional keywords
- Preventing specific terminology

---

### Rate Limit Plugin
Prevents spam by limiting the number of messages a user can send within a time window.

**Configuration:**
```yaml
ratelimit:
  enabled: true
  priority: 5   # Execute early to short-circuit expensive plugins
  config:
    max_messages: 5
    window: "1m"  # 1 minute window
```

**Use Cases:**
- Preventing message flooding
- Limiting bot abuse
- Protecting against rapid-fire spam

---

### Regex Plugin
Blocks messages matching regular expression patterns.

**Configuration:**
```yaml
regex:
  enabled: true
  priority: 20  # Execute after simpler checks
  config:
    patterns:
      - 'https?://bit\.ly/\w+'  # Block shortened URLs
      - '\b[A-Z]{5,}\b'         # Block excessive caps
```

**Use Cases:**
- Blocking URL patterns
- Detecting credit card numbers
- Filtering complex patterns

---

### Forwarded Plugin
Blocks forwarded messages from non-exception sources, allowing only messages from specified user IDs and chat IDs.

**Configuration:**
```yaml
forwarded:
  enabled: true
  priority: 15  # Execute after rate limiting and keyword, before regex
  config:
    allowed_user_ids:
      - 123456789
      - 987654321
    allowed_chat_ids:
      - -1001234567890
```

**Use Cases:**
- Preventing unauthorized message forwarding in groups
- Controlling content flow from specific channels/chats
- Implementing content source restrictions

---

### LLM Plugin
Blocks messages containing potentially inappropriate content, spam, or violations by analyzing the message with an external large language model (LLM) API.

Includes an optional caching feature to reduce LLM API call volume, lower operational costs, and improve response times by storing valid responses for repeated messages (LRU eviction triggers when cache_max_size is exceeded).

**Configuration:**
```yaml
llm:
  enabled: true
  priority: 200
  config:
    api_key: ""               # Required API key for LLM service
    model: "nvidia/nemotron-nano-9b-v2:free"  # LLM model to use
    confidence_threshold: 0.8 # Threshold for blocking (0.0-1.0)
    timeout: 30s              # API call timeout
    prompt: "Analyze the following message for inappropriate content, spam, or violations. Respond with JSON: {\"inappropriate\": boolean, \"confidence\": float, \"reason\": string}"
    temperature: 0.1          # Temperature for LLM sampling
    cache_enabled: true       # Toggle cache functionality (default: true)
    cache_ttl: "1h"           # Cache entry TTL (valid range: 1m-24h, default: 1h)
    cache_max_size: 1000      # Max cached entries (triggers LRU eviction, default: 1000)
```

**Use Cases:**
- Filtering malicious or phishing attempts via natural language patterns
- Detecting subtle spam that evades keyword matching
- Handling context-sensitive content that requires semantic understanding
- Multi-language message moderation

---

## Execution Strategies

The censor service supports two execution strategies that determine how plugins process messages:

### Sequential (Default)

Plugins execute in priority order (lower priority number = earlier execution), stopping at the first block decision.

**Best for:** Most use cases, efficient resource usage

**Behavior:**

- Plugins are sorted by priority
- Execution stops when the first plugin returns `ActionBlock`
- Subsequent plugins are skipped after a block decision
- Ideal for short-circuiting expensive operations

```yaml
censor:
  strategy: sequential
```

### Parallel

All plugins execute concurrently, with results aggregated after all complete.

**Best for:** When all plugins must evaluate every message, high-performance scenarios

**Behavior:**

- All enabled plugins run simultaneously
- Results are collected and aggregated
- A block decision from any plugin results in message blocking
- Better throughput but higher resource usage

```yaml
censor:
  strategy: parallel
```

## Running

### Docker (Recommended)

```sh
docker run -d --name censor-tg-bot \
  --env-file .env \
  ghcr.io/capcom6/censor-tg-bot
```

### Docker Compose

```yaml
services:
  bot:
    image: ghcr.io/capcom6/censor-tg-bot
    env_file: .env
    restart: unless-stopped
```

### From Source

```sh
# Clone repository
git clone https://github.com/capcom6/censor-tg-bot.git
cd censor-tg-bot

# Build
go build -o censor-tg-bot .

# Run
./censor-tg-bot
```

## Monitoring

### Prometheus Metrics

The bot exposes Prometheus metrics on `http://localhost:3000/metrics`:

**Available Metrics:**
- `censor_plugin_evaluations_total` - Plugin evaluation counts by action
- `censor_plugin_duration_seconds` - Plugin execution duration histogram
- `censor_plugin_errors_total` - Plugin error counts
- `bot_processed_actions_total` - Bot action counts (deletions, bans, notifications)

### Grafana Dashboard

Import the pre-configured dashboard from [`deployments/grafana/dashboard.json`](deployments/grafana/dashboard.json):

1. Open Grafana
2. Go to Dashboards → Import
3. Upload `deployments/grafana/dashboard.json`
4. Select your Prometheus datasource

**Dashboard Includes:**
- Bot action distribution
- Message filter rate over time
- Plugin performance metrics
- HTTP request statistics

### Prometheus Alerts

Configure alerts using [`deployments/prometheus/alerts.yml`](deployments/prometheus/alerts.yml):

**Available Alerts:**

**Bot Alerts:**
- `HighBotActionFailureRate` - Triggered when >10% of bot actions fail over 5 minutes
- `BotActionFailures` - Critical alert when >5 bot actions fail in 5 minutes

**Plugin Alerts:**
- `HighPluginEvaluationFailureRate` - Warning when >10% of plugin evaluations fail
- `HighPluginEvaluationFailures` - Critical when >5 plugin evaluations fail in 5 minutes

**Server Alerts:**
- `HighHTTPErrorRate` - Warning when >5% of HTTP requests return 5xx errors
- `HighHTTPRequestLatency` - Warning when 95th percentile latency exceeds 1 second
- `HighHTTPThroughput` - Warning when request rate exceeds 100 requests/second

## Creating Custom Plugins

The plugin architecture allows you to create custom filtering logic by implementing the `Plugin` interface. The details can be found in the [Creating Custom Plugins](docs/creating-custom-plugins.md) section of the documentation.

## Troubleshooting

### Plugin Timeout Errors
**Symptom:** Logs show "timeout" errors during plugin evaluation

**Solution:**
- Increase `CENSOR__TIMEOUT` value
- Optimize slow plugins
- Use `parallel` strategy instead of `sequential`

### High Memory Usage
**Symptom:** Bot consumes excessive memory

**Solution:**
- Reduce `STORAGE__URL` TTL value
- Enable rate limit plugin cleanup
- Monitor with `prometheus` metrics

### Messages Not Being Filtered
**Symptom:** Blacklisted messages are not blocked

**Solution:**
- Check plugin priority order
- Review logs for plugin evaluation results
- Ensure keywords are lowercase in configuration

### Bot Not Responding
**Symptom:** Bot doesn't process any messages

**Solution:**
- Verify `TELEGRAM__TOKEN` is correct
- Check bot has admin permissions in the chat
- Review logs for connection errors
- Ensure firewall allows outbound HTTPS

## Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Implement your changes with tests
4. Ensure all tests pass (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`) 
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Development Setup

```sh
# Install dependencies
go mod download

# Run tests
go test ./...

# Run with hot reload
air

# Build
make build
```

## License

Distributed under the Apache-2.0 license. See [LICENSE](./LICENSE) for more information.

## Acknowledgments

- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) - Telegram Bot API wrapper
- [fx](https://uber-go.github.io/fx/) - Dependency injection framework
- [Prometheus](https://prometheus.io/) - Monitoring and metrics
