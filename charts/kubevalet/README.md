# kubevalet

Lightweight Kubernetes user management with a web UI.

Creates x509 users via the Kubernetes CSR API, issues kubeconfigs, and manages RBAC bindings — no LDAP, no OIDC, no Dex.

## Features

- Create Kubernetes users (x509 / CSR API)
- Assign preset roles (`cluster-admin`, `admin`, `edit`, `view`) or define custom RBAC rules
- Multi-namespace scoped bindings per user
- **Groups** — manage k8s Group subjects with their own RBAC; users added to a group via x509 O field inherit permissions automatically
- **Groups-only users** — assign a user to groups without direct RBAC
- Download / view generated kubeconfigs
- **Graph view** — visualise any user's full access tree
- **PostgreSQL as source of truth** — all user state stored in postgres; Sync button recreates missing k8s objects from DB
- Kubeconfig API server address configurable at runtime from the Settings UI
- Simple username/password auth backed by PostgreSQL

## Install

```bash
helm install kubevalet oci://ghcr.io/truebad0ur/kubevalet \
  --version 0.3.3 \
  --namespace kubevalet --create-namespace \
  --set cluster.server=https://<your-api-server>:6443 \
  --set auth.adminPassword=changeme
```

## Access the UI

```bash
kubectl port-forward svc/kubevalet 8080:80 -n kubevalet
# open http://localhost:8080
```

## Key values

| Value | Default | Description |
|---|---|---|
| `cluster.server` | `https://kubernetes.default.svc.cluster.local` | API server URL embedded in kubeconfigs |
| `cluster.name` | `kubernetes` | Cluster name in kubeconfig context |
| `auth.adminPassword` | `admin` | Initial admin password — **change this** |
| `auth.jwtSecret` | _(auto-generated)_ | JWT signing secret |
| `postgres.password` | _(auto-generated)_ | PostgreSQL password |
| `postgres.persistence.enabled` | `false` | Enable PVC for PostgreSQL |
| `ingress.enabled` | `false` | Expose via Ingress |
| `networkPolicy.enabled` | `false` | Enable NetworkPolicy (requires CNI support) |

## Upgrade

```bash
helm upgrade kubevalet oci://ghcr.io/truebad0ur/kubevalet \
  --version 0.3.3 \
  --namespace kubevalet \
  --reuse-values
```

## Source

[github.com/TrueBad0ur/kubevalet](https://github.com/TrueBad0ur/kubevalet)
