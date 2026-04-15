package models

import "time"

type LocalUser struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type CreateLocalUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"` // "admin" or "viewer" (default: "viewer")
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required"`
}
