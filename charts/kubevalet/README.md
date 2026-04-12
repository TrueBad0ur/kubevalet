# kubevalet

Lightweight Kubernetes user management with a web UI.

Creates x509 users via the Kubernetes CSR API, issues kubeconfigs, and manages RBAC bindings — no LDAP, no OIDC, no Dex.

## Overview

kubevalet solves a common problem: giving people access to a Kubernetes cluster without setting up a full identity provider. It generates x509 certificates using the Kubernetes CSR API, stores all state in PostgreSQL, and exposes a clean web UI for day-to-day user management.

## Features

- **x509 users** — create Kubernetes users via the CSR API; private keys stored in cluster Secrets and PostgreSQL, never exposed raw
- **RBAC management** — assign preset cluster roles (`cluster-admin`, `admin`, `edit`, `view`), define custom rules (API groups, resources, verbs), or configure per-namespace bindings
- **Groups** — manage k8s `Group` subjects with their own RBAC; any user whose x509 cert includes a group name in the `O` field automatically inherits group permissions
- **Groups-only users** — create a user that belongs to groups without any direct RBAC binding
- **Kubeconfig download** — view and download generated kubeconfigs directly from the UI; API server address configurable at runtime from Settings (no redeploy needed)
- **Graph view** — visualise any user's full access tree: cluster-wide role and all per-namespace bindings
- **PostgreSQL as source of truth** — all user state (RBAC config, cert PEM, private key) stored in PostgreSQL; **Sync** button recreates any missing k8s objects from DB without touching existing ones
- **Bundled PostgreSQL** — includes an optional PostgreSQL StatefulSet so you can get started with a single `helm install`; swap for an external instance anytime

## Prerequisites

- Kubernetes 1.24+ (CSR API v1)
- Helm 3.8+ (OCI support)
- A CNI that does not block pod-to-API-server traffic

## Install

```bash
helm install kubevalet oci://ghcr.io/truebad0ur/kubevalet \
  --version 0.3.11 \
  --namespace kubevalet --create-namespace \
  --set cluster.server=https://<your-api-server>:6443 \
  --set auth.adminPassword=changeme
```

> `cluster.server` is the API server address that will be embedded in generated kubeconfigs. Set it to the address your users connect to from outside the cluster. It can also be changed at runtime in the Settings UI.

## Access the UI

```bash
kubectl port-forward svc/kubevalet 8080:80 -n kubevalet
# open http://localhost:8080
```

Default credentials: `admin` / value of `auth.adminPassword`.

## Upgrade

```bash
helm upgrade kubevalet oci://ghcr.io/truebad0ur/kubevalet \
  --version 0.3.11 \
  --namespace kubevalet \
  --reuse-values
```

JWT secret and PostgreSQL password are preserved across upgrades automatically via Secret lookup — they are generated once on first install and never regenerated unless the Secret is deleted.

## Expose via Ingress

```bash
helm upgrade kubevalet oci://ghcr.io/truebad0ur/kubevalet \
  --namespace kubevalet --reuse-values \
  --set ingress.enabled=true \
  --set ingress.host=kubevalet.example.com \
  --set ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod
```

> Always enable TLS in production — credentials and JWT tokens are transmitted in plain HTTP otherwise.

## Use external PostgreSQL

```bash
helm install kubevalet oci://ghcr.io/truebad0ur/kubevalet \
  --namespace kubevalet --create-namespace \
  --set postgres.enabled=false \
  --set postgres.external.dsn="postgres://user:pass@host:5432/kubevalet?sslmode=require" \
  --set cluster.server=https://<your-api-server>:6443 \
  --set auth.adminPassword=changeme
```

## Enable NetworkPolicy

Requires a CNI that enforces NetworkPolicy (Calico, Cilium, Weave, etc.):

```bash
helm upgrade kubevalet oci://ghcr.io/truebad0ur/kubevalet \
  --namespace kubevalet --reuse-values \
  --set networkPolicy.enabled=true
```

This creates policies that restrict kubevalet pods to only talk to PostgreSQL, the Kubernetes API, and DNS. PostgreSQL pods only accept connections from kubevalet.

## Run without Helm

```bash
export POSTGRES_DSN=postgres://kubevalet:pass@localhost:5432/kubevalet
export JWT_SECRET=your-random-secret
export CLUSTER_SERVER=https://your-api:6443
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD=changeme
# optional:
export KUBECONFIG=/path/to/kubeconfig   # defaults to in-cluster service account
export NAMESPACE=kubevalet              # namespace where Secrets are stored
export TOKEN_TTL=24h

./bin/kubevalet
```

## Key values

| Value | Default | Description |
|---|---|---|
| `cluster.server` | `https://kubernetes.default.svc.cluster.local` | API server URL embedded in kubeconfigs — set to the external address users connect to |
| `cluster.name` | `kubernetes` | Cluster name in kubeconfig context |
| `auth.adminPassword` | `admin` | Initial admin password — **change this** |
| `auth.jwtSecret` | _(auto-generated)_ | JWT signing secret — generated on first install, preserved across upgrades |
| `auth.tokenTTL` | `24h` | JWT token lifetime |
| `postgres.enabled` | `true` | Deploy bundled PostgreSQL StatefulSet |
| `postgres.password` | _(auto-generated)_ | PostgreSQL password — generated on first install, preserved across upgrades |
| `postgres.persistence.enabled` | `false` | Enable PVC for PostgreSQL — data is lost on pod restart if disabled |
| `postgres.persistence.size` | `2Gi` | PVC size |
| `ingress.enabled` | `false` | Expose via Ingress |
| `ingress.host` | `kubevalet.example.com` | Ingress hostname |
| `ingress.className` | `nginx` | IngressClass name |
| `networkPolicy.enabled` | `false` | Create NetworkPolicy resources |
| `resources` | see values.yaml | CPU/memory requests and limits for the kubevalet container |

## Architecture

```
web browser
    │  JWT (Bearer token)
    ▼
kubevalet (Go + Vue 3, single binary)
    ├── PostgreSQL        — source of truth: users, groups, RBAC config, certs, keys
    └── Kubernetes API
            ├── CertificateSigningRequest   — x509 cert issuance and approval
            ├── Secret                      — private key storage per user
            ├── ClusterRole / ClusterRoleBinding
            └── Role / RoleBinding          — per-namespace bindings
```

All k8s objects created by kubevalet are named `kubevalet-<username>` (users) or `kubevalet-group-<name>` (groups) and labeled `app.kubernetes.io/managed-by=kubevalet`.

## Security

- Distroless runtime image (`gcr.io/distroless/static-debian12:nonroot`)
- `readOnlyRootFilesystem: true`, all capabilities dropped, `allowPrivilegeEscalation: false`
- `seccompProfile: RuntimeDefault` on all pods
- JWT secret auto-generated (32 chars) on first install
- Login rate limiting: 10 attempts per minute per IP
- HTTP security headers: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`
- Input validation: usernames and group names enforced as DNS labels

## Source

[github.com/TrueBad0ur/kubevalet](https://github.com/TrueBad0ur/kubevalet)
