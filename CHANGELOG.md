# Changelog

All notable changes to this project will be documented in this file.

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
