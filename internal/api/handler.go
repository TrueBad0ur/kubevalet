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

	protected.GET("/auth/me", h.Me)
	protected.GET("/users", h.ListUsers)
	protected.POST("/users", h.CreateUser)
	protected.PUT("/users/:name/rbac", h.UpdateUserRBAC)
	protected.DELETE("/users/:name", h.DeleteUser)
	protected.GET("/users/:name/kubeconfig", h.DownloadKubeconfig)
	protected.POST("/users/:name/sync", h.SyncUser)
	protected.POST("/users/:name/renew", h.RenewCertificate)
	protected.GET("/settings", h.GetSettings)
	protected.PUT("/settings", h.UpdateSettings)
	protected.PUT("/settings/password", h.ChangePassword)
	protected.GET("/groups", h.ListGroups)
	protected.POST("/groups", h.CreateGroup)
	protected.PUT("/groups/:name", h.UpdateGroup)
	protected.DELETE("/groups/:name", h.DeleteGroup)
	protected.POST("/groups/:name/sync", h.SyncGroup)
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

