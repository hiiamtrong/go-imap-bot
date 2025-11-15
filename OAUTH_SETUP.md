# Google OAuth Setup & Implementation Guide

## ✅ Implementation Complete

Google OAuth authentication has been successfully integrated into your application!

## 🔧 Setup Instructions

### 1. Google Cloud Console Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the **Google+ API**
4. Go to **Credentials** → **Create Credentials** → **OAuth 2.0 Client ID**
5. Configure the OAuth consent screen if prompted
6. Add **Authorized redirect URIs**:
   - Development: `http://localhost:5173/auth/callback`
   - Production: `https://your-domain.com/auth/callback`
7. Copy the **Client ID** and **Client Secret**

### 2. Environment Variables

Add these to your `.env` file (see `.env.oauth.example`):

```bash
# OAuth Configuration
GOOGLE_CLIENT_ID=your-google-client-id-here
GOOGLE_CLIENT_SECRET=your-google-client-secret-here
GOOGLE_REDIRECT_URL=http://localhost:5173/auth/callback
JWT_SECRET=your-secret-jwt-key-change-this-in-production

# Generate a secure JWT secret:
# openssl rand -base64 32
```

### 3. Update Frontend API URL

In `web/.env`:
```bash
VITE_API_URL=http://localhost:8080
```

### 4. Access Control

Only the email address configured in `MAIL_USERNAME` can access the application. This ensures single-user access control without requiring database configuration.

Make sure your `.env` file has:
```bash
MAIL_USERNAME=your-email@gmail.com
```

This email must match your Google account email for successful authentication.

## 🏗️ Architecture

### Backend (Go)
- **OAuth Config** (`internal/config/oauth_config.go`): Google OAuth2 configuration
- **Auth Handler** (`internal/api/handlers/auth_handler.go`): Login, callback, user info endpoints (no database dependencies)
- **JWT Middleware** (`internal/middleware/auth.go`): Token validation
- **Protected Routes**: All API endpoints now require JWT authentication
- **Access Control**: Email validation against `MAIL_USERNAME` environment variable

### Frontend (SvelteKit)
- **Auth Store** (`web/src/lib/stores/auth.ts`): Authentication state management
- **Login Page** (`web/src/routes/login/+page.svelte`): Google OAuth button
- **Callback Page** (`web/src/routes/auth/callback/+page.svelte`): OAuth flow completion
- **API Client** (`web/src/lib/api/client.ts`): Automatic JWT token injection
- **Protected Routes**: Redirects unauthenticated users to login

## 🔒 Security Features

- ✅ JWT-based authentication
- ✅ Token expiration (24 hours)
- ✅ Single-user access control via `MAIL_USERNAME`
- ✅ Secure HTTP-only cookies for OAuth state
- ✅ Automatic token refresh
- ✅ Protected API endpoints
- ✅ No database dependency for authentication

## 📝 API Endpoints

### Public Endpoints
- `GET /auth/login` - Initiates OAuth flow
- `GET /auth/callback` - OAuth callback handler

### Protected Endpoints (require JWT)
- `GET /api/auth/me` - Get current user
- `POST /api/auth/refresh` - Refresh JWT token
- All other `/api/*` endpoints

## 🚀 Usage

### Login Flow
1. User clicks "Sign in with Google" on `/login`
2. Redirected to Google OAuth consent screen
3. After approval, redirected to `/auth/callback`
4. JWT token generated and stored in localStorage
5. User redirected to dashboard

### API Requests
All API requests automatically include the JWT token:
```typescript
const response = await api.getTransactions();
// Authorization header automatically added
```

### Logout
```typescript
auth.logout(); // Clears token and redirects to login
```

## 🐛 Troubleshooting

### "Unauthorized email address" error
- Ensure the Google account email matches the `MAIL_USERNAME` environment variable exactly
- Check your `.env` file for the correct email address

### Redirect URI mismatch
- Ensure `GOOGLE_REDIRECT_URL` matches exactly what's configured in Google Console
- Include the protocol (`http://` or `https://`)

### Token expired
- Tokens expire after 24 hours
- Use the refresh endpoint or login again

## 📦 Docker Deployment

OAuth environment variables are already configured in `docker-compose.yml`. Just set them in your `.env` file before running:

```bash
docker-compose up -d
```

## 🎉 What's Next?

- Configure your Google OAuth credentials
- Set `MAIL_USERNAME` to your Google account email
- Test the login flow
- Deploy to production with HTTPS
- Update production redirect URLs

## 📌 Important Notes

### Authentication vs. Bill Users

This application separates authentication from bill tracking:

- **Authentication**: Uses Google OAuth and validates against `MAIL_USERNAME` only (no database involved)
- **Bill Users**: The `users` table stores people who owe money on bills (separate from authentication)
- **JWT Token**: Contains only `email` and `name` (no user ID from database)

The authenticated user can manage bills for any users in the `users` table, but only the owner (matching `MAIL_USERNAME`) can log in to the application.

Happy coding! 🚀
