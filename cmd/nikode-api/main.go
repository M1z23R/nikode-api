package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dimitrije/nikode-api/internal/config"
	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/handlers"
	authmw "github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/internal/hub"
	"github.com/m1z23r/drift/pkg/drift"
	"github.com/m1z23r/drift/pkg/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	jwtService := services.NewJWTService(cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)
	userService := services.NewUserService(db)
	tokenService := services.NewTokenService(db)
	workspaceService := services.NewWorkspaceService(db)
	collectionService := services.NewCollectionService(db)
	emailService := services.NewEmailService(cfg.SMTP)
	apiKeyService := services.NewAPIKeyService(db)
	vaultService := services.NewVaultService(db)
	openAPIService := services.NewOpenAPIService()

	h := hub.NewHub()
	go h.Run()

	authHandler := handlers.NewAuthHandler(cfg, userService, tokenService, jwtService)
	userHandler := handlers.NewUserHandler(userService)
	workspaceHandler := handlers.NewWorkspaceHandler(workspaceService, userService, emailService, h, cfg.BaseURL)
	collectionHandler := handlers.NewCollectionHandler(collectionService, workspaceService, h)
	inviteHandler := handlers.NewInviteHandler(workspaceService, h)
	pingPongHandler := handlers.NewWebSocketHandler()
	syncHandler := handlers.NewSyncHandler(h, workspaceService, userService, jwtService)
	apiKeyHandler := handlers.NewAPIKeyHandler(apiKeyService, workspaceService)
	vaultHandler := handlers.NewVaultHandler(vaultService, workspaceService)
	automationHandler := handlers.NewAutomationHandler(collectionService, openAPIService)

	app := drift.New()

	if cfg.IsProduction() {
		app.SetMode(drift.ReleaseMode)
	} else {
		app.SetMode(drift.DebugMode)
	}

	app.Use(middleware.Recovery())
	app.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		MaxAge:       86400,
	}))
	app.Use(middleware.BodyParser())

	api := app.Group("/api/v1")

	auth := api.Group("/auth")
	auth.Get("/:provider/consent", authHandler.GetConsentURL)
	auth.Get("/:provider/callback", authHandler.Callback)
	auth.Post("/exchange", authHandler.ExchangeCode)
	auth.Post("/refresh", authHandler.RefreshToken)
	auth.Post("/logout", authHandler.Logout)

	protected := api.Group("")
	protected.Use(authmw.Auth(jwtService))

	protected.Post("/auth/logout-all", authHandler.LogoutAll)

	protected.Get("/users/me", userHandler.GetMe)
	protected.Patch("/users/me", userHandler.UpdateMe)

	protected.Get("/workspaces", workspaceHandler.List)
	protected.Post("/workspaces", workspaceHandler.Create)
	protected.Get("/workspaces/:workspaceId", workspaceHandler.Get)
	protected.Patch("/workspaces/:workspaceId", workspaceHandler.Update)
	protected.Delete("/workspaces/:workspaceId", workspaceHandler.Delete)
	protected.Get("/workspaces/:workspaceId/members", workspaceHandler.GetMembers)
	protected.Post("/workspaces/:workspaceId/members", workspaceHandler.InviteMember)
	protected.Delete("/workspaces/:workspaceId/members/:memberId", workspaceHandler.RemoveMember)
	protected.Post("/workspaces/:workspaceId/leave", workspaceHandler.LeaveWorkspace)
	protected.Get("/workspaces/:workspaceId/invites", workspaceHandler.GetWorkspaceInvites)
	protected.Delete("/workspaces/:workspaceId/invites/:inviteId", workspaceHandler.CancelInvite)

	protected.Get("/invites", workspaceHandler.GetMyInvites)
	protected.Post("/invites/:inviteId/accept", workspaceHandler.AcceptInvite)
	protected.Post("/invites/:inviteId/decline", workspaceHandler.DeclineInvite)

	protected.Get("/workspaces/:workspaceId/collections", collectionHandler.List)
	protected.Post("/workspaces/:workspaceId/collections", collectionHandler.Create)
	protected.Get("/workspaces/:workspaceId/collections/:collectionId", collectionHandler.Get)
	protected.Patch("/workspaces/:workspaceId/collections/:collectionId", collectionHandler.Update)
	protected.Delete("/workspaces/:workspaceId/collections/:collectionId", collectionHandler.Delete)

	// API Key management (owner only)
	protected.Post("/workspaces/:workspaceId/api-keys", apiKeyHandler.Create)
	protected.Get("/workspaces/:workspaceId/api-keys", apiKeyHandler.List)
	protected.Delete("/workspaces/:workspaceId/api-keys/:keyId", apiKeyHandler.Revoke)

	// Vault (zero-knowledge encrypted vault per workspace)
	protected.Post("/workspaces/:workspaceId/vault", vaultHandler.CreateVault)
	protected.Get("/workspaces/:workspaceId/vault", vaultHandler.GetVault)
	protected.Delete("/workspaces/:workspaceId/vault", vaultHandler.DeleteVault)
	protected.Get("/workspaces/:workspaceId/vault/items", vaultHandler.ListItems)
	protected.Post("/workspaces/:workspaceId/vault/items", vaultHandler.CreateItem)
	protected.Patch("/workspaces/:workspaceId/vault/items/:itemId", vaultHandler.UpdateItem)
	protected.Delete("/workspaces/:workspaceId/vault/items/:itemId", vaultHandler.DeleteItem)

	// Automation endpoints (API key auth)
	automation := api.Group("/automation")
	automation.Use(authmw.APIKeyAuth(apiKeyService))
	automation.Put("/collections", automationHandler.UpsertCollection)

	api.Get("/health", func(c *drift.Context) {
		_ = c.JSON(200, map[string]string{"status": "ok"})
	})

	api.Get("/ws", pingPongHandler.Connect)
	api.Get("/sync", syncHandler.Connect)

	// Public invite pages (no auth required)
	app.Get("/invite/:inviteId", inviteHandler.ViewInvite)
	app.Post("/invite/:inviteId/accept", inviteHandler.AcceptInvite)
	app.Post("/invite/:inviteId/decline", inviteHandler.DeclineInvite)

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			_ = tokenService.CleanupExpired(context.Background())
		}
	}()

	go func() {
		addr := fmt.Sprintf(":%s", cfg.Port)
		log.Printf("Server starting on %s", addr)
		if err := app.Run(addr); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
