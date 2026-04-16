package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kubevalet/kubevalet/internal/auth"
)

const ctxKeyUsername = "username"
const ctxKeyRole = "role"

func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			respondError(c, http.StatusUnauthorized, fmt.Errorf("missing or invalid Authorization header"))
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseToken(tokenStr, h.cfg.JWTSecret)
		if err != nil {
			respondError(c, http.StatusUnauthorized, fmt.Errorf("invalid token: %w", err))
			c.Abort()
			return
		}
		c.Set(ctxKeyUsername, claims.Username)
		c.Set(ctxKeyRole, claims.Role)
		c.Next()
	}
}

func (h *Handler) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ctxKeyRole)
		if role != "admin" {
			respondError(c, http.StatusForbidden, fmt.Errorf("admin access required"))
			c.Abort()
			return
		}
		c.Next()
	}
}
