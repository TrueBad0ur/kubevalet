package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kubevalet/kubevalet/internal/auth"
	"github.com/kubevalet/kubevalet/internal/models"
)

func (h *Handler) requireLocalUsers(c *gin.Context) bool {
	if !h.cfg.EnableLocalUsers {
		respondError(c, http.StatusForbidden, fmt.Errorf("local users feature is disabled"))
		return false
	}
	return true
}

func (h *Handler) ListLocalUsers(c *gin.Context) {
	if !h.requireLocalUsers(c) {
		return
	}
	rows, err := h.db.Query(c.Request.Context(),
		"SELECT id, username, role, created_at FROM admin_users ORDER BY created_at")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	users := make([]models.LocalUser, 0)
	for rows.Next() {
		var u models.LocalUser
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		users = append(users, u)
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "total": len(users)})
}

func (h *Handler) CreateLocalUser(c *gin.Context) {
	if !h.requireLocalUsers(c) {
		return
	}
	var req models.CreateLocalUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateName(req.Username); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	role := req.Role
	if role != "admin" && role != "viewer" {
		role = "viewer"
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	var u models.LocalUser
	err = h.db.QueryRow(c.Request.Context(),
		"INSERT INTO admin_users (username, password, role) VALUES ($1, $2, $3) RETURNING id, username, role, created_at",
		req.Username, hash, role,
	).Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(c, http.StatusConflict, fmt.Errorf("user %q already exists", req.Username))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, u)
}

func (h *Handler) DeleteLocalUser(c *gin.Context) {
	if !h.requireLocalUsers(c) {
		return
	}
	targetUsername := c.Param("username")
	currentUsername, _ := c.Get(ctxKeyUsername)

	if targetUsername == "admin" {
		respondError(c, http.StatusForbidden, fmt.Errorf("user \"admin\" cannot be deleted"))
		return
	}
	if targetUsername == currentUsername.(string) {
		respondError(c, http.StatusBadRequest, fmt.Errorf("cannot delete your own account"))
		return
	}

	var count int
	if err := h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM admin_users").Scan(&count); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	if count <= 1 {
		respondError(c, http.StatusBadRequest, fmt.Errorf("cannot delete the last user"))
		return
	}

	result, err := h.db.Exec(c.Request.Context(), "DELETE FROM admin_users WHERE username=$1", targetUsername)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	if result.RowsAffected() == 0 {
		respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found", targetUsername))
		return
	}
	c.Status(http.StatusNoContent)
}

type resetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

func (h *Handler) ResetLocalUserPassword(c *gin.Context) {
	if !h.requireLocalUsers(c) {
		return
	}
	targetUsername := c.Param("username")
	currentUsername, _ := c.Get(ctxKeyUsername)

	// An admin's password can only be changed by that admin themselves.
	// Check if target is an admin and the requester is a different user.
	if targetUsername != currentUsername.(string) {
		var targetRole string
		err := h.db.QueryRow(c.Request.Context(),
			"SELECT role FROM admin_users WHERE username=$1", targetUsername).Scan(&targetRole)
		if err != nil {
			respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found", targetUsername))
			return
		}
		if targetRole == "admin" {
			respondError(c, http.StatusForbidden, fmt.Errorf("cannot change another admin's password"))
			return
		}
	}

	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	result, err := h.db.Exec(c.Request.Context(),
		"UPDATE admin_users SET password=$1 WHERE username=$2", hash, targetUsername)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	if result.RowsAffected() == 0 {
		respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found", targetUsername))
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) UpdateLocalUserRole(c *gin.Context) {
	if !h.requireLocalUsers(c) {
		return
	}
	targetUsername := c.Param("username")
	currentUsername, _ := c.Get(ctxKeyUsername)

	if targetUsername == "admin" {
		respondError(c, http.StatusForbidden, fmt.Errorf("cannot change role of the \"admin\" user"))
		return
	}
	if targetUsername == currentUsername.(string) {
		respondError(c, http.StatusBadRequest, fmt.Errorf("cannot change your own role"))
		return
	}

	var req models.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if req.Role != "admin" && req.Role != "viewer" {
		respondError(c, http.StatusBadRequest, fmt.Errorf("role must be \"admin\" or \"viewer\""))
		return
	}

	// Prevent removing the last admin
	if req.Role == "viewer" {
		var adminCount int
		if err := h.db.QueryRow(c.Request.Context(),
			"SELECT COUNT(*) FROM admin_users WHERE role='admin'").Scan(&adminCount); err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		if adminCount <= 1 {
			respondError(c, http.StatusBadRequest, fmt.Errorf("cannot demote the last admin"))
			return
		}
	}

	result, err := h.db.Exec(c.Request.Context(),
		"UPDATE admin_users SET role=$1 WHERE username=$2", req.Role, targetUsername)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	if result.RowsAffected() == 0 {
		respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found", targetUsername))
		return
	}
	c.Status(http.StatusNoContent)
}
