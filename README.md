# IMAP Bot Go

A Telegram bot that monitors email inboxes for bank transactions, automates bill splitting, and manages expense tracking.

## Project Description

IMAP Bot Go is a robust Golang application that connects to your email inbox via IMAP, monitors for bank transaction notifications, and provides a convenient Telegram bot interface for managing and splitting expenses. It automatically parses transaction details from bank emails, allows users to split bills among friends, send reminders for pending payments, and track transaction history.

### Key Features:
- Email inbox monitoring for transaction notifications
- Automated transaction parsing and categorization
- Bill splitting functionality with multiple users
- Payment reminders with customizable templates (including "angry" reminders!)
- Transaction tagging and organization
- Manual bill creation and management
- Transaction history and statistics

This project uses Go for its performance and concurrency capabilities, SQLite for data storage, and the Telegram Bot API for user interaction. The IMAP protocol enables secure email monitoring without requiring email forwarding or third-party access to your accounts.

## Table of Contents
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Features](#features)
- [Contributing](#contributing)
- [License](#license)

## Installation

### Prerequisites
- Go 1.16 or higher
- SQLite
- A Telegram Bot Token (obtained from BotFather)
- Email account with IMAP access enabled

### Steps

1. Clone the repository:
```bash
git clone https://github.com/hiiamtrong/go-imap-bot.git
cd go-imap-bot
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o go-imap-bot cmd/bot/main.go
```

## Configuration

1. Create a `.env` file in the project root with the following variables:
```
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
IMAP_SERVER=imap.example.com
IMAP_PORT=993
IMAP_USERNAME=your_email@example.com
IMAP_PASSWORD=your_email_password
DATABASE_PATH=./data/bot.db
```

## Usage

### Running the Bot

```bash
./go-imap-bot
```

### Bot Commands

- `/start` - Begin interaction with the bot and register your email
- `/addbill` - Create a virtual bill manually
- `/transactions` - View recent transactions
- `/stats` - View transaction statistics
- `/adduser` - Add a new user to the system
- `/addtag` - Create a new transaction tag
- `/remind` - Send reminders to users who owe money
- `/donebillmanual` - Manually mark bills as completed for selected users
- `/settings` - Configure bot settings
- `/cancel` - Cancel the current operation

## Features

### Transaction Monitoring

The bot monitors your email inbox for bank transaction notifications, automatically parses the amount, description, and other details, and stores them for easy access.

### Bill Splitting

When you make a payment, you can split the bill among multiple users. The bot will track who owes what and allow you to send reminders to those who haven't paid.

Split bills in two ways:
- Custom amounts per user
- Equal splits among selected users

### Payment Reminders

Send payment reminders to users with pending bills:
- Standard reminders
- "Angry" reminders with a more assertive template
- Manual completion of bills for users who have paid

### Tagging System

Organize your transactions with tags for better categorization and reporting.

### Virtual Bills

Create virtual transactions for expenses that don't have a corresponding bank notification.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

---

Created with ❤️ by [Trong Vu](https://github.com/hiiamtrong) 