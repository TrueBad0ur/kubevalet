package models

import "time"

type RoleTemplate struct {
	ID                int64              `json:"id"`
	Name              string             `json:"name"`
	Description       string             `json:"description,omitempty"`
	ClusterRole       string             `json:"clusterRole,omitempty"`
	CustomRole        bool               `json:"customRole,omitempty"`
	Rules             []PolicyRule       `json:"rules,omitempty"`
	NamespaceBindings []NamespaceBinding `json:"namespaceBindings,omitempty"`
	CreatedAt         time.Time          `json:"createdAt"`
}

type CreateTemplateRequest struct {
	Name              string             `json:"name"        binding:"required"`
	Description       string             `json:"description"`
	ClusterRole       string             `json:"clusterRole"`
	CustomRole        bool               `json:"customRole"`
	Rules             []PolicyRule       `json:"rules"`
	NamespaceBindings []NamespaceBinding `json:"namespaceBindings"`
}
