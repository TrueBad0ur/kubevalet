package models

import "time"

type Cluster struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Kubeconfig  *string   `json:"kubeconfig,omitempty"` // nil = in-cluster
	APIServer   string    `json:"apiServer"`
	ClusterName string    `json:"clusterName"`
	CreatedAt   time.Time `json:"createdAt"`
}

type CreateClusterRequest struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description"`
	Kubeconfig  string `json:"kubeconfig"  binding:"required"`
	APIServer   string `json:"apiServer"   binding:"required"`
	ClusterName string `json:"clusterName"`
}
