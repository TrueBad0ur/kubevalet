package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	certificatesv1 "k8s.io/api/certificates/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/kubevalet/kubevalet/internal/cert"
	"github.com/kubevalet/kubevalet/internal/k8s"
	"github.com/kubevalet/kubevalet/internal/kubeconfig"
	"github.com/kubevalet/kubevalet/internal/models"
)

func (h *Handler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	advanced := len(req.Rules) > 0
	if !advanced {
		if req.ClusterRole == "" && (req.Namespace == "" || req.Role == "") {
			respondError(c, http.StatusBadRequest, fmt.Errorf("provide either clusterRole, namespace+role, or rules"))
			return
		}
	} else {
		// namespace may be empty (cluster-wide custom role)
	}

	// 1. Generate private key + CSR
	kp, err := cert.Generate(req.Name, req.Groups)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 2. Submit CSR to k8s, approve it, wait for signed certificate
	annotations := buildAnnotations(req)
	certPEM, err := h.k8s.SubmitAndApproveCSR(c.Request.Context(), req.Name, kp.CSRPEM, annotations)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			respondError(c, http.StatusConflict, fmt.Errorf("user %q already exists", req.Name))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 3. Persist private key in a Secret (only copy — never logged or returned raw)
	if err := h.k8s.StorePrivateKey(c.Request.Context(), req.Name, h.cfg.Namespace, kp.PrivateKeyPEM); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 4. Create RBAC binding
	if advanced {
		if req.Namespace != "" {
			err = h.k8s.CreateCustomRole(c.Request.Context(), req.Name, req.Namespace, req.Rules)
		} else {
			err = h.k8s.CreateCustomClusterRole(c.Request.Context(), req.Name, req.Rules)
		}
	} else if req.ClusterRole != "" {
		err = h.k8s.CreateClusterRoleBinding(c.Request.Context(), req.Name, req.ClusterRole)
	} else {
		err = h.k8s.CreateRoleBinding(c.Request.Context(), req.Name, req.Namespace, req.Role)
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 5. Build kubeconfig
	caData, err := h.k8s.GetCAData()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	kubeconfigYAML, err := kubeconfig.Build(kubeconfig.BuildParams{
		Username:      req.Name,
		ClusterName:   h.cfg.ClusterName,
		ClusterServer: h.clusterServer(),
		ClusterCA:     caData,
		ClientCert:    certPEM,
		ClientKey:     kp.PrivateKeyPEM,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, models.CreateUserResponse{
		User: models.User{
			Name:        req.Name,
			Groups:      req.Groups,
			ClusterRole: req.ClusterRole,
			Namespace:   req.Namespace,
			Role:        req.Role,
			CustomRole:  advanced,
			Status:      "Active",
			CreatedAt:   time.Now().UTC(),
		},
		Kubeconfig: string(kubeconfigYAML),
	})
}

func (h *Handler) ListUsers(c *gin.Context) {
	csrs, err := h.k8s.ListManagedCSRs(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	users := make([]models.User, 0, len(csrs))
	for _, csr := range csrs {
		username := csr.Labels[k8s.LabelUsername]
		ann := csr.Annotations

		var groups []string
		if g := ann[k8s.AnnotationGroups]; g != "" {
			groups = strings.Split(g, ",")
		}

		u := models.User{
			Name:        username,
			Groups:      groups,
			ClusterRole: ann[k8s.AnnotationClusterRole],
			Namespace:   ann[k8s.AnnotationNamespace],
			Role:        ann[k8s.AnnotationRole],
			CustomRole:  ann[k8s.AnnotationCustomRole] == "true",
			Status:      csrStatusString(csr),
			CreatedAt:   csr.CreationTimestamp.Time,
		}
		if u.CustomRole {
			rules, err := h.k8s.GetCustomRoleRules(c.Request.Context(), username, u.Namespace)
			if err == nil {
				u.Rules = rules
			}
		}
		users = append(users, u)
	}

	c.JSON(http.StatusOK, gin.H{"users": users, "total": len(users)})
}

func (h *Handler) DeleteUser(c *gin.Context) {
	username := c.Param("name")
	ctx := c.Request.Context()

	// Read CSR first to find the namespace for RoleBinding cleanup
	csr, err := h.k8s.GetCSR(ctx, username)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found", username))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	ns := csr.Annotations[k8s.AnnotationNamespace]
	customRole := csr.Annotations[k8s.AnnotationCustomRole] == "true"

	var errs []string
	for _, fn := range []func() error{
		func() error { return h.k8s.DeleteCSR(ctx, username) },
		func() error { return h.k8s.DeletePrivateKey(ctx, username, h.cfg.Namespace) },
		func() error { return h.k8s.DeleteClusterRoleBinding(ctx, username) },
		func() error {
			if ns == "" {
				return nil
			}
			return h.k8s.DeleteRoleBinding(ctx, username, ns)
		},
		func() error {
			if !customRole {
				return nil
			}
			if ns != "" {
				return h.k8s.DeleteCustomRole(ctx, username, ns)
			}
			return h.k8s.DeleteCustomClusterRole(ctx, username)
		},
	} {
		if err := fn(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		respondError(c, http.StatusInternalServerError, fmt.Errorf(strings.Join(errs, "; ")))
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) UpdateUserRBAC(c *gin.Context) {
	username := c.Param("name")
	ctx := c.Request.Context()

	var req models.UpdateRBACRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	advanced := len(req.Rules) > 0
	if !advanced && req.ClusterRole == "" && (req.Namespace == "" || req.Role == "") {
		respondError(c, http.StatusBadRequest, fmt.Errorf("provide clusterRole, namespace+role, or rules"))
		return
	}

	csr, err := h.k8s.GetCSR(ctx, username)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found", username))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	oldNs := csr.Annotations[k8s.AnnotationNamespace]
	oldCustom := csr.Annotations[k8s.AnnotationCustomRole] == "true"

	// Remove old bindings
	var errs []string
	collect := func(e error) {
		if e != nil {
			errs = append(errs, e.Error())
		}
	}
	collect(h.k8s.DeleteClusterRoleBinding(ctx, username))
	if oldNs != "" {
		collect(h.k8s.DeleteRoleBinding(ctx, username, oldNs))
	}
	if oldCustom {
		if oldNs != "" {
			collect(h.k8s.DeleteCustomRole(ctx, username, oldNs))
		} else {
			collect(h.k8s.DeleteCustomClusterRole(ctx, username))
		}
	}
	if len(errs) > 0 {
		respondError(c, http.StatusInternalServerError, fmt.Errorf(strings.Join(errs, "; ")))
		return
	}

	// Create new bindings
	if advanced {
		if req.Namespace != "" {
			err = h.k8s.CreateCustomRole(ctx, username, req.Namespace, req.Rules)
		} else {
			err = h.k8s.CreateCustomClusterRole(ctx, username, req.Rules)
		}
	} else if req.ClusterRole != "" {
		err = h.k8s.CreateClusterRoleBinding(ctx, username, req.ClusterRole)
	} else {
		err = h.k8s.CreateRoleBinding(ctx, username, req.Namespace, req.Role)
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// Update CSR annotations (preserve groups)
	newAnn := map[string]string{}
	if g := csr.Annotations[k8s.AnnotationGroups]; g != "" {
		newAnn[k8s.AnnotationGroups] = g
	}
	if req.ClusterRole != "" {
		newAnn[k8s.AnnotationClusterRole] = req.ClusterRole
	}
	if req.Namespace != "" {
		newAnn[k8s.AnnotationNamespace] = req.Namespace
	}
	if req.Role != "" {
		newAnn[k8s.AnnotationRole] = req.Role
	}
	if advanced {
		newAnn[k8s.AnnotationCustomRole] = "true"
	}
	if err := h.k8s.UpdateCSRAnnotations(ctx, username, newAnn); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) DownloadKubeconfig(c *gin.Context) {
	username := c.Param("name")
	ctx := c.Request.Context()

	csr, err := h.k8s.GetCSR(ctx, username)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found", username))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	if len(csr.Status.Certificate) == 0 {
		respondError(c, http.StatusConflict, fmt.Errorf("certificate for user %q is not ready", username))
		return
	}

	privateKey, err := h.k8s.GetPrivateKey(ctx, username, h.cfg.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(c, http.StatusNotFound, fmt.Errorf("private key for user %q is not available (user was created before key storage was introduced)", username))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	caData, err := h.k8s.GetCAData()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	kubeconfigYAML, err := kubeconfig.Build(kubeconfig.BuildParams{
		Username:      username,
		ClusterName:   h.cfg.ClusterName,
		ClusterServer: h.clusterServer(),
		ClusterCA:     caData,
		ClientCert:    csr.Status.Certificate,
		ClientKey:     privateKey,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.Header("Content-Disposition", `attachment; filename="`+username+`.kubeconfig"`)
	c.Data(http.StatusOK, "application/x-yaml", kubeconfigYAML)
}

// --- helpers ---

func buildAnnotations(req models.CreateUserRequest) map[string]string {
	ann := map[string]string{}
	if len(req.Groups) > 0 {
		ann[k8s.AnnotationGroups] = strings.Join(req.Groups, ",")
	}
	if req.ClusterRole != "" {
		ann[k8s.AnnotationClusterRole] = req.ClusterRole
	}
	if req.Namespace != "" {
		ann[k8s.AnnotationNamespace] = req.Namespace
	}
	if req.Role != "" {
		ann[k8s.AnnotationRole] = req.Role
	}
	if len(req.Rules) > 0 {
		ann[k8s.AnnotationCustomRole] = "true"
	}
	return ann
}

func csrStatusString(csr certificatesv1.CertificateSigningRequest) string {
	for _, cond := range csr.Status.Conditions {
		if cond.Type == certificatesv1.CertificateDenied {
			return "Denied"
		}
		if cond.Type == certificatesv1.CertificateFailed {
			return "Failed"
		}
	}
	if len(csr.Status.Certificate) > 0 {
		return "Active"
	}
	return "Pending"
}
