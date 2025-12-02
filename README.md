# Censor Telegram Bot

A simple keyword-based antispam bot for Telegram, written in Go.

## Features

- Keyword-based message filtering
- Easy to set up and deploy
- Dockerized for easy deployment

## Prerequisites

- Go 1.24 (for building the binary)
- Docker (for containerized deployment)

## Configuration

The app uses environment variables to configure it. For example, `.env` file with the following content can be used:

```sh
TELEGRAM__TOKEN=xxx:yyyyy    # bot token from @BotFather
TELEGRAM__ADMIN_ID=123456789 # admin id
CENSOR__BLACKLIST='x,y,z'    # list of banned words, separated by comma
```

Replace placeholders with your actual values.

## Running

The simplest way to run the app is to use Docker.

```sh
docker run -d --name censor-tg-bot --env-file .env capcom6/censor-tg-bot
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

Distributed under the Apache-2.0 license. See [LICENSE](./LICENSE) for more information.
