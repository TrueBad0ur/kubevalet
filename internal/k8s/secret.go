package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorePrivateKey saves the user's private key PEM in a Secret in the given namespace.
// The key is never persisted anywhere else — this is the only copy.
func (c *Client) StorePrivateKey(ctx context.Context, username, namespace string, privateKeyPEM []byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName(username),
			Namespace: namespace,
			Labels:    managedLabels(username),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tls.key": privateKeyPEM,
		},
	}
	_, err := c.Kubernetes.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("store private key: %w", err)
	}
	return nil
}

// GetPrivateKey retrieves the user's private key PEM from the Secret.
func (c *Client) GetPrivateKey(ctx context.Context, username, namespace string) ([]byte, error) {
	secret, err := c.Kubernetes.CoreV1().Secrets(namespace).Get(ctx, resourceName(username), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get private key secret: %w", err)
	}
	key, ok := secret.Data["tls.key"]
	if !ok {
		return nil, fmt.Errorf("secret %s has no tls.key", resourceName(username))
	}
	return key, nil
}

// DeletePrivateKey removes the Secret holding the user's private key.
func (c *Client) DeletePrivateKey(ctx context.Context, username, namespace string) error {
	err := c.Kubernetes.CoreV1().Secrets(namespace).Delete(ctx, resourceName(username), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete private key secret: %w", err)
	}
	return nil
}
