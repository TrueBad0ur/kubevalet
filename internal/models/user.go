package models

import "time"

type PolicyRule struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

type NamespaceBinding struct {
	Namespace  string       `json:"namespace"`
	Role       string       `json:"role,omitempty"`
	CustomRole bool         `json:"customRole,omitempty"`
	Rules      []PolicyRule `json:"rules,omitempty"` // populated on list, not stored in annotation
}

type User struct {
	Name              string             `json:"name"`
	Groups            []string           `json:"groups,omitempty"`
	// Cluster-wide binding
	ClusterRole       string             `json:"clusterRole,omitempty"`
	CustomRole        bool               `json:"customRole,omitempty"` // cluster-wide custom role
	Rules             []PolicyRule       `json:"rules,omitempty"`      // cluster-wide custom rules
	// Namespace-scoped bindings (one or many)
	NamespaceBindings []NamespaceBinding `json:"namespaceBindings,omitempty"`
	Status            string             `json:"status"`
	CreatedAt         time.Time          `json:"createdAt"`
	CertExpiresAt     *time.Time         `json:"certExpiresAt,omitempty"`
}

type CreateUserRequest struct {
	Name              string             `json:"name"        binding:"required"`
	Groups            []string           `json:"groups"`
	// Cluster-wide
	ClusterRole       string             `json:"clusterRole"`
	Rules             []PolicyRule       `json:"rules"` // cluster-wide custom rules
	// Namespace-scoped (one or many)
	NamespaceBindings []NamespaceBinding `json:"namespaceBindings"`
}

type UpdateRBACRequest struct {
	Groups            []string           `json:"groups"`
	// Cluster-wide
	ClusterRole       string             `json:"clusterRole"`
	Rules             []PolicyRule       `json:"rules"`
	// Namespace-scoped
	NamespaceBindings []NamespaceBinding `json:"namespaceBindings"`
}

type UpdateRBACResponse struct {
	Kubeconfig string `json:"kubeconfig,omitempty"` // populated when groups changed and cert was regenerated
}

type CreateUserResponse struct {
	User       User   `json:"user"`
	Kubeconfig string `json:"kubeconfig"`
}

type RenewCertificateResponse struct {
	Kubeconfig    string    `json:"kubeconfig"`
	CertExpiresAt time.Time `json:"certExpiresAt"`
}
