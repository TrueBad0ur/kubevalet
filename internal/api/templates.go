package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kubevalet/kubevalet/internal/models"
)

func (h *Handler) ListTemplates(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, name, description, cluster_role, custom_role, rules, ns_bindings, created_at
		FROM role_templates ORDER BY name
	`)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	templates := make([]models.RoleTemplate, 0)
	for rows.Next() {
		var t models.RoleTemplate
		var rulesJSON, nsJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.ClusterRole, &t.CustomRole, &rulesJSON, &nsJSON, &t.CreatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(rulesJSON, &t.Rules)
		_ = json.Unmarshal(nsJSON, &t.NamespaceBindings)
		templates = append(templates, t)
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (h *Handler) CreateTemplate(c *gin.Context) {
	var req models.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateName(req.Name); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	rulesJSON, _ := json.Marshal(req.Rules)
	nsJSON, _ := json.Marshal(req.NamespaceBindings)

	overwrite := c.Query("overwrite") == "true"

	var id int64
	var createdAt time.Time
	var err error
	if overwrite {
		err = h.db.QueryRow(c.Request.Context(), `
			INSERT INTO role_templates (name, description, cluster_role, custom_role, rules, ns_bindings)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (name) DO UPDATE SET
				description = EXCLUDED.description,
				cluster_role = EXCLUDED.cluster_role,
				custom_role = EXCLUDED.custom_role,
				rules = EXCLUDED.rules,
				ns_bindings = EXCLUDED.ns_bindings
			RETURNING id, created_at
		`, req.Name, req.Description, req.ClusterRole, req.CustomRole, rulesJSON, nsJSON).Scan(&id, &createdAt)
	} else {
		err = h.db.QueryRow(c.Request.Context(), `
			INSERT INTO role_templates (name, description, cluster_role, custom_role, rules, ns_bindings)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, created_at
		`, req.Name, req.Description, req.ClusterRole, req.CustomRole, rulesJSON, nsJSON).Scan(&id, &createdAt)
	}
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			respondError(c, http.StatusConflict, fmt.Errorf("template %q already exists", req.Name))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, models.RoleTemplate{
		ID:                id,
		Name:              req.Name,
		Description:       req.Description,
		ClusterRole:       req.ClusterRole,
		CustomRole:        req.CustomRole,
		Rules:             req.Rules,
		NamespaceBindings: req.NamespaceBindings,
		CreatedAt:         createdAt,
	})
}

func (h *Handler) DeleteTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	_, err = h.db.Exec(c.Request.Context(), "DELETE FROM role_templates WHERE id=$1", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}
