package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SubmitAndApproveCSR creates a CertificateSigningRequest, approves it, and waits
// for the signed certificate. Returns the certificate PEM bytes.
func (c *Client) SubmitAndApproveCSR(ctx context.Context, username string, csrPEM []byte, annotations map[string]string) ([]byte, error) {
	name := resourceName(username)

	csr := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      managedLabels(username),
			Annotations: annotations,
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:    csrPEM,
			SignerName: certificatesv1.KubeAPIServerClientSignerName,
			Usages:     []certificatesv1.KeyUsage{certificatesv1.UsageClientAuth},
		},
	}

	created, err := c.Kubernetes.CertificatesV1().CertificateSigningRequests().Create(ctx, csr, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create csr: %w", err)
	}

	created.Status.Conditions = append(created.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:               certificatesv1.CertificateApproved,
		Status:             corev1.ConditionTrue,
		Reason:             "KubeValetApproved",
		Message:            "Approved by kubevalet",
		LastUpdateTime:     metav1.Now(),
	})

	_, err = c.Kubernetes.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, name, created, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("approve csr: %w", err)
	}

	return c.pollCertificate(ctx, name, 30*time.Second)
}

func (c *Client) pollCertificate(ctx context.Context, csrName string, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		current, err := c.Kubernetes.CertificatesV1().CertificateSigningRequests().Get(ctx, csrName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("get csr: %w", err)
		}
		if len(current.Status.Certificate) > 0 {
			return current.Status.Certificate, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for certificate to be signed")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (c *Client) GetCSR(ctx context.Context, username string) (*certificatesv1.CertificateSigningRequest, error) {
	csr, err := c.Kubernetes.CertificatesV1().CertificateSigningRequests().Get(ctx, resourceName(username), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get csr: %w", err)
	}
	return csr, nil
}

func (c *Client) ListManagedCSRs(ctx context.Context) ([]certificatesv1.CertificateSigningRequest, error) {
	list, err := c.Kubernetes.CertificatesV1().CertificateSigningRequests().List(ctx, metav1.ListOptions{
		LabelSelector: LabelManagedBy + "=" + LabelManagedByValue,
	})
	if err != nil {
		return nil, fmt.Errorf("list csrs: %w", err)
	}
	return list.Items, nil
}

// UpdateCSRAnnotations replaces all kubevalet.io/ annotations on the CSR with the provided map.
func (c *Client) UpdateCSRAnnotations(ctx context.Context, username string, annotations map[string]string) error {
	csr, err := c.Kubernetes.CertificatesV1().CertificateSigningRequests().Get(ctx, resourceName(username), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get csr: %w", err)
	}
	if csr.Annotations == nil {
		csr.Annotations = map[string]string{}
	}
	for k := range csr.Annotations {
		if strings.HasPrefix(k, "kubevalet.io/") {
			delete(csr.Annotations, k)
		}
	}
	for k, v := range annotations {
		csr.Annotations[k] = v
	}
	_, err = c.Kubernetes.CertificatesV1().CertificateSigningRequests().Update(ctx, csr, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update csr annotations: %w", err)
	}
	return nil
}

func (c *Client) DeleteCSR(ctx context.Context, username string) error {
	err := c.Kubernetes.CertificatesV1().CertificateSigningRequests().Delete(ctx, resourceName(username), metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete csr: %w", err)
	}
	return nil
}

// GetCAData returns the cluster CA certificate PEM.
func (c *Client) GetCAData() ([]byte, error) {
	ca := c.RestConfig.TLSClientConfig.CAData
	if len(ca) > 0 {
		return ca, nil
	}
	if c.RestConfig.TLSClientConfig.CAFile != "" {
		data, err := os.ReadFile(c.RestConfig.TLSClientConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca file: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("no CA data available in kubeconfig")
}

func managedLabels(username string) map[string]string {
	return map[string]string{
		LabelManagedBy: LabelManagedByValue,
		LabelUsername:  username,
	}
}
