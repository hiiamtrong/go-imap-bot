# Docker Setup

## Prerequisites

Before running docker-compose, install the static adapter for SvelteKit:

```bash
cd web
npm install -D @sveltejs/adapter-static
cd ..
```

## Environment Variables

Create a `.env` file in the root directory with the following variables:

```bash
# Mail Configuration
MAIL_SERVER=imap.gmail.com:993
MAIL_USERNAME=your_email@gmail.com
MAIL_PASSWORD=your_app_password
MAIL_MAILBOX=[Gmail]/All Mail
MAIL_FROM=your_email@gmail.com
MAIL_HOST=smtp.gmail.com
MAIL_PORT=587

# VietQR Configuration
VIETQR_CLIENT_ID=your_client_id
VIETQR_API_KEY=your_api_key
VIETQR_BANK_ID=963388
VIETQR_ACCOUNT_NO=your_account_number
VIETQR_ACCOUNT_NAME=YOUR NAME
VIETQR_TEMPLATE=print

# Telegram Bot
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_BOT_API_ENDPOINT=https://api.telegram.org
TELEGRAM_BOT_CHAT_ID=your_chat_id

# AWS Configuration
AWS_PROFILE=imap-bot
AWS_REGION=ap-southeast-1
AWS_S3_BUCKET=imap-bot-qr-code

# Security
CIPHER_SECRET_KEY=your_secret_key

# Web API URL (for production)
VITE_API_URL=http://localhost:8080/api
```

## Build and Run

### Build and start all services:

```bash
docker-compose up -d --build
```

### View logs:

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f bot
docker-compose logs -f api
docker-compose logs -f web
```

### Stop services:

```bash
docker-compose down
```

## Services

- **bot**: IMAP bot service (Telegram bot)
- **api**: REST API server (port 8080)
- **web**: Web frontend (port 5173)

## Access

- Web UI: http://localhost:5173
- API: http://localhost:8080/api

## Data Persistence

The SQLite database is stored in `./data/mail.db` and is shared between bot and api services.

## Development vs Production

For development, you can run services individually:

```bash
# Bot
go run cmd/bot/main.go

# API
go run cmd/server/main.go

# Web
cd web && npm run dev
```

For production with Docker:

```bash
docker-compose up -d --build
```
