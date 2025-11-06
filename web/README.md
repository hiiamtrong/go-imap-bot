# Bill Splitter - Expense Tracking Web App

A modern web application built with SvelteKit for tracking expenses, splitting bills, and managing shared payments. This is the frontend interface for the go-imap-bot backend system.

## Features

### 🎯 Core Functionality
- **Dashboard** - Overview of transactions, balance, and pending splits
- **Transaction Management** - View and filter all transactions
- **Bill Splitting** - Split expenses equally or with custom amounts
- **Payment Reminders** - Send email reminders with QR payment codes
- **User Management** - Manage users who can be added to bill splits
- **Tag System** - Categorize transactions for better organization
- **Statistics** - View spending analytics with charts and breakdowns

### 💡 Key Features
- Automatic transaction tracking from bank emails (via backend)
- Virtual bill creation for manual expenses
- Flexible split options (equal or custom amounts per user)
- Payment tracking and completion status
- Email reminders (normal and "angry" mode)
- VietQR payment code integration
- User whitelist for skipping reminders
- Monthly and category-based spending analysis
- Responsive design with Tailwind CSS

## Tech Stack

- **Framework:** SvelteKit (Svelte 5 with Runes)
- **Language:** TypeScript
- **Styling:** Tailwind CSS
- **Charts:** Chart.js
- **Date Handling:** date-fns
- **QR Codes:** qrcode library

## Getting Started

### Prerequisites

- Node.js 18+ and npm
- Backend API running (go-imap-bot)

### Installation

1. Install dependencies:
```bash
npm install
```

2. Configure the API endpoint:

Create a `.env` file in the project root:
```env
VITE_API_URL=http://localhost:8080/api
```

3. Start the development server:
```bash
npm run dev
```

The app will be available at `http://localhost:5173`

### Building for Production

```bash
npm run build
```

Preview the production build:
```bash
npm run preview
```

## Project Structure

```
web/
├── src/
│   ├── lib/
│   │   ├── api/           # API client
│   │   ├── components/    # Reusable components
│   │   ├── stores/        # Svelte stores
│   │   ├── types/         # TypeScript types
│   │   └── utils/         # Utility functions
│   ├── routes/            # SvelteKit routes (pages)
│   │   ├── transactions/  # Transaction pages
│   │   ├── users/         # User management
│   │   ├── tags/          # Tag management
│   │   ├── statistics/    # Analytics page
│   │   ├── reminders/     # Reminder system
│   │   └── settings/      # Settings page
│   ├── app.css           # Global styles
│   ├── app.html          # HTML template
│   └── app.d.ts          # Type declarations
├── static/               # Static assets
└── package.json
```

## Pages Overview

### Dashboard (`/`)
- Current balance display
- Quick stats (total transactions, pending/completed splits)
- Recent transactions list
- Quick access to create virtual bills

### Transactions (`/transactions`)
- Full transaction list with filtering
- Search by description
- Filter by type (income/expense)
- View transaction details

### Transaction Details (`/transactions/:id`)
- Full transaction information
- Add/remove tags
- Create bill splits
- View split status
- Mark splits as paid

### Users (`/users`)
- Add/edit/delete users
- Manage whitelist status
- View user information

### Tags (`/tags`)
- Create new tags
- View all available tags
- Categorize transactions

### Statistics (`/statistics`)
- Overview cards (spent, received, balance)
- Monthly spending chart
- Category breakdown (pie chart)
- Detailed tables

### Reminders (`/reminders`)
- View all pending payments
- Select users to remind
- Send normal or "angry" reminders
- Bulk reminder sending

### Settings (`/settings`)
- Configure API endpoint
- View application info

## API Integration

The app communicates with the backend API at the configured `VITE_API_URL`.

### API Endpoints Used

- `GET /transactions` - List transactions
- `GET /transactions/:id` - Get transaction details
- `POST /transactions/virtual` - Create virtual bill
- `GET /users` - List users
- `POST /users` - Create user
- `PUT /users/:id` - Update user
- `DELETE /users/:id` - Delete user
- `GET /tags` - List tags
- `POST /tags` - Create tag
- `POST /transactions/:id/tags/:tagId` - Add tag to transaction
- `DELETE /transactions/:id/tags/:tagId` - Remove tag
- `POST /splits` - Create bill split
- `POST /splits/:id/complete` - Mark split as paid
- `DELETE /splits/:id` - Delete split
- `POST /reminders` - Send payment reminders
- `GET /statistics` - Get spending statistics

## Configuration

### Environment Variables

- `VITE_API_URL` - Backend API base URL (default: `http://localhost:8080/api`)

### Settings Page

Users can also configure the API URL directly in the Settings page, which saves to browser localStorage.

## Development

### Code Style

- TypeScript with strict mode
- Svelte 5 Runes API (`$state`, `$derived`, `$effect`)
- Tailwind CSS for styling
- Component-based architecture

### Key Files

- `src/lib/api/client.ts` - API client singleton
- `src/lib/types/index.ts` - TypeScript type definitions
- `src/lib/stores/*.ts` - Global state management
- `src/lib/utils/format.ts` - Formatting utilities
- `src/app.css` - Global styles and Tailwind utilities

## Features in Detail

### Bill Splitting

1. Navigate to a transaction
2. Click "Split Bill"
3. Select users to split among
4. Choose split mode:
   - **Equal Split** - Divides amount equally
   - **Custom Amounts** - Set specific amounts per user
5. Add optional reasons for each split
6. Confirm and create

### Payment Reminders

1. Go to Reminders page
2. View all pending payments
3. Select splits to remind
4. Choose reminder type (normal/angry)
5. Send reminders

Reminders include:
- Professional email format
- VietQR payment codes
- Payment details and reasons

### Transaction Tagging

1. Open transaction details
2. Click "Add Tag"
3. Select existing tag or create new
4. Tags help categorize expenses for statistics

## License

MIT License - See LICENSE file in the root directory
