package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kubevalet/kubevalet/internal/auth"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	row := h.db.QueryRow(c.Request.Context(),
		"SELECT password FROM admin_users WHERE username = $1", req.Username)

	var hash string
	if err := row.Scan(&hash); err != nil {
		// Deliberate vague error to prevent username enumeration
		respondError(c, http.StatusUnauthorized, fmt.Errorf("invalid credentials"))
		return
	}

	if !auth.CheckPassword(hash, req.Password) {
		respondError(c, http.StatusUnauthorized, fmt.Errorf("invalid credentials"))
		return
	}

	token, err := auth.SignToken(req.Username, h.cfg.JWTSecret, h.cfg.TokenTTL)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *Handler) Me(c *gin.Context) {
	username, _ := c.Get(ctxKeyUsername)
	c.JSON(http.StatusOK, gin.H{"username": username})
}
