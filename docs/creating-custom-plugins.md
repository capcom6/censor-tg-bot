## Creating Custom Plugins

The plugin architecture allows you to create custom filtering logic by implementing the `Plugin` interface.

### Plugin Interface

All plugins must implement five methods:

```go
type Plugin interface {
    // Name returns the unique identifier for this plugin
    Name() string

    // Capabilities declares what the plugin inspects (text, media)
    Capabilities() Capability

    // Evaluate inspects a message and returns a decision
    Evaluate(ctx context.Context, msg Message) (Result, error)

    // Priority returns the execution priority (lower = earlier execution)
    Priority() int

    // Cleanup performs periodic maintenance (no-op when the app cache
    // sweeper handles eviction)
    Cleanup(ctx context.Context)
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

// Metadata registers the plugin with the wiring.
func Metadata() plugin.Metadata {
    return plugin.Metadata{
        Name: "myplugin",
        Factory: func(params map[string]any, deps plugin.Deps) (plugin.Plugin, error) {
            config, err := NewConfig(params)
            if err != nil {
                return nil, err
            }

            return New(config, deps)
        },
    }
}

// New creates the plugin from its config block and shared deps.
//
// deps.Cache is the app-scoped cache named after the plugin (guaranteed
// non-nil); deps.Logger is the module logger for non-fatal errors, such as
// cache failures that must degrade instead of failing Evaluate.
func New(config Config, deps plugin.Deps) (plugin.Plugin, error) {
    return &Plugin{config: config}, nil
}

func (p *Plugin) Name() string {
    return "myplugin"
}

func (p *Plugin) Capabilities() plugin.Capability {
    return plugin.CapabilityText
}

func (p *Plugin) Priority() int {
    return 15 // Lower number = earlier execution
}

func (p *Plugin) Cleanup(context.Context) {
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

### Step 2: Register the Plugin

Update [`internal/censor/plugins/module.go`](../internal/censor/plugins/module.go) to register your plugin metadata:

```go
fx.Provide(
    fx.Annotate(myplugin.Metadata, fx.ResultTags(`group:"metadata"`)),
),
```

The wiring builds every registered plugin from its YAML config block and the
shared `plugin.Deps` (the app-scoped cache named after the plugin, plus the
module logger), so no changes to `internal/censor/module.go` or the config
module are needed.

### Step 3: Add Configuration to YAML

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

1. **Use appropriate priority values** (defaults run from the lightweight plugin to the most expensive one):
   - 1-15: Trivial in-memory checks (user lists, forwards, rate limiting)
   - 20-30: Content matching (keywords, regex)
   - 100-200: Stateful checks backed by storage (duplicates)
   - 200+: External API calls (decisions, LLM)

2. **Return ActionSkip when uncertain** - Let other plugins make the decision

3. **Include detailed metadata** - Helps with debugging and monitoring

4. **Handle errors gracefully** - Return errors for unexpected failures, not filtering decisions

5. **Test thoroughly** - Create unit tests in `myplugin_test.go`
