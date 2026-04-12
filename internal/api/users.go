package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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

// ── Create ────────────────────────────────────────────────────────────────────

func (h *Handler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateName(req.Name); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	clusterCustom := len(req.Rules) > 0

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
	certPEM, err := h.k8s.SubmitAndApproveCSR(ctx, req.Name, kp.CSRPEM, annotations)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			respondError(c, http.StatusConflict, fmt.Errorf("user %q already exists", req.Name))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 3. Store private key in k8s Secret
	if err := h.k8s.StorePrivateKey(ctx, req.Name, h.cfg.Namespace, kp.PrivateKeyPEM); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 4. Create RBAC bindings (all optional — user may belong to groups only)
	if req.ClusterRole != "" {
		err = h.k8s.CreateClusterRoleBinding(ctx, req.Name, req.ClusterRole)
	} else if clusterCustom {
		err = h.k8s.CreateCustomClusterRole(ctx, req.Name, req.Rules)
	} else if len(req.NamespaceBindings) > 0 {
		err = h.k8s.CreateNamespaceBindings(ctx, req.Name, req.NamespaceBindings)
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// 5. Persist to postgres (source of truth)
	_ = h.upsertUserDB(ctx, req.Name, req.Groups, req.ClusterRole, clusterCustom,
		req.Rules, req.NamespaceBindings, string(certPEM), string(kp.PrivateKeyPEM))

	// 6. Build kubeconfig
	caData, err := h.k8s.GetCAData()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	kubeconfigYAML, err := kubeconfig.Build(kubeconfig.BuildParams{
		Username:      req.Name,
		ClusterName:   h.cfg.ClusterName,
		ClusterServer: h.clusterServer(ctx),
		ClusterCA:     caData,
		ClientCert:    certPEM,
		ClientKey:     kp.PrivateKeyPEM,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	var expiresAt *time.Time
	if t, err := cert.ParseExpiry(certPEM); err == nil {
		expiresAt = &t
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
			CertExpiresAt:     expiresAt,
		},
		Kubeconfig: string(kubeconfigYAML),
	})
}

// ── List ──────────────────────────────────────────────────────────────────────

func (h *Handler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// 1. Get all managed CSRs (one call — used for status + migration)
	csrs, err := h.k8s.ListManagedCSRs(ctx)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	csrByName := make(map[string]certificatesv1.CertificateSigningRequest, len(csrs))
	for _, csr := range csrs {
		csrByName[csr.Labels[k8s.LabelUsername]] = csr
	}

	// 2. Read from postgres
	rows, err := h.db.Query(ctx, `
		SELECT name, groups, cluster_role, custom_role, rules, ns_bindings, cert_pem, created_at
		FROM users ORDER BY name
	`)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	dbNames := make(map[string]bool)
	users := make([]models.User, 0)

	for rows.Next() {
		var (
			name        string
			groups      []string
			clusterRole string
			customRole  bool
			rulesJSON   []byte
			nsJSON      []byte
			certPEM     string
			createdAt   time.Time
		)
		if err := rows.Scan(&name, &groups, &clusterRole, &customRole, &rulesJSON, &nsJSON, &certPEM, &createdAt); err != nil {
			continue
		}
		dbNames[name] = true

		var rules []models.PolicyRule
		var nsBindings []models.NamespaceBinding
		_ = json.Unmarshal(rulesJSON, &rules)
		_ = json.Unmarshal(nsJSON, &nsBindings)

		// Status: prefer CSR conditions, fall back to "Active" if cert in DB
		status := "Active"
		if csr, ok := csrByName[name]; ok {
			status = csrStatusString(csr)
		} else if certPEM == "" {
			status = "Pending"
		}

		var expiresAt *time.Time
		if certPEM != "" {
			if t, err := cert.ParseExpiry([]byte(certPEM)); err == nil {
				expiresAt = &t
			}
		}

		users = append(users, models.User{
			Name:              name,
			Groups:            groups,
			ClusterRole:       clusterRole,
			CustomRole:        customRole,
			Rules:             rules,
			NamespaceBindings: nsBindings,
			Status:            status,
			CreatedAt:         createdAt,
			CertExpiresAt:     expiresAt,
		})
	}
	rows.Close()

	// 3. Migration: for any CSR-managed users not yet in postgres, import them
	for _, csr := range csrs {
		username := csr.Labels[k8s.LabelUsername]
		if dbNames[username] {
			continue
		}
		if u := h.importCSRUser(ctx, csr); u != nil {
			users = append(users, *u)
		}
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Name < users[j].Name
	})

	c.JSON(http.StatusOK, gin.H{"users": users, "total": len(users)})
}

// ── Delete ────────────────────────────────────────────────────────────────────

func (h *Handler) DeleteUser(c *gin.Context) {
	username := c.Param("name")
	ctx := c.Request.Context()

	// Determine RBAC type from DB first, fall back to CSR
	isClusterCustom, hasNsBindings := h.userRBACType(ctx, username)

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
	_ = hasNsBindings
	collect(h.k8s.DeleteAllNamespaceBindings(ctx, username))
	_, _ = h.db.Exec(ctx, "DELETE FROM users WHERE name=$1", username)

	if len(errs) > 0 {
		respondError(c, http.StatusInternalServerError, fmt.Errorf(strings.Join(errs, "; ")))
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Update RBAC ───────────────────────────────────────────────────────────────

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

	// Get old state (prefer DB, fall back to CSR)
	oldGroups, oldClusterCustom := h.getUserOldState(ctx, username)

	// Delete old k8s bindings
	var errs []string
	collect := func(e error) {
		if e != nil {
			errs = append(errs, e.Error())
		}
	}
	collect(h.k8s.DeleteClusterRoleBinding(ctx, username))
	if oldClusterCustom {
		collect(h.k8s.DeleteCustomClusterRole(ctx, username))
	}
	collect(h.k8s.DeleteAllNamespaceBindings(ctx, username))
	if len(errs) > 0 {
		respondError(c, http.StatusInternalServerError, fmt.Errorf(strings.Join(errs, "; ")))
		return
	}

	// Create new k8s bindings
	var err error
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

	newGroups := req.Groups

	// If groups changed → regenerate cert
	if groupsChanged(oldGroups, newGroups) {
		kp, err := cert.Generate(username, newGroups)
		if err != nil {
			respondError(c, http.StatusInternalServerError, fmt.Errorf("generate key pair: %w", err))
			return
		}

		if err := h.k8s.DeleteCSR(ctx, username); err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		time.Sleep(300 * time.Millisecond)

		newAnn := buildAnnotationsFromFields(newGroups, req.ClusterRole, clusterCustom, nsScoped, req.NamespaceBindings)
		certPEM, err := h.k8s.SubmitAndApproveCSR(ctx, username, kp.CSRPEM, newAnn)
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}

		_ = h.k8s.DeletePrivateKey(ctx, username, h.cfg.Namespace)
		if err := h.k8s.StorePrivateKey(ctx, username, h.cfg.Namespace, kp.PrivateKeyPEM); err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}

		// Update postgres with new cert + RBAC
		_ = h.upsertUserDB(ctx, username, newGroups, req.ClusterRole, clusterCustom,
			req.Rules, req.NamespaceBindings, string(certPEM), string(kp.PrivateKeyPEM))

		caData, err := h.k8s.GetCAData()
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		kubeconfigYAML, err := kubeconfig.Build(kubeconfig.BuildParams{
			Username:      username,
			ClusterName:   h.cfg.ClusterName,
			ClusterServer: h.clusterServer(ctx),
			ClusterCA:     caData,
			ClientCert:    certPEM,
			ClientKey:     kp.PrivateKeyPEM,
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}

		c.JSON(http.StatusOK, models.UpdateRBACResponse{Kubeconfig: string(kubeconfigYAML)})
		return
	}

	// Groups unchanged — update CSR annotations + postgres (no new cert)
	newAnn := buildAnnotationsFromFields(newGroups, req.ClusterRole, clusterCustom, nsScoped, req.NamespaceBindings)
	if err := h.k8s.UpdateCSRAnnotations(ctx, username, newAnn); err != nil {
		// CSR may not exist (was cleaned up) — not fatal since DB is source of truth
		_ = err
	}
	// Update RBAC fields in postgres; keep existing cert_pem / private_key_pem
	_ = h.upsertUserDB(ctx, username, newGroups, req.ClusterRole, clusterCustom,
		req.Rules, req.NamespaceBindings, "", "")

	c.JSON(http.StatusOK, models.UpdateRBACResponse{})
}

// ── Download kubeconfig ───────────────────────────────────────────────────────

func (h *Handler) DownloadKubeconfig(c *gin.Context) {
	username := c.Param("name")
	ctx := c.Request.Context()

	caData, err := h.k8s.GetCAData()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	// Try postgres first (preferred path)
	var certPEM, privateKeyPEM string
	dbErr := h.db.QueryRow(ctx,
		"SELECT cert_pem, private_key_pem FROM users WHERE name=$1", username).
		Scan(&certPEM, &privateKeyPEM)

	if dbErr == nil && certPEM != "" && privateKeyPEM != "" {
		// If Secret is missing, recreate it silently
		if _, serr := h.k8s.GetPrivateKey(ctx, username, h.cfg.Namespace); serr != nil {
			_ = h.k8s.StorePrivateKey(ctx, username, h.cfg.Namespace, []byte(privateKeyPEM))
		}

		kubeconfigYAML, err := kubeconfig.Build(kubeconfig.BuildParams{
			Username:      username,
			ClusterName:   h.cfg.ClusterName,
			ClusterServer: h.clusterServer(ctx),
			ClusterCA:     caData,
			ClientCert:    []byte(certPEM),
			ClientKey:     []byte(privateKeyPEM),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		c.Header("Content-Disposition", `attachment; filename="`+username+`.kubeconfig"`)
		c.Data(http.StatusOK, "application/x-yaml", kubeconfigYAML)
		return
	}

	// Fallback: read from CSR + Secret (pre-DB users)
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
			respondError(c, http.StatusNotFound, fmt.Errorf("private key for user %q is not available", username))
			return
		}
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	kubeconfigYAML, err := kubeconfig.Build(kubeconfig.BuildParams{
		Username:      username,
		ClusterName:   h.cfg.ClusterName,
		ClusterServer: h.clusterServer(ctx),
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

// ── Sync (repair missing k8s objects from DB) ─────────────────────────────────

func (h *Handler) SyncUser(c *gin.Context) {
	username := c.Param("name")
	ctx := c.Request.Context()

	var (
		groups        []string
		clusterRole   string
		customRole    bool
		rulesJSON     []byte
		nsJSON        []byte
		privateKeyPEM string
	)
	err := h.db.QueryRow(ctx, `
		SELECT groups, cluster_role, custom_role, rules, ns_bindings, private_key_pem
		FROM users WHERE name=$1
	`, username).Scan(&groups, &clusterRole, &customRole, &rulesJSON, &nsJSON, &privateKeyPEM)
	if err != nil {
		respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found in database", username))
		return
	}

	var rules []models.PolicyRule
	var nsBindings []models.NamespaceBinding
	_ = json.Unmarshal(rulesJSON, &rules)
	_ = json.Unmarshal(nsJSON, &nsBindings)

	repaired := []string{}

	// Ensure Secret (private key) exists
	if privateKeyPEM != "" {
		if _, serr := h.k8s.GetPrivateKey(ctx, username, h.cfg.Namespace); serr != nil {
			if err := h.k8s.StorePrivateKey(ctx, username, h.cfg.Namespace, []byte(privateKeyPEM)); err != nil {
				respondError(c, http.StatusInternalServerError, fmt.Errorf("recreate secret: %w", err))
				return
			}
			repaired = append(repaired, "secret")
		}
	}

	// Recreate RBAC: delete all then recreate from DB (repair operation, brief gap is acceptable)
	_ = h.k8s.DeleteClusterRoleBinding(ctx, username)
	if customRole && len(nsBindings) == 0 {
		_ = h.k8s.DeleteCustomClusterRole(ctx, username)
	}
	_ = h.k8s.DeleteAllNamespaceBindings(ctx, username)

	var rbacErr error
	if clusterRole != "" {
		rbacErr = h.k8s.CreateClusterRoleBinding(ctx, username, clusterRole)
	} else if customRole && len(nsBindings) == 0 {
		rbacErr = h.k8s.CreateCustomClusterRole(ctx, username, rules)
	} else if len(nsBindings) > 0 {
		rbacErr = h.k8s.CreateNamespaceBindings(ctx, username, nsBindings)
	}
	if rbacErr != nil {
		respondError(c, http.StatusInternalServerError, fmt.Errorf("recreate rbac: %w", rbacErr))
		return
	}
	repaired = append(repaired, "rbac")

	c.JSON(http.StatusOK, gin.H{"repaired": repaired})
}

// ── DB helpers ────────────────────────────────────────────────────────────────

// upsertUserDB persists user state to postgres.
// certPEM/privateKeyPEM are only updated when non-empty (to avoid overwriting on RBAC-only updates).
func (h *Handler) upsertUserDB(ctx context.Context, name string, groups []string,
	clusterRole string, customRole bool, rules []models.PolicyRule,
	nsBindings []models.NamespaceBinding, certPEM, privateKeyPEM string) error {

	rulesJSON, _ := json.Marshal(rules)
	nsJSON, _ := json.Marshal(nsBindings)
	if rulesJSON == nil {
		rulesJSON = []byte("[]")
	}
	if nsJSON == nil {
		nsJSON = []byte("[]")
	}
	if groups == nil {
		groups = []string{}
	}

	_, err := h.db.Exec(ctx, `
		INSERT INTO users (name, groups, cluster_role, custom_role, rules, ns_bindings, cert_pem, private_key_pem)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (name) DO UPDATE SET
			groups          = EXCLUDED.groups,
			cluster_role    = EXCLUDED.cluster_role,
			custom_role     = EXCLUDED.custom_role,
			rules           = EXCLUDED.rules,
			ns_bindings     = EXCLUDED.ns_bindings,
			cert_pem        = CASE WHEN EXCLUDED.cert_pem        != '' THEN EXCLUDED.cert_pem        ELSE users.cert_pem        END,
			private_key_pem = CASE WHEN EXCLUDED.private_key_pem != '' THEN EXCLUDED.private_key_pem ELSE users.private_key_pem END
	`, name, groups, clusterRole, customRole, rulesJSON, nsJSON, certPEM, privateKeyPEM)
	return err
}

// importCSRUser migrates a CSR-tracked user into postgres and returns it.
func (h *Handler) importCSRUser(ctx context.Context, csr certificatesv1.CertificateSigningRequest) *models.User {
	username := csr.Labels[k8s.LabelUsername]
	ann := csr.Annotations

	var groups []string
	if g := ann[k8s.AnnotationGroups]; g != "" {
		groups = strings.Split(g, ",")
	}

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

	isClusterCustom := ann[k8s.AnnotationCustomRole] == "true" &&
		ann[k8s.AnnotationNamespace] == "" &&
		ann[k8s.AnnotationNamespaceBindings] == ""

	clusterRole := ann[k8s.AnnotationClusterRole]

	// Fetch custom role rules
	var rules []models.PolicyRule
	if isClusterCustom {
		rules, _ = h.k8s.GetCustomRoleRules(ctx, username, "")
	}
	for i, nb := range nsBindings {
		if nb.CustomRole {
			nsBindings[i].Rules, _ = h.k8s.GetCustomRoleRules(ctx, username, nb.Namespace)
		}
	}

	certPEM := string(csr.Status.Certificate)
	privateKey, _ := h.k8s.GetPrivateKey(ctx, username, h.cfg.Namespace)

	_ = h.upsertUserDB(ctx, username, groups, clusterRole, isClusterCustom,
		rules, nsBindings, certPEM, string(privateKey))

	var expiresAt *time.Time
	if certPEM != "" {
		if t, err := cert.ParseExpiry([]byte(certPEM)); err == nil {
			expiresAt = &t
		}
	}

	return &models.User{
		Name:              username,
		Groups:            groups,
		ClusterRole:       clusterRole,
		CustomRole:        isClusterCustom,
		Rules:             rules,
		NamespaceBindings: nsBindings,
		Status:            csrStatusString(csr),
		CreatedAt:         csr.CreationTimestamp.Time,
		CertExpiresAt:     expiresAt,
	}
}

// getUserOldState returns groups and customRole flag for a user, preferring DB over CSR.
func (h *Handler) getUserOldState(ctx context.Context, username string) (groups []string, clusterCustom bool) {
	var customRole bool
	var groupsArr []string
	err := h.db.QueryRow(ctx, "SELECT groups, custom_role FROM users WHERE name=$1", username).
		Scan(&groupsArr, &customRole)
	if err == nil {
		return groupsArr, customRole
	}
	// Fallback: read from CSR
	csr, err := h.k8s.GetCSR(ctx, username)
	if err != nil {
		return nil, false
	}
	ann := csr.Annotations
	if g := ann[k8s.AnnotationGroups]; g != "" {
		groups = strings.Split(g, ",")
	}
	clusterCustom = ann[k8s.AnnotationCustomRole] == "true" &&
		ann[k8s.AnnotationNamespace] == "" &&
		ann[k8s.AnnotationNamespaceBindings] == ""
	return groups, clusterCustom
}

// userRBACType returns (isClusterCustom, hasNsBindings) for delete logic.
func (h *Handler) userRBACType(ctx context.Context, username string) (isClusterCustom, hasNsBindings bool) {
	var customRole bool
	var nsJSON []byte
	err := h.db.QueryRow(ctx, "SELECT custom_role, ns_bindings FROM users WHERE name=$1", username).
		Scan(&customRole, &nsJSON)
	if err == nil {
		var nsBindings []models.NamespaceBinding
		_ = json.Unmarshal(nsJSON, &nsBindings)
		return customRole && len(nsBindings) == 0, len(nsBindings) > 0
	}
	// Fallback: CSR
	csr, err := h.k8s.GetCSR(ctx, username)
	if err != nil {
		return false, false
	}
	ann := csr.Annotations
	isCC := ann[k8s.AnnotationCustomRole] == "true" &&
		ann[k8s.AnnotationNamespace] == "" &&
		ann[k8s.AnnotationNamespaceBindings] == ""
	return isCC, false
}

// ── Annotation helpers ────────────────────────────────────────────────────────

func buildAnnotations(req models.CreateUserRequest) map[string]string {
	return buildAnnotationsFromFields(req.Groups, req.ClusterRole,
		len(req.Rules) > 0, len(req.NamespaceBindings) > 0, req.NamespaceBindings)
}

func buildAnnotationsFromFields(groups []string, clusterRole string, clusterCustom, nsScoped bool,
	nsBindings []models.NamespaceBinding) map[string]string {

	ann := map[string]string{}
	if len(groups) > 0 {
		ann[k8s.AnnotationGroups] = strings.Join(groups, ",")
	}
	if clusterRole != "" {
		ann[k8s.AnnotationClusterRole] = clusterRole
	}
	if clusterCustom {
		ann[k8s.AnnotationCustomRole] = "true"
	}
	if nsScoped {
		if data, err := json.Marshal(compactNsBindings(nsBindings)); err == nil {
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

func groupsChanged(old, new []string) bool {
	cp := func(s []string) []string {
		c := make([]string, len(s))
		copy(c, s)
		sort.Strings(c)
		return c
	}
	o, n := cp(old), cp(new)
	if len(o) != len(n) {
		return true
	}
	for i := range o {
		if o[i] != n[i] {
			return true
		}
	}
	return false
}

// ── Renew certificate ─────────────────────────────────────────────────────────

func (h *Handler) RenewCertificate(c *gin.Context) {
	username := c.Param("name")
	ctx := c.Request.Context()

	var (
		groups      []string
		clusterRole string
		customRole  bool
		rulesJSON   []byte
		nsJSON      []byte
	)
	err := h.db.QueryRow(ctx, `
		SELECT groups, cluster_role, custom_role, rules, ns_bindings
		FROM users WHERE name=$1
	`, username).Scan(&groups, &clusterRole, &customRole, &rulesJSON, &nsJSON)
	if err != nil {
		respondError(c, http.StatusNotFound, fmt.Errorf("user %q not found", username))
		return
	}

	var rules []models.PolicyRule
	var nsBindings []models.NamespaceBinding
	_ = json.Unmarshal(rulesJSON, &rules)
	_ = json.Unmarshal(nsJSON, &nsBindings)

	kp, err := cert.Generate(username, groups)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	if err := h.k8s.DeleteCSR(ctx, username); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	time.Sleep(300 * time.Millisecond)

	ann := buildAnnotationsFromFields(groups, clusterRole, customRole, len(nsBindings) > 0, nsBindings)
	certPEM, err := h.k8s.SubmitAndApproveCSR(ctx, username, kp.CSRPEM, ann)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	_ = h.k8s.DeletePrivateKey(ctx, username, h.cfg.Namespace)
	if err := h.k8s.StorePrivateKey(ctx, username, h.cfg.Namespace, kp.PrivateKeyPEM); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	_ = h.upsertUserDB(ctx, username, groups, clusterRole, customRole, rules, nsBindings,
		string(certPEM), string(kp.PrivateKeyPEM))

	caData, err := h.k8s.GetCAData()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	kubeconfigYAML, err := kubeconfig.Build(kubeconfig.BuildParams{
		Username:      username,
		ClusterName:   h.cfg.ClusterName,
		ClusterServer: h.clusterServer(ctx),
		ClusterCA:     caData,
		ClientCert:    certPEM,
		ClientKey:     kp.PrivateKeyPEM,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	expiresAt, _ := cert.ParseExpiry(certPEM)
	c.JSON(http.StatusOK, models.RenewCertificateResponse{
		Kubeconfig:    string(kubeconfigYAML),
		CertExpiresAt: expiresAt,
	})
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
