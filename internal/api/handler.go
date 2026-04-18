package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubevalet/kubevalet/internal/config"
	"github.com/kubevalet/kubevalet/internal/k8s"
)

type Handler struct {
	k8s *k8s.Client
	cfg *config.Config
	db  *pgxpool.Pool
}

func New(k8sClient *k8s.Client, cfg *config.Config, db *pgxpool.Pool) *Handler {
	return &Handler{k8s: k8sClient, cfg: cfg, db: db}
}

// RegisterPublic registers routes that do not require authentication.
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.POST("/auth/login", h.Login)
}

// RegisterProtected registers routes behind JWT auth middleware.
func (h *Handler) RegisterProtected(rg *gin.RouterGroup) {
	protected := rg.Group("")
	protected.Use(h.AuthRequired())

	// Available to all authenticated users (admin + viewer)
	protected.GET("/auth/me", h.Me)
	protected.GET("/users", h.ListUsers)
	protected.GET("/groups", h.ListGroups)
	protected.GET("/settings", h.GetSettings)
	protected.PUT("/settings/password", h.ChangePassword)

	// Admin-only routes
	admin := protected.Group("")
	admin.Use(h.RequireAdmin())

	admin.POST("/users", h.CreateUser)
	admin.PUT("/users/:name/rbac", h.UpdateUserRBAC)
	admin.DELETE("/users/:name", h.DeleteUser)
	admin.GET("/users/:name/kubeconfig", h.DownloadKubeconfig)
	admin.POST("/users/:name/sync", h.SyncUser)
	admin.POST("/users/:name/renew", h.RenewCertificate)
	admin.PUT("/settings", h.UpdateSettings)
	admin.POST("/groups", h.CreateGroup)
	admin.PUT("/groups/:name", h.UpdateGroup)
	admin.DELETE("/groups/:name", h.DeleteGroup)
	admin.POST("/groups/:name/sync", h.SyncGroup)

	admin.GET("/local-users", h.ListLocalUsers)
	admin.POST("/local-users", h.CreateLocalUser)
	admin.DELETE("/local-users/:username", h.DeleteLocalUser)
	admin.PUT("/local-users/:username/password", h.ResetLocalUserPassword)
	admin.PUT("/local-users/:username/role", h.UpdateLocalUserRole)

	protected.GET("/templates", h.ListTemplates)
	admin.POST("/templates", h.CreateTemplate)
	admin.DELETE("/templates/:id", h.DeleteTemplate)
}

type errorResponse struct {
	Error string `json:"error"`
}

func respondError(c *gin.Context, status int, err error) {
	c.JSON(status, errorResponse{Error: err.Error()})
}

// clusterServer returns the API server address for kubeconfig generation.
// Priority: DB setting > env/config > in-cluster rest host.
func (h *Handler) clusterServer(ctx context.Context) string {
	var val string
	_ = h.db.QueryRow(ctx, "SELECT value FROM app_settings WHERE key='cluster_server'").Scan(&val)
	if val != "" {
		return val
	}
	if h.cfg.ClusterServer != "" {
		return h.cfg.ClusterServer
	}
	return h.k8s.RestConfig.Host
}

