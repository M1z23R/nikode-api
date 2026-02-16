package middleware

import (
	"strings"

	"github.com/m1z23r/drift/pkg/drift"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/google/uuid"
)

const (
	UserIDKey    = "user_id"
	UserEmailKey = "user_email"
)

func Auth(jwtService *services.JWTService) drift.HandlerFunc {
	return func(c *drift.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Unauthorized("missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Unauthorized("invalid authorization header format")
			return
		}

		claims, err := jwtService.ValidateAccessToken(parts[1])
		if err != nil {
			c.Unauthorized("invalid or expired token")
			return
		}

		c.Set(UserIDKey, claims.UserID)
		c.Set(UserEmailKey, claims.Email)

		c.Next()
	}
}

func GetUserID(c *drift.Context) uuid.UUID {
	if id, ok := c.Get(UserIDKey); ok {
		if uid, ok := id.(uuid.UUID); ok {
			return uid
		}
	}
	return uuid.Nil
}

func GetUserEmail(c *drift.Context) string {
	if email, ok := c.Get(UserEmailKey); ok {
		if e, ok := email.(string); ok {
			return e
		}
	}
	return ""
}
