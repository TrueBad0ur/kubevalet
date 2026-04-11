package k8s

const (
	// Labels
	LabelManagedBy = "app.kubernetes.io/managed-by"
	LabelManagedByValue = "kubevalet"
	LabelUsername  = "kubevalet.io/username"

	// Annotations — stored on the CSR to reconstruct User on list
	AnnotationGroups      = "kubevalet.io/groups"
	AnnotationClusterRole = "kubevalet.io/cluster-role"
	AnnotationNamespace   = "kubevalet.io/namespace"
	AnnotationRole        = "kubevalet.io/role"
	AnnotationCustomRole        = "kubevalet.io/custom-role"         // "true" when cluster-wide custom Role was created
	AnnotationNamespaceBindings = "kubevalet.io/namespace-bindings"  // JSON array of {namespace,role?,customRole?}

	// Naming prefix for all managed k8s objects
	ResourcePrefix = "kubevalet-"
)

func resourceName(username string) string {
	return ResourcePrefix + username
}
