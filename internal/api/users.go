package api

import (
	"encoding/json"
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

	clusterCustom := len(req.Rules) > 0
	nsScoped := len(req.NamespaceBindings) > 0

	if req.ClusterRole == "" && !clusterCustom && !nsScoped {
		respondError(c, http.StatusBadRequest, fmt.Errorf("provide clusterRole, rules (cluster-wide), or namespaceBindings"))
		return
	}
	for _, nb := range req.NamespaceBindings {
		if nb.Namespace == "" {
			respondError(c, http.StatusBadRequest, fmt.Errorf("each namespaceBinding must have a namespace"))
			return
		}
		if nb.Role == "" && len(nb.Rules) == 0 {
			respondError(c, http.StatusBadRequest, fmt.Errorf("each namespaceBinding must have role or rules"))
			return
		}
	}

	// 1. Generate private key + CSR
	kp, err := cert.Generate(req.Name, req.Groups)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 2. Submit CSR
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

	// 3. Store private key
	if err := h.k8s.StorePrivateKey(c.Request.Context(), req.Name, h.cfg.Namespace, kp.PrivateKeyPEM); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 4. Create RBAC bindings
	if req.ClusterRole != "" {
		err = h.k8s.CreateClusterRoleBinding(c.Request.Context(), req.Name, req.ClusterRole)
	} else if clusterCustom {
		err = h.k8s.CreateCustomClusterRole(c.Request.Context(), req.Name, req.Rules)
	} else {
		err = h.k8s.CreateNamespaceBindings(c.Request.Context(), req.Name, req.NamespaceBindings)
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
			Name:              req.Name,
			Groups:            req.Groups,
			ClusterRole:       req.ClusterRole,
			CustomRole:        clusterCustom,
			NamespaceBindings: req.NamespaceBindings,
			Status:            "Active",
			CreatedAt:         time.Now().UTC(),
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

		// Namespace bindings: new JSON format or backward-compat single-namespace
		var nsBindings []models.NamespaceBinding
		if nb := ann[k8s.AnnotationNamespaceBindings]; nb != "" {
			nsBindings = decodeNsBindings(nb)
		} else if oldNs := ann[k8s.AnnotationNamespace]; oldNs != "" {
			isCustom := ann[k8s.AnnotationCustomRole] == "true"
			nsBindings = []models.NamespaceBinding{{
				Namespace:  oldNs,
				Role:       ann[k8s.AnnotationRole],
				CustomRole: isCustom,
			}}
		}

		// Cluster-wide custom: customRole=true with no namespace annotations (old or new)
		isClusterCustom := ann[k8s.AnnotationCustomRole] == "true" &&
			ann[k8s.AnnotationNamespace] == "" &&
			ann[k8s.AnnotationNamespaceBindings] == ""

		u := models.User{
			Name:              username,
			Groups:            groups,
			ClusterRole:       ann[k8s.AnnotationClusterRole],
			CustomRole:        isClusterCustom,
			NamespaceBindings: nsBindings,
			Status:            csrStatusString(csr),
			CreatedAt:         csr.CreationTimestamp.Time,
		}

		// Fetch rules for cluster-wide custom role
		if u.CustomRole {
			if rules, err := h.k8s.GetCustomRoleRules(c.Request.Context(), username, ""); err == nil {
				u.Rules = rules
			}
		}
		// Fetch rules for custom namespace bindings
		for i, nb := range u.NamespaceBindings {
			if nb.CustomRole {
				if rules, err := h.k8s.GetCustomRoleRules(c.Request.Context(), username, nb.Namespace); err == nil {
					u.NamespaceBindings[i].Rules = rules
				}
			}
		}

		users = append(users, u)
	}

	c.JSON(http.StatusOK, gin.H{"users": users, "total": len(users)})
}

func (h *Handler) DeleteUser(c *gin.Context) {
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

	ann := csr.Annotations
	// Cluster-wide custom role (old or new format)
	isClusterCustom := ann[k8s.AnnotationCustomRole] == "true" &&
		ann[k8s.AnnotationNamespace] == "" &&
		ann[k8s.AnnotationNamespaceBindings] == ""

	var errs []string
	collect := func(e error) {
		if e != nil {
			errs = append(errs, e.Error())
		}
	}
	collect(h.k8s.DeleteCSR(ctx, username))
	collect(h.k8s.DeletePrivateKey(ctx, username, h.cfg.Namespace))
	collect(h.k8s.DeleteClusterRoleBinding(ctx, username))
	if isClusterCustom {
		collect(h.k8s.DeleteCustomClusterRole(ctx, username))
	}
	collect(h.k8s.DeleteAllNamespaceBindings(ctx, username))

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

	clusterCustom := len(req.Rules) > 0
	nsScoped := len(req.NamespaceBindings) > 0

	if req.ClusterRole == "" && !clusterCustom && !nsScoped {
		respondError(c, http.StatusBadRequest, fmt.Errorf("provide clusterRole, rules, or namespaceBindings"))
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

	ann := csr.Annotations
	isOldClusterCustom := ann[k8s.AnnotationCustomRole] == "true" &&
		ann[k8s.AnnotationNamespace] == "" &&
		ann[k8s.AnnotationNamespaceBindings] == ""

	// Delete old bindings
	var errs []string
	collect := func(e error) {
		if e != nil {
			errs = append(errs, e.Error())
		}
	}
	collect(h.k8s.DeleteClusterRoleBinding(ctx, username))
	if isOldClusterCustom {
		collect(h.k8s.DeleteCustomClusterRole(ctx, username))
	}
	collect(h.k8s.DeleteAllNamespaceBindings(ctx, username))
	if len(errs) > 0 {
		respondError(c, http.StatusInternalServerError, fmt.Errorf(strings.Join(errs, "; ")))
		return
	}

	// Create new bindings
	if req.ClusterRole != "" {
		err = h.k8s.CreateClusterRoleBinding(ctx, username, req.ClusterRole)
	} else if clusterCustom {
		err = h.k8s.CreateCustomClusterRole(ctx, username, req.Rules)
	} else {
		err = h.k8s.CreateNamespaceBindings(ctx, username, req.NamespaceBindings)
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// Update CSR annotations
	newAnn := map[string]string{}
	if g := ann[k8s.AnnotationGroups]; g != "" {
		newAnn[k8s.AnnotationGroups] = g
	}
	if req.ClusterRole != "" {
		newAnn[k8s.AnnotationClusterRole] = req.ClusterRole
	}
	if clusterCustom {
		newAnn[k8s.AnnotationCustomRole] = "true"
	}
	if nsScoped {
		if data, jerr := json.Marshal(compactNsBindings(req.NamespaceBindings)); jerr == nil {
			newAnn[k8s.AnnotationNamespaceBindings] = string(data)
		}
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
	if len(req.Rules) > 0 {
		ann[k8s.AnnotationCustomRole] = "true"
	}
	if len(req.NamespaceBindings) > 0 {
		if data, err := json.Marshal(compactNsBindings(req.NamespaceBindings)); err == nil {
			ann[k8s.AnnotationNamespaceBindings] = string(data)
		}
	}
	return ann
}

type compactBinding struct {
	Namespace  string `json:"namespace"`
	Role       string `json:"role,omitempty"`
	CustomRole bool   `json:"customRole,omitempty"`
}

func compactNsBindings(bindings []models.NamespaceBinding) []compactBinding {
	result := make([]compactBinding, len(bindings))
	for i, b := range bindings {
		result[i] = compactBinding{
			Namespace:  b.Namespace,
			Role:       b.Role,
			CustomRole: b.CustomRole || len(b.Rules) > 0,
		}
	}
	return result
}

func decodeNsBindings(s string) []models.NamespaceBinding {
	var items []compactBinding
	if err := json.Unmarshal([]byte(s), &items); err != nil {
		return nil
	}
	result := make([]models.NamespaceBinding, len(items))
	for i, it := range items {
		result[i] = models.NamespaceBinding{
			Namespace:  it.Namespace,
			Role:       it.Role,
			CustomRole: it.CustomRole,
		}
	}
	return result
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
