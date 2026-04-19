package api

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubevalet/kubevalet/internal/config"
	"github.com/kubevalet/kubevalet/internal/k8s"
)

type Handler struct {
	mgr *k8s.Manager
	k8s *k8s.Client // default cluster client — used by existing handlers until per-cluster migration
	cfg *config.Config
	db  *pgxpool.Pool
}

func New(mgr *k8s.Manager, cfg *config.Config, db *pgxpool.Pool) *Handler {
	defaultClient, _ := mgr.Get(context.Background(), mgr.DefaultID())
	return &Handler{mgr: mgr, k8s: defaultClient, cfg: cfg, db: db}
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
	protected.GET("/clusters", h.ListClusters)

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

	admin.POST("/clusters", h.CreateCluster)
	admin.DELETE("/clusters/:id", h.DeleteCluster)

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

// k8sForCluster returns the k8s client for the cluster ID in the request context.
func (h *Handler) k8sForCluster(c *gin.Context) (*k8s.Client, int64, error) {
	clusterID := h.clusterIDFromCtx(c)
	client, err := h.mgr.Get(c.Request.Context(), clusterID)
	if err != nil {
		return nil, 0, fmt.Errorf("get k8s client: %w", err)
	}
	return client, clusterID, nil
}

// clusterIDFromCtx reads cluster_id from query param, defaulting to the primary cluster.
func (h *Handler) clusterIDFromCtx(c *gin.Context) int64 {
	var id int64
	if v := c.Query("cluster_id"); v != "" {
		fmt.Sscan(v, &id)
	}
	if id <= 0 {
		id = h.mgr.DefaultID()
	}
	return id
}

// clusterInfo returns apiServer and clusterName for a given cluster ID.
// Falls back to app_settings (legacy), then env/config, then in-cluster host.
func (h *Handler) clusterInfo(ctx context.Context, clusterID int64) (apiServer, clusterName string) {
	_ = h.db.QueryRow(ctx,
		"SELECT api_server, cluster_name FROM clusters WHERE id=$1", clusterID,
	).Scan(&apiServer, &clusterName)

	if apiServer == "" {
		apiServer = h.cfg.ClusterServer
	}
	if apiServer == "" {
		if c, err := h.mgr.Get(ctx, h.mgr.DefaultID()); err == nil {
			apiServer = c.RestConfig.Host
		}
	}
	if clusterName == "" {
		clusterName = h.cfg.ClusterName
	}
	return
}

// clusterServer returns the API server URL for the default cluster (legacy compat).
func (h *Handler) clusterServer(ctx context.Context) string {
	apiServer, _ := h.clusterInfo(ctx, h.mgr.DefaultID())
	return apiServer
}
