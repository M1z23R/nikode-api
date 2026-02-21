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
		h.states.Range(func(key, value any) bool {
			if sd, ok := value.(stateData); ok && now.After(sd.expiresAt) {
				h.states.Delete(key)
			}
			return true
		})
		h.authCodes.Range(func(key, value any) bool {
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

	c.Redirect(302, redirectURL)
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
	c.Redirect(302, redirectURL)
}
