package models

import "time"

type PolicyRule struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

type User struct {
	Name        string       `json:"name"`
	Groups      []string     `json:"groups,omitempty"`
	ClusterRole string       `json:"clusterRole,omitempty"`
	Namespace   string       `json:"namespace,omitempty"`
	Role        string       `json:"role,omitempty"`
	CustomRole  bool         `json:"customRole,omitempty"`
	Rules       []PolicyRule `json:"rules,omitempty"`
	Status      string       `json:"status"`
	CreatedAt   time.Time    `json:"createdAt"`
}

type CreateUserRequest struct {
	Name        string       `json:"name"        binding:"required"`
	Groups      []string     `json:"groups"`
	ClusterRole string       `json:"clusterRole"`
	Namespace   string       `json:"namespace"`
	Role        string       `json:"role"`
	Rules       []PolicyRule `json:"rules"` // advanced: custom RBAC rules (creates a dedicated Role/ClusterRole)
}

type CreateUserResponse struct {
	User       User   `json:"user"`
	Kubeconfig string `json:"kubeconfig"`
}

type UpdateRBACRequest struct {
	ClusterRole string       `json:"clusterRole"`
	Namespace   string       `json:"namespace"`
	Role        string       `json:"role"`
	Rules       []PolicyRule `json:"rules"`
}
