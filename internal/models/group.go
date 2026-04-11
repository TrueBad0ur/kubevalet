package models

import "time"

type Group struct {
	ID                int64              `json:"id"`
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	ClusterRole       string             `json:"clusterRole,omitempty"`
	CustomRole        bool               `json:"customRole,omitempty"`
	Rules             []PolicyRule       `json:"rules,omitempty"`
	NamespaceBindings []NamespaceBinding `json:"namespaceBindings,omitempty"`
	CreatedAt         time.Time          `json:"createdAt"`
}

type CreateGroupRequest struct {
	Name              string             `json:"name"        binding:"required"`
	Description       string             `json:"description"`
	ClusterRole       string             `json:"clusterRole"`
	Rules             []PolicyRule       `json:"rules"`
	NamespaceBindings []NamespaceBinding `json:"namespaceBindings"`
}

type UpdateGroupRequest struct {
	Description       string             `json:"description"`
	ClusterRole       string             `json:"clusterRole"`
	Rules             []PolicyRule       `json:"rules"`
	NamespaceBindings []NamespaceBinding `json:"namespaceBindings"`
}
