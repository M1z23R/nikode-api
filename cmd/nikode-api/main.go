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
	"github.com/dimitrije/nikode-api/internal/sse"
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
	teamService := services.NewTeamService(db)
	workspaceService := services.NewWorkspaceService(db, teamService)
	collectionService := services.NewCollectionService(db)
	emailService := services.NewEmailService(cfg.SMTP)

	hub := sse.NewHub()
	go hub.Run()

	authHandler := handlers.NewAuthHandler(cfg, userService, tokenService, jwtService)
	userHandler := handlers.NewUserHandler(userService)
	teamHandler := handlers.NewTeamHandler(teamService, userService, emailService, cfg.BaseURL)
	workspaceHandler := handlers.NewWorkspaceHandler(workspaceService, teamService)
	collectionHandler := handlers.NewCollectionHandler(collectionService, workspaceService, hub)
	sseHandler := handlers.NewSSEHandler(hub, workspaceService)
	inviteHandler := handlers.NewInviteHandler(teamService, userService)
	wsHandler := handlers.NewWebSocketHandler()

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

	protected.Get("/teams", teamHandler.List)
	protected.Post("/teams", teamHandler.Create)
	protected.Get("/teams/:id", teamHandler.Get)
	protected.Patch("/teams/:id", teamHandler.Update)
	protected.Delete("/teams/:id", teamHandler.Delete)
	protected.Get("/teams/:id/members", teamHandler.GetMembers)
	protected.Post("/teams/:id/members", teamHandler.InviteMember)
	protected.Delete("/teams/:id/members/:memberId", teamHandler.RemoveMember)
	protected.Post("/teams/:id/leave", teamHandler.LeaveTeam)
	protected.Get("/teams/:id/invites", teamHandler.GetTeamInvites)
	protected.Delete("/teams/:id/invites/:inviteId", teamHandler.CancelInvite)

	protected.Get("/invites", teamHandler.GetMyInvites)
	protected.Post("/invites/:inviteId/accept", teamHandler.AcceptInvite)
	protected.Post("/invites/:inviteId/decline", teamHandler.DeclineInvite)

	protected.Get("/workspaces", workspaceHandler.List)
	protected.Post("/workspaces", workspaceHandler.Create)
	protected.Get("/workspaces/:workspaceId", workspaceHandler.Get)
	protected.Patch("/workspaces/:workspaceId", workspaceHandler.Update)
	protected.Delete("/workspaces/:workspaceId", workspaceHandler.Delete)

	protected.Get("/workspaces/:workspaceId/collections", collectionHandler.List)
	protected.Post("/workspaces/:workspaceId/collections", collectionHandler.Create)
	protected.Get("/workspaces/:workspaceId/collections/:collectionId", collectionHandler.Get)
	protected.Patch("/workspaces/:workspaceId/collections/:collectionId", collectionHandler.Update)
	protected.Delete("/workspaces/:workspaceId/collections/:collectionId", collectionHandler.Delete)

	protected.Get("/workspaces/:workspaceId/events", sseHandler.Connect)
	protected.Post("/sse/:clientId/subscribe/:workspaceId", sseHandler.Subscribe)
	protected.Post("/sse/:clientId/unsubscribe/:workspaceId", sseHandler.Unsubscribe)

	api.Get("/health", func(c *drift.Context) {
		_ = c.JSON(200, map[string]string{"status": "ok"})
	})

	api.Get("/ws", wsHandler.Connect)

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
