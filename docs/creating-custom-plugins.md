## Creating Custom Plugins

The plugin architecture allows you to create custom filtering logic by implementing the `Plugin` interface.

### Plugin Interface

All plugins must implement three methods:

```go
type Plugin interface {
    // Name returns the unique identifier for this plugin
    Name() string

    // Evaluate inspects a message and returns a decision
    Evaluate(ctx context.Context, msg Message) (Result, error)

    // Priority returns the execution priority (lower = earlier execution)
    Priority() int
}
```

### Plugin Actions

Plugins return one of three actions:

- **`ActionSkip`** - Plugin has no opinion, continue to next plugin
- **`ActionAllow`** - Explicitly allow the message
- **`ActionBlock`** - Block the message and trigger deletion/ban

### Step 1: Create Plugin Directory Structure

Create a new directory under `internal/censor/plugins/myplugin/` with the following files:

**`config.go`** - Define plugin configuration:

```go
package myplugin

type Config struct {
    // Your configuration fields
    Threshold int
}

func NewConfig(data map[string]any) (Config, error) {
    // Parse configuration from map
    return Config{}, nil
}
```

**`myplugin.go`** - Implement the plugin logic:

```go
package myplugin

import (
    "context"
    "github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

type Plugin struct {
    config Config
}

func New(config Config) plugin.Plugin {
    return &Plugin{config: config}
}

func (p *Plugin) Name() string {
    return "myplugin"
}

func (p *Plugin) Priority() int {
    return 15 // Lower number = earlier execution
}

func (p *Plugin) Evaluate(ctx context.Context, msg plugin.Message) (plugin.Result, error) {
    // Access message content
    text := msg.Text
    if text == "" {
        text = msg.Caption
    }

    // Your filtering logic here
    if shouldBlock(text) {
        return plugin.Result{
            Action:   plugin.ActionBlock,
            Reason:   "Custom violation detected",
            Metadata: map[string]any{
                "detail": "specific violation info",
                "confidence": 0.95,
            },
            Plugin: p.Name(),
        }, nil
    }
    
    return plugin.Result{
        Action:   plugin.ActionSkip,
        Reason:   "no violation detected",
        Metadata: nil,
        Plugin:   p.Name(),
    }, nil
}
```

### Step 2: Create Configuration Provider

Update [`internal/censor/module.go`](internal/censor/module.go) to provide plugin config:

```go
fx.Provide(
    func(config Config) (myplugin.Config, error) {
        configMap := map[string]any{}
        if v, ok := config.Plugins["myplugin"]; ok {
            configMap = v.Config
        }
        
        c, err := myplugin.NewConfig(configMap)
        if err != nil {
            return c, fmt.Errorf("failed to create myplugin config: %w", err)
        }
        
        return c, nil
    },
),
```

### Step 3: Register the Plugin

Update [`internal/censor/plugins/module.go`](internal/censor/plugins/module.go) to register your plugin:

```go
fx.Provide(
    fx.Annotate(myplugin.New, fx.ResultTags(`group:"plugins"`)),
),
```

### Step 4: Add Configuration to YAML

Add your plugin configuration to `config.yml`:

```yaml
censor:
  plugins:
    myplugin:
      enabled: true
      priority: 15
      config:
        threshold: 10
```

### Message Structure

The `plugin.Message` struct provides access to:

- `Text` - Message text content
- `Caption` - Media caption (for photos, videos, etc.)
- `UserID` - Telegram user ID
- `ChatID` - Telegram chat ID
- `MessageID` - Unique message identifier
- `IsEdit` - Whether this is an edited message

### Best Practices

1. **Use appropriate priority values:**
   - 1-5: Very high priority (rate limiting, simple checks)
   - 10-15: Medium priority (keyword matching)
   - 20+: Low priority (expensive operations like regex, ML models)

2. **Return ActionSkip when uncertain** - Let other plugins make the decision

3. **Include detailed metadata** - Helps with debugging and monitoring

4. **Handle errors gracefully** - Return errors for unexpected failures, not filtering decisions

5. **Test thoroughly** - Create unit tests in `myplugin_test.go`
