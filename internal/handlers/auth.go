package handlers

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/dimitrije/nikode-api/internal/config"
	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/oauth"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type AuthHandler struct {
	cfg          *config.Config
	providers    map[string]oauth.Provider
	userService  UserServiceInterface
	tokenService TokenServiceInterface
	jwtService   JWTServiceInterface
	states       sync.Map
	authCodes    sync.Map
}

type stateData struct {
	expiresAt time.Time
}

type authCodeData struct {
	userID    uuid.UUID
	expiresAt time.Time
}

func NewAuthHandler(
	cfg *config.Config,
	userService UserServiceInterface,
	tokenService TokenServiceInterface,
	jwtService JWTServiceInterface,
) *AuthHandler {
	h := &AuthHandler{
		cfg:          cfg,
		providers:    make(map[string]oauth.Provider),
		userService:  userService,
		tokenService: tokenService,
		jwtService:   jwtService,
	}

	if cfg.GitHub.ClientID != "" {
		h.providers["github"] = oauth.NewGitHubProvider(cfg.GitHub)
	}
	if cfg.GitLab.ClientID != "" {
		h.providers["gitlab"] = oauth.NewGitLabProvider(cfg.GitLab)
	}
	if cfg.Google.ClientID != "" {
		h.providers["google"] = oauth.NewGoogleProvider(cfg.Google)
	}

	go h.cleanupStates()

	return h
}

func (h *AuthHandler) cleanupStates() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		now := time.Now()
		h.states.Range(func(key, value interface{}) bool {
			if sd, ok := value.(stateData); ok && now.After(sd.expiresAt) {
				h.states.Delete(key)
			}
			return true
		})
		h.authCodes.Range(func(key, value interface{}) bool {
			if acd, ok := value.(authCodeData); ok && now.After(acd.expiresAt) {
				h.authCodes.Delete(key)
			}
			return true
		})
	}
}

func (h *AuthHandler) GetConsentURL(c *drift.Context) {
	provider := c.Param("provider")

	p, ok := h.providers[provider]
	if !ok {
		c.BadRequest("unsupported provider: " + provider)
		return
	}

	state, err := oauth.GenerateState()
	if err != nil {
		c.InternalServerError("failed to generate state")
		return
	}

	h.states.Store(state, stateData{expiresAt: time.Now().Add(10 * time.Minute)})

	_ = c.JSON(200, dto.ConsentURLResponse{
		URL: p.GetConsentURL(state),
	})
}

func (h *AuthHandler) Callback(c *drift.Context) {
	provider := c.Param("provider")

	p, ok := h.providers[provider]
	if !ok {
		h.redirectWithError(c, "unsupported provider")
		return
	}

	state := c.QueryParam("state")
	if state == "" {
		h.redirectWithError(c, "missing state parameter")
		return
	}

	sd, ok := h.states.LoadAndDelete(state)
	if !ok {
		h.redirectWithError(c, "invalid or expired state")
		return
	}

	sdTyped, ok := sd.(stateData)
	if !ok || time.Now().After(sdTyped.expiresAt) {
		h.redirectWithError(c, "state expired")
		return
	}

	code := c.QueryParam("code")
	if code == "" {
		h.redirectWithError(c, "missing authorization code")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userInfo, err := p.ExchangeCode(ctx, code)
	if err != nil {
		h.redirectWithError(c, "failed to exchange code: "+err.Error())
		return
	}

	user, err := h.userService.FindOrCreateFromOAuth(ctx, userInfo)
	if err != nil {
		h.redirectWithError(c, "failed to create user")
		return
	}

	authCode, err := oauth.GenerateState()
	if err != nil {
		h.redirectWithError(c, "failed to generate auth code")
		return
	}

	h.authCodes.Store(authCode, authCodeData{
		userID:    user.ID,
		expiresAt: time.Now().Add(30 * time.Second),
	})

	redirectURL := fmt.Sprintf("%s?code=%s",
		h.cfg.FrontendCallbackURL,
		url.QueryEscape(authCode),
	)

	h.renderCallbackPage(c, redirectURL, authCode, "")
}

func (h *AuthHandler) ExchangeCode(c *drift.Context) {
	var req dto.ExchangeCodeRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Code == "" {
		c.BadRequest("code is required")
		return
	}

	acd, ok := h.authCodes.LoadAndDelete(req.Code)
	if !ok {
		c.Unauthorized("invalid or expired code")
		return
	}

	codeData, ok := acd.(authCodeData)
	if !ok || time.Now().After(codeData.expiresAt) {
		c.Unauthorized("code expired")
		return
	}

	ctx := context.Background()

	user, err := h.userService.GetByID(ctx, codeData.userID)
	if err != nil {
		c.Unauthorized("user not found")
		return
	}

	tokenPair, err := h.jwtService.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		c.InternalServerError("failed to generate tokens")
		return
	}

	tokenHash := services.HashToken(tokenPair.RefreshToken)
	expiresAt := time.Now().Add(h.jwtService.RefreshExpiry())
	if err := h.tokenService.StoreRefreshToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		c.InternalServerError("failed to store refresh token")
		return
	}

	_ = c.JSON(200, dto.TokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	})
}

func (h *AuthHandler) RefreshToken(c *drift.Context) {
	var req dto.RefreshTokenRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.RefreshToken == "" {
		c.BadRequest("refresh_token is required")
		return
	}

	userID, err := h.jwtService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.Unauthorized("invalid refresh token")
		return
	}

	tokenHash := services.HashToken(req.RefreshToken)
	ctx := context.Background()

	storedUserID, err := h.tokenService.ValidateRefreshToken(ctx, tokenHash)
	if err != nil || storedUserID != userID {
		c.Unauthorized("refresh token not found or expired")
		return
	}

	user, err := h.userService.GetByID(ctx, userID)
	if err != nil {
		c.Unauthorized("user not found")
		return
	}

	if err := h.tokenService.RevokeRefreshToken(ctx, tokenHash); err != nil {
		c.InternalServerError("failed to revoke old token")
		return
	}

	tokenPair, err := h.jwtService.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		c.InternalServerError("failed to generate tokens")
		return
	}

	newTokenHash := services.HashToken(tokenPair.RefreshToken)
	expiresAt := time.Now().Add(h.jwtService.RefreshExpiry())
	if err := h.tokenService.StoreRefreshToken(ctx, user.ID, newTokenHash, expiresAt); err != nil {
		c.InternalServerError("failed to store refresh token")
		return
	}

	_ = c.JSON(200, dto.TokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	})
}

func (h *AuthHandler) Logout(c *drift.Context) {
	var req dto.RefreshTokenRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.RefreshToken != "" {
		tokenHash := services.HashToken(req.RefreshToken)
		_ = h.tokenService.RevokeRefreshToken(context.Background(), tokenHash)
	}

	_ = c.JSON(200, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) LogoutAll(c *drift.Context) {
	userID := middleware.GetUserID(c)
	if userID == (uuid.UUID{}) {
		c.Unauthorized("not authenticated")
		return
	}

	if err := h.tokenService.RevokeAllUserTokens(context.Background(), userID); err != nil {
		c.InternalServerError("failed to revoke tokens")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "all sessions logged out"})
}

func (h *AuthHandler) redirectWithError(c *drift.Context, errMsg string) {
	redirectURL := fmt.Sprintf("%s?error=%s",
		h.cfg.FrontendCallbackURL,
		url.QueryEscape(errMsg),
	)
	h.renderCallbackPage(c, redirectURL, errMsg, "error")
}

func (h *AuthHandler) renderCallbackPage(c *drift.Context, deepLink, code, status string) {
	title := "Sign-in Successful"
	heading := "You're signed in!"
	subtitle := "Redirecting you to Nikode..."
	headingColor := "#111827"
	statusCode := 200
	codeSection := ""

	if status == "error" {
		title = "Sign-in Failed"
		heading = "Sign-in failed"
		subtitle = code
		headingColor = "#991b1b"
		statusCode = 400
	} else {
		codeSection = fmt.Sprintf(`
        <div class="divider"></div>
        <p class="fallback-hint">Didn't redirect automatically? Copy the code below and paste it in the Nikode app.</p>
        <div class="code-container">
            <code id="auth-code">%s</code>
            <button onclick="copyCode()" class="copy-btn" id="copy-btn">Copy</button>
        </div>`, code)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        * { box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; background: #f9fafb; color: #374151; margin: 0; padding: 40px 20px; min-height: 100vh; }
        .container { max-width: 400px; margin: 0 auto; background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 40px 32px; text-align: center; }
        .icon { margin-bottom: 24px; }
        .icon svg { width: 48px; height: 48px; }
        h1 { font-size: 20px; font-weight: 600; color: %s; margin: 0 0 8px 0; }
        .subtitle { color: #6b7280; font-size: 14px; margin: 0 0 4px 0; }
        .close-hint { color: #9ca3af; font-size: 13px; margin: 0; }
        .divider { border-top: 1px solid #e5e7eb; margin: 24px 0; }
        .fallback-hint { color: #6b7280; font-size: 13px; margin: 0 0 12px 0; }
        .code-container { display: flex; align-items: center; background: #f3f4f6; border: 1px solid #e5e7eb; border-radius: 6px; padding: 8px 12px; gap: 8px; }
        .code-container code { flex: 1; font-family: monospace; font-size: 13px; color: #111827; word-break: break-all; text-align: left; }
        .copy-btn { background: #374151; color: #fff; border: none; border-radius: 4px; padding: 6px 12px; font-size: 12px; font-weight: 500; cursor: pointer; white-space: nowrap; }
        .copy-btn:hover { background: #1f2937; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">
            <svg width="512" height="512" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg">
                <rect x="0" y="0" width="512" height="512" rx="80" ry="80" fill="#374151"/>
                <text x="256" y="380" font-family="Arial, Helvetica, sans-serif" font-size="360" font-weight="bold" fill="#f3f4f6" text-anchor="middle">N</text>
            </svg>
        </div>
        <h1>%s</h1>
        <p class="subtitle">%s</p>
        <p class="close-hint">You can close this window.</p>%s
    </div>
    <script>
        window.location.href = %q;
        function copyCode() {
            var code = document.getElementById('auth-code').textContent;
            navigator.clipboard.writeText(code).then(function() {
                document.getElementById('copy-btn').textContent = 'Copied!';
                setTimeout(function() { document.getElementById('copy-btn').textContent = 'Copy'; }, 2000);
            });
        }
    </script>
</body>
</html>`, title, headingColor, heading, subtitle, codeSection, deepLink)

	_ = c.HTML(statusCode, html)
}
