package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dimitrije/nikode-api/internal/config"
	"github.com/dimitrije/nikode-api/internal/database"
	"github.com/dimitrije/nikode-api/internal/models"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: promote-admin <email>")
		os.Exit(1)
	}

	email := os.Args[1]

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

	result, err := db.Pool.Exec(ctx, `
		UPDATE users SET global_role = $1, updated_at = NOW()
		WHERE email = $2
	`, models.GlobalRoleSuperAdmin, email)
	if err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		log.Fatalf("No user found with email: %s", email)
	}

	fmt.Printf("Successfully promoted %s to super admin\n", email)
}
