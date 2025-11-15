package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/middleware"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		cfg: cfg,
	}
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

type AuthUser struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      *AuthUser `json:"user"`
}

// Login initiates the OAuth flow
func (h *AuthHandler) Login(c echo.Context) error {
	state := generateState()

	// Store state in a cookie for verification in callback
	c.SetCookie(&http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	url := h.cfg.OAuth.GetGoogleOAuthConfig().AuthCodeURL(state)
	return c.JSON(http.StatusOK, map[string]string{
		"url": url,
	})
}

// Callback handles the OAuth callback
func (h *AuthHandler) Callback(c echo.Context) error {
	code := c.QueryParam("code")

	oauthConfig := h.cfg.OAuth.GetGoogleOAuthConfig()
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to exchange token: %v", err))
	}

	// Get user info from Google
	client := oauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get user info: %v", err))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to read response: %v", err))
	}

	var googleUser GoogleUserInfo
	if err := json.Unmarshal(data, &googleUser); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to parse user info: %v", err))
	}

	// Check if email matches MAIL_USERNAME
	if googleUser.Email != h.cfg.MailConfig.Username {
		return echo.NewHTTPError(http.StatusForbidden, "unauthorized email address")
	}

	// Generate JWT token
	jwtToken, expiresAt, err := h.generateJWT(googleUser.Email, googleUser.Name)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to generate token: %v", err))
	}

	return c.JSON(http.StatusOK, &AuthResponse{
		Token:     jwtToken,
		ExpiresAt: expiresAt,
		User: &AuthUser{
			Email: googleUser.Email,
			Name:  googleUser.Name,
		},
	})
}

// Me returns the current authenticated user
func (h *AuthHandler) Me(c echo.Context) error {
	email, ok := c.Get("email").(string)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user context")
	}

	name, ok := c.Get("name").(string)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user context")
	}

	return c.JSON(http.StatusOK, &AuthUser{
		Email: email,
		Name:  name,
	})
}

// Refresh generates a new JWT token for authenticated users
func (h *AuthHandler) Refresh(c echo.Context) error {
	email, ok := c.Get("email").(string)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user context")
	}

	name, ok := c.Get("name").(string)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user context")
	}

	// Generate new JWT token
	jwtToken, expiresAt, err := h.generateJWT(email, name)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to generate token: %v", err))
	}

	return c.JSON(http.StatusOK, &AuthResponse{
		Token:     jwtToken,
		ExpiresAt: expiresAt,
		User: &AuthUser{
			Email: email,
			Name:  name,
		},
	})
}

func (h *AuthHandler) generateJWT(email, name string) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour)

	claims := &middleware.JWTClaims{
		Email: email,
		Name:  name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.cfg.OAuth.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
