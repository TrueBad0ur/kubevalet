package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Manager holds k8s clients per cluster, created lazily from kubeconfigs in DB.
type Manager struct {
	mu         sync.RWMutex
	clients    map[int64]*Client
	db         *pgxpool.Pool
	defaultID  int64
	defaultClient *Client
}

func NewManager(defaultID int64, defaultClient *Client, db *pgxpool.Pool) *Manager {
	m := &Manager{
		clients:       make(map[int64]*Client),
		db:            db,
		defaultID:     defaultID,
		defaultClient: defaultClient,
	}
	m.clients[defaultID] = defaultClient
	return m
}

// Get returns the k8s client for the given cluster ID.
// For the default cluster (in-cluster) it returns the pre-built client.
// For other clusters it builds lazily from kubeconfig stored in DB.
func (m *Manager) Get(ctx context.Context, clusterID int64) (*Client, error) {
	m.mu.RLock()
	c, ok := m.clients[clusterID]
	m.mu.RUnlock()
	if ok {
		return c, nil
	}

	var kubeconfig *string
	err := m.db.QueryRow(ctx,
		"SELECT kubeconfig FROM clusters WHERE id=$1", clusterID,
	).Scan(&kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("cluster %d not found: %w", clusterID, err)
	}
	if kubeconfig == nil {
		// Another in-cluster-type entry — reuse default client
		m.mu.Lock()
		m.clients[clusterID] = m.defaultClient
		m.mu.Unlock()
		return m.defaultClient, nil
	}

	c, err = NewFromBytes([]byte(*kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("build client for cluster %d: %w", clusterID, err)
	}

	m.mu.Lock()
	m.clients[clusterID] = c
	m.mu.Unlock()
	return c, nil
}

// Invalidate removes a cached client so it will be rebuilt on next Get.
func (m *Manager) Invalidate(clusterID int64) {
	m.mu.Lock()
	delete(m.clients, clusterID)
	m.mu.Unlock()
}

// DefaultID returns the ID of the default (in-cluster) cluster.
func (m *Manager) DefaultID() int64 {
	return m.defaultID
}
