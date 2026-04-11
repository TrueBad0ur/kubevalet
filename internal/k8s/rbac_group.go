package k8s

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/kubevalet/kubevalet/internal/models"
)

func groupLabels(groupName string) map[string]string {
	return map[string]string{
		LabelManagedBy: LabelManagedByValue,
		LabelGroup:     groupName,
	}
}

// CreateGroupClusterRoleBinding binds a ClusterRole to a k8s Group cluster-wide.
func (c *Client) CreateGroupClusterRoleBinding(ctx context.Context, groupName, clusterRole string) error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   groupResourceName(groupName),
			Labels: groupLabels(groupName),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.GroupKind, Name: groupName},
		},
	}
	_, err := c.Kubernetes.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create group cluster role binding: %w", err)
	}
	return nil
}

// CreateGroupCustomClusterRole creates a ClusterRole with custom rules and binds it to a k8s Group.
func (c *Client) CreateGroupCustomClusterRole(ctx context.Context, groupName string, rules []models.PolicyRule) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   groupResourceName(groupName),
			Labels: groupLabels(groupName),
		},
		Rules: toK8sRules(rules),
	}
	if _, err := c.Kubernetes.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create group custom cluster role: %w", err)
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   groupResourceName(groupName),
			Labels: groupLabels(groupName),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     groupResourceName(groupName),
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.GroupKind, Name: groupName},
		},
	}
	if _, err := c.Kubernetes.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create group cluster role binding: %w", err)
	}
	return nil
}

// CreateGroupNamespaceBindings creates RoleBindings for each namespace binding.
func (c *Client) CreateGroupNamespaceBindings(ctx context.Context, groupName string, bindings []models.NamespaceBinding) error {
	for _, b := range bindings {
		if len(b.Rules) > 0 {
			if err := c.createGroupCustomRole(ctx, groupName, b.Namespace, b.Rules); err != nil {
				return err
			}
		} else {
			if err := c.createGroupRoleBinding(ctx, groupName, b.Namespace, b.Role); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) createGroupRoleBinding(ctx context.Context, groupName, namespace, role string) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      groupResourceName(groupName),
			Namespace: namespace,
			Labels:    groupLabels(groupName),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     role,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.GroupKind, Name: groupName},
		},
	}
	_, err := c.Kubernetes.RbacV1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create group role binding: %w", err)
	}
	return nil
}

func (c *Client) createGroupCustomRole(ctx context.Context, groupName, namespace string, rules []models.PolicyRule) error {
	r := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      groupResourceName(groupName),
			Namespace: namespace,
			Labels:    groupLabels(groupName),
		},
		Rules: toK8sRules(rules),
	}
	if _, err := c.Kubernetes.RbacV1().Roles(namespace).Create(ctx, r, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create group custom role: %w", err)
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      groupResourceName(groupName),
			Namespace: namespace,
			Labels:    groupLabels(groupName),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     groupResourceName(groupName),
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.GroupKind, Name: groupName},
		},
	}
	if _, err := c.Kubernetes.RbacV1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create group role binding: %w", err)
	}
	return nil
}

func (c *Client) DeleteGroupClusterRoleBinding(ctx context.Context, groupName string) error {
	err := c.Kubernetes.RbacV1().ClusterRoleBindings().Delete(ctx, groupResourceName(groupName), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete group cluster role binding: %w", err)
	}
	return nil
}

func (c *Client) DeleteGroupCustomClusterRole(ctx context.Context, groupName string) error {
	err := c.Kubernetes.RbacV1().ClusterRoles().Delete(ctx, groupResourceName(groupName), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete group custom cluster role: %w", err)
	}
	return nil
}

// DeleteAllGroupNamespaceBindings removes all RoleBindings and custom Roles for this group across all namespaces.
func (c *Client) DeleteAllGroupNamespaceBindings(ctx context.Context, groupName string) error {
	selector := LabelGroup + "=" + groupName

	rbs, err := c.Kubernetes.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("list group role bindings: %w", err)
	}
	for _, rb := range rbs.Items {
		if err := c.Kubernetes.RbacV1().RoleBindings(rb.Namespace).Delete(ctx, rb.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete group role binding %s/%s: %w", rb.Namespace, rb.Name, err)
		}
	}

	roles, err := c.Kubernetes.RbacV1().Roles("").List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("list group roles: %w", err)
	}
	for _, r := range roles.Items {
		if err := c.Kubernetes.RbacV1().Roles(r.Namespace).Delete(ctx, r.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete group role %s/%s: %w", r.Namespace, r.Name, err)
		}
	}
	return nil
}

// GetGroupCustomClusterRoleRules returns the rules from the group's custom ClusterRole.
func (c *Client) GetGroupCustomClusterRoleRules(ctx context.Context, groupName string) ([]models.PolicyRule, error) {
	cr, err := c.Kubernetes.RbacV1().ClusterRoles().Get(ctx, groupResourceName(groupName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get group custom cluster role: %w", err)
	}
	return fromK8sRules(cr.Rules), nil
}

// GetGroupNamespaceRoleRules returns the rules from the group's custom Role in a namespace.
func (c *Client) GetGroupNamespaceRoleRules(ctx context.Context, groupName, namespace string) ([]models.PolicyRule, error) {
	r, err := c.Kubernetes.RbacV1().Roles(namespace).Get(ctx, groupResourceName(groupName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get group custom role: %w", err)
	}
	return fromK8sRules(r.Rules), nil
}
