package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kubevalet/kubevalet/internal/k8s"
	"github.com/kubevalet/kubevalet/internal/models"
)

func (h *Handler) ListClusters(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, name, description, api_server, cluster_name, created_at
		FROM clusters ORDER BY name
	`)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	clusters := make([]models.Cluster, 0)
	for rows.Next() {
		var cl models.Cluster
		if err := rows.Scan(&cl.ID, &cl.Name, &cl.Description, &cl.APIServer, &cl.ClusterName, &cl.CreatedAt); err != nil {
			continue
		}
		clusters = append(clusters, cl)
	}
	c.JSON(http.StatusOK, gin.H{"clusters": clusters})
}

func (h *Handler) CreateCluster(c *gin.Context) {
	var req models.CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateName(req.Name); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if req.ClusterName == "" {
		req.ClusterName = "kubernetes"
	}

	// Validate kubeconfig by trying to build a client
	if _, err := k8s.NewFromBytes([]byte(req.Kubeconfig)); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	var id int64
	var createdAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		INSERT INTO clusters (name, description, kubeconfig, api_server, cluster_name)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, req.Name, req.Description, req.Kubeconfig, req.APIServer, req.ClusterName).Scan(&id, &createdAt)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			respondError(c, http.StatusConflict, err)
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, models.Cluster{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		APIServer:   req.APIServer,
		ClusterName: req.ClusterName,
		CreatedAt:   createdAt,
	})
}

func (h *Handler) DeleteCluster(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if id == h.mgr.DefaultID() {
		respondError(c, http.StatusForbidden, errors.New("cannot delete the default cluster"))
		return
	}
	_, err = h.db.Exec(c.Request.Context(), "DELETE FROM clusters WHERE id=$1", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	h.mgr.Invalidate(id)
	c.Status(http.StatusNoContent)
}
