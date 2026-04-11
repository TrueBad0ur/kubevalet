package k8s

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/kubevalet/kubevalet/internal/models"
)

// CreateClusterRoleBinding binds a ClusterRole to a user cluster-wide.
func (c *Client) CreateClusterRoleBinding(ctx context.Context, username, clusterRole string) error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   resourceName(username),
			Labels: managedLabels(username),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.UserKind, Name: username},
		},
	}
	_, err := c.Kubernetes.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create cluster role binding: %w", err)
	}
	return nil
}

// CreateRoleBinding binds a ClusterRole to a user within a single namespace.
func (c *Client) CreateRoleBinding(ctx context.Context, username, namespace, role string) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName(username),
			Namespace: namespace,
			Labels:    managedLabels(username),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole", // bind built-in ClusterRoles (admin/edit/view) namespace-scoped
			Name:     role,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.UserKind, Name: username},
		},
	}
	_, err := c.Kubernetes.RbacV1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create role binding: %w", err)
	}
	return nil
}

func (c *Client) DeleteClusterRoleBinding(ctx context.Context, username string) error {
	err := c.Kubernetes.RbacV1().ClusterRoleBindings().Delete(ctx, resourceName(username), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete cluster role binding: %w", err)
	}
	return nil
}

func (c *Client) DeleteRoleBinding(ctx context.Context, username, namespace string) error {
	err := c.Kubernetes.RbacV1().RoleBindings(namespace).Delete(ctx, resourceName(username), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete role binding: %w", err)
	}
	return nil
}

// CreateCustomClusterRole creates a ClusterRole with user-defined rules and binds it to the user.
func (c *Client) CreateCustomClusterRole(ctx context.Context, username string, rules []models.PolicyRule) error {
	policyRules := toK8sRules(rules)
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   resourceName(username),
			Labels: managedLabels(username),
		},
		Rules: policyRules,
	}
	if _, err := c.Kubernetes.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create custom cluster role: %w", err)
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   resourceName(username),
			Labels: managedLabels(username),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     resourceName(username),
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.UserKind, Name: username},
		},
	}
	if _, err := c.Kubernetes.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create cluster role binding: %w", err)
	}
	return nil
}

// CreateCustomRole creates a namespace-scoped Role with user-defined rules and binds it.
func (c *Client) CreateCustomRole(ctx context.Context, username, namespace string, rules []models.PolicyRule) error {
	policyRules := toK8sRules(rules)
	r := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName(username),
			Namespace: namespace,
			Labels:    managedLabels(username),
		},
		Rules: policyRules,
	}
	if _, err := c.Kubernetes.RbacV1().Roles(namespace).Create(ctx, r, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create custom role: %w", err)
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName(username),
			Namespace: namespace,
			Labels:    managedLabels(username),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     resourceName(username),
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.UserKind, Name: username},
		},
	}
	if _, err := c.Kubernetes.RbacV1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create role binding: %w", err)
	}
	return nil
}

// CreateNamespaceBindings creates RoleBindings (and custom Roles if needed) for each namespace binding.
func (c *Client) CreateNamespaceBindings(ctx context.Context, username string, bindings []models.NamespaceBinding) error {
	for _, b := range bindings {
		var err error
		if len(b.Rules) > 0 {
			err = c.CreateCustomRole(ctx, username, b.Namespace, b.Rules)
		} else {
			err = c.CreateRoleBinding(ctx, username, b.Namespace, b.Role)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteAllNamespaceBindings removes all RoleBindings and custom Roles for this user across all namespaces.
func (c *Client) DeleteAllNamespaceBindings(ctx context.Context, username string) error {
	selector := LabelUsername + "=" + username

	rbs, err := c.Kubernetes.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("list role bindings: %w", err)
	}
	for _, rb := range rbs.Items {
		if err := c.Kubernetes.RbacV1().RoleBindings(rb.Namespace).Delete(ctx, rb.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete role binding %s/%s: %w", rb.Namespace, rb.Name, err)
		}
	}

	roles, err := c.Kubernetes.RbacV1().Roles("").List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("list roles: %w", err)
	}
	for _, r := range roles.Items {
		if err := c.Kubernetes.RbacV1().Roles(r.Namespace).Delete(ctx, r.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete role %s/%s: %w", r.Namespace, r.Name, err)
		}
	}
	return nil
}

func (c *Client) DeleteCustomClusterRole(ctx context.Context, username string) error {
	err := c.Kubernetes.RbacV1().ClusterRoles().Delete(ctx, resourceName(username), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete custom cluster role: %w", err)
	}
	return nil
}

func (c *Client) DeleteCustomRole(ctx context.Context, username, namespace string) error {
	err := c.Kubernetes.RbacV1().Roles(namespace).Delete(ctx, resourceName(username), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete custom role: %w", err)
	}
	return nil
}

func toK8sRules(rules []models.PolicyRule) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, rbacv1.PolicyRule{
			APIGroups: r.APIGroups,
			Resources: r.Resources,
			Verbs:     r.Verbs,
		})
	}
	return out
}

func fromK8sRules(rules []rbacv1.PolicyRule) []models.PolicyRule {
	out := make([]models.PolicyRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, models.PolicyRule{
			APIGroups: r.APIGroups,
			Resources: r.Resources,
			Verbs:     r.Verbs,
		})
	}
	return out
}

// GetCustomRoleRules returns the rules from the kubevalet-managed Role or ClusterRole for this user.
func (c *Client) GetCustomRoleRules(ctx context.Context, username, namespace string) ([]models.PolicyRule, error) {
	if namespace == "" {
		cr, err := c.Kubernetes.RbacV1().ClusterRoles().Get(ctx, resourceName(username), metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("get custom cluster role: %w", err)
		}
		return fromK8sRules(cr.Rules), nil
	}
	r, err := c.Kubernetes.RbacV1().Roles(namespace).Get(ctx, resourceName(username), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get custom role: %w", err)
	}
	return fromK8sRules(r.Rules), nil
}
