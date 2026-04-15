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
		"SELECT id, username, created_at FROM admin_users ORDER BY created_at")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	users := make([]models.LocalUser, 0)
	for rows.Next() {
		var u models.LocalUser
		if err := rows.Scan(&u.ID, &u.Username, &u.CreatedAt); err != nil {
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
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	var u models.LocalUser
	err = h.db.QueryRow(c.Request.Context(),
		"INSERT INTO admin_users (username, password) VALUES ($1, $2) RETURNING id, username, created_at",
		req.Username, hash,
	).Scan(&u.ID, &u.Username, &u.CreatedAt)
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
		respondError(c, http.StatusBadRequest, fmt.Errorf("cannot delete the last admin user"))
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
