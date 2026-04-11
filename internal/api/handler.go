package api

import (
	"net/http"

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
	protected.DELETE("/users/:name", h.DeleteUser)
	protected.GET("/users/:name/kubeconfig", h.DownloadKubeconfig)
}

type errorResponse struct {
	Error string `json:"error"`
}

func respondError(c *gin.Context, status int, err error) {
	c.JSON(status, errorResponse{Error: err.Error()})
}

func (h *Handler) clusterServer() string {
	if h.cfg.ClusterServer != "" {
		return h.cfg.ClusterServer
	}
	return h.k8s.RestConfig.Host
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
