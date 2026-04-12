package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubevalet/kubevalet/internal/auth"
)

// loginLimiter provides simple per-IP brute-force protection.
// Allows up to 10 attempts per minute per IP; resets after 1 minute of no failures.
var loginLimiter = struct {
	mu      sync.Mutex
	entries map[string]*loginEntry
}{entries: make(map[string]*loginEntry)}

type loginEntry struct {
	count    int
	resetAt  time.Time
}

func checkLoginRate(ip string) bool {
	loginLimiter.mu.Lock()
	defer loginLimiter.mu.Unlock()
	e, ok := loginLimiter.entries[ip]
	if !ok || time.Now().After(e.resetAt) {
		loginLimiter.entries[ip] = &loginEntry{count: 1, resetAt: time.Now().Add(time.Minute)}
		return true
	}
	e.count++
	return e.count <= 10
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	if !checkLoginRate(c.ClientIP()) {
		respondError(c, http.StatusTooManyRequests, fmt.Errorf("too many login attempts, try again later"))
		return
	}

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
