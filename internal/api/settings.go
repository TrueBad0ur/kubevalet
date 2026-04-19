package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kubevalet/kubevalet/internal/auth"
	"github.com/kubevalet/kubevalet/internal/version"
)

func (h *Handler) GetSettings(c *gin.Context) {
	clusterID := h.clusterIDFromCtx(c)
	var clusterServer string
	_ = h.db.QueryRow(c.Request.Context(),
		"SELECT api_server FROM clusters WHERE id=$1", clusterID).Scan(&clusterServer)
	c.JSON(http.StatusOK, gin.H{
		"version":           version.Version,
		"clusterServer":     clusterServer,
		"localUsersEnabled": h.cfg.EnableLocalUsers,
	})
}

type updateSettingsRequest struct {
	ClusterServer string `json:"clusterServer"`
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	clusterID := h.clusterIDFromCtx(c)
	_, err := h.db.Exec(c.Request.Context(),
		"UPDATE clusters SET api_server=$1 WHERE id=$2", req.ClusterServer, clusterID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Errorf("save setting: %w", err))
		return
	}
	c.Status(http.StatusNoContent)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword"     binding:"required"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	usernameVal, _ := c.Get(ctxKeyUsername)
	username, _ := usernameVal.(string)

	var hash string
	err := h.db.QueryRow(c.Request.Context(),
		"SELECT password FROM admin_users WHERE username = $1", username).Scan(&hash)
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Errorf("user not found"))
		return
	}

	if !auth.CheckPassword(hash, req.CurrentPassword) {
		respondError(c, http.StatusUnauthorized, fmt.Errorf("current password is incorrect"))
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	_, err = h.db.Exec(c.Request.Context(),
		"UPDATE admin_users SET password = $1 WHERE username = $2", newHash, username)
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Errorf("update password: %w", err))
		return
	}

	c.Status(http.StatusNoContent)
}
