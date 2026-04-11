package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kubevalet/kubevalet/internal/models"
)

func (h *Handler) ListGroups(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, name, description, cluster_role, custom_role, rules, ns_bindings, created_at
		FROM groups ORDER BY name
	`)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	groups := make([]models.Group, 0)
	for rows.Next() {
		var g models.Group
		var rulesJSON, nsJSON []byte
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.ClusterRole, &g.CustomRole, &rulesJSON, &nsJSON, &g.CreatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		_ = json.Unmarshal(rulesJSON, &g.Rules)
		_ = json.Unmarshal(nsJSON, &g.NamespaceBindings)
		groups = append(groups, g)
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups, "total": len(groups)})
}

func (h *Handler) CreateGroup(c *gin.Context) {
	var req models.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateGroupRBAC(req.ClusterRole, req.Rules, req.NamespaceBindings); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	clusterCustom := len(req.Rules) > 0
	nsScoped := len(req.NamespaceBindings) > 0

	rulesJSON, _ := json.Marshal(req.Rules)
	nsJSON, _ := json.Marshal(req.NamespaceBindings)
	if rulesJSON == nil {
		rulesJSON = []byte("[]")
	}
	if nsJSON == nil {
		nsJSON = []byte("[]")
	}

	var id int64
	err := h.db.QueryRow(c.Request.Context(), `
		INSERT INTO groups (name, description, cluster_role, custom_role, rules, ns_bindings)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id
	`, req.Name, req.Description, req.ClusterRole, clusterCustom, rulesJSON, nsJSON).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			respondError(c, http.StatusConflict, fmt.Errorf("group %q already exists", req.Name))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// Create k8s bindings if RBAC was specified
	ctx := c.Request.Context()
	if req.ClusterRole != "" {
		err = h.k8s.CreateGroupClusterRoleBinding(ctx, req.Name, req.ClusterRole)
	} else if clusterCustom {
		err = h.k8s.CreateGroupCustomClusterRole(ctx, req.Name, req.Rules)
	} else if nsScoped {
		err = h.k8s.CreateGroupNamespaceBindings(ctx, req.Name, req.NamespaceBindings)
	}
	if err != nil {
		// Roll back DB insert
		_, _ = h.db.Exec(c.Request.Context(), "DELETE FROM groups WHERE id=$1", id)
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	g := models.Group{
		ID:                id,
		Name:              req.Name,
		Description:       req.Description,
		ClusterRole:       req.ClusterRole,
		CustomRole:        clusterCustom,
		Rules:             req.Rules,
		NamespaceBindings: req.NamespaceBindings,
	}
	c.JSON(http.StatusCreated, g)
}

func (h *Handler) UpdateGroup(c *gin.Context) {
	groupName := c.Param("name")
	ctx := c.Request.Context()

	var req models.UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateGroupRBAC(req.ClusterRole, req.Rules, req.NamespaceBindings); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	// Check group exists
	var oldCustomRole bool
	var oldClusterRole string
	err := h.db.QueryRow(ctx, "SELECT cluster_role, custom_role FROM groups WHERE name=$1", groupName).
		Scan(&oldClusterRole, &oldCustomRole)
	if err != nil {
		respondError(c, http.StatusNotFound, fmt.Errorf("group %q not found", groupName))
		return
	}

	clusterCustom := len(req.Rules) > 0
	nsScoped := len(req.NamespaceBindings) > 0

	// Delete old k8s bindings
	var errs []string
	collect := func(e error) {
		if e != nil {
			errs = append(errs, e.Error())
		}
	}
	collect(h.k8s.DeleteGroupClusterRoleBinding(ctx, groupName))
	if oldCustomRole && oldClusterRole == "" {
		collect(h.k8s.DeleteGroupCustomClusterRole(ctx, groupName))
	}
	collect(h.k8s.DeleteAllGroupNamespaceBindings(ctx, groupName))
	if len(errs) > 0 {
		respondError(c, http.StatusInternalServerError, fmt.Errorf(strings.Join(errs, "; ")))
		return
	}

	// Create new k8s bindings
	if req.ClusterRole != "" {
		err = h.k8s.CreateGroupClusterRoleBinding(ctx, groupName, req.ClusterRole)
	} else if clusterCustom {
		err = h.k8s.CreateGroupCustomClusterRole(ctx, groupName, req.Rules)
	} else if nsScoped {
		err = h.k8s.CreateGroupNamespaceBindings(ctx, groupName, req.NamespaceBindings)
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	rulesJSON, _ := json.Marshal(req.Rules)
	nsJSON, _ := json.Marshal(req.NamespaceBindings)
	if rulesJSON == nil {
		rulesJSON = []byte("[]")
	}
	if nsJSON == nil {
		nsJSON = []byte("[]")
	}

	_, err = h.db.Exec(ctx, `
		UPDATE groups SET description=$1, cluster_role=$2, custom_role=$3, rules=$4, ns_bindings=$5
		WHERE name=$6
	`, req.Description, req.ClusterRole, clusterCustom, rulesJSON, nsJSON, groupName)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) DeleteGroup(c *gin.Context) {
	groupName := c.Param("name")
	ctx := c.Request.Context()

	var customRole bool
	var clusterRole string
	err := h.db.QueryRow(ctx, "SELECT cluster_role, custom_role FROM groups WHERE name=$1", groupName).
		Scan(&clusterRole, &customRole)
	if err != nil {
		respondError(c, http.StatusNotFound, fmt.Errorf("group %q not found", groupName))
		return
	}

	var errs []string
	collect := func(e error) {
		if e != nil {
			errs = append(errs, e.Error())
		}
	}
	collect(h.k8s.DeleteGroupClusterRoleBinding(ctx, groupName))
	if customRole && clusterRole == "" {
		collect(h.k8s.DeleteGroupCustomClusterRole(ctx, groupName))
	}
	collect(h.k8s.DeleteAllGroupNamespaceBindings(ctx, groupName))

	if len(errs) > 0 {
		respondError(c, http.StatusInternalServerError, fmt.Errorf(strings.Join(errs, "; ")))
		return
	}

	_, err = h.db.Exec(ctx, "DELETE FROM groups WHERE name=$1", groupName)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func validateGroupRBAC(clusterRole string, rules []models.PolicyRule, nsBindings []models.NamespaceBinding) error {
	for _, nb := range nsBindings {
		if nb.Namespace == "" {
			return fmt.Errorf("each namespaceBinding must have a namespace")
		}
		if nb.Role == "" && len(nb.Rules) == 0 {
			return fmt.Errorf("each namespaceBinding must have role or rules")
		}
	}
	_ = clusterRole
	_ = rules
	return nil
}
