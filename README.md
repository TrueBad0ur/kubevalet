# kubevalet

[![Image build](https://github.com/TrueBad0ur/kubevalet/actions/workflows/image-release.yml/badge.svg)](https://github.com/TrueBad0ur/kubevalet/actions/workflows/image-release.yml)
[![Chart release](https://github.com/TrueBad0ur/kubevalet/actions/workflows/chart-release.yml/badge.svg)](https://github.com/TrueBad0ur/kubevalet/actions/workflows/chart-release.yml)
[![Docker Image Version](https://img.shields.io/docker/v/truebad0ur/kubevalet?label=docker&sort=semver)](https://hub.docker.com/r/truebad0ur/kubevalet)
[![Docker Pulls](https://img.shields.io/docker/pulls/truebad0ur/kubevalet)](https://hub.docker.com/r/truebad0ur/kubevalet)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kubevalet)](https://artifacthub.io/packages/helm/kubevalet/kubevalet)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Lightweight Kubernetes user management with a web UI.

Creates x509 users via the Kubernetes CSR API, issues kubeconfigs, and manages RBAC bindings — no LDAP, no OIDC, no Dex.

## What it does

- Create Kubernetes users (x509 / CSR API)
- Assign preset roles (`cluster-admin`, `admin`, `edit`, `view`) or define custom RBAC rules (API groups, resources, verbs)
- Multi-namespace scoped bindings per user
- **Groups** — manage k8s Group subjects with their own RBAC; users added to a group via x509 O field inherit permissions automatically
- Download / view generated kubeconfigs
- **Graph view** — visualise any user's full access tree: cluster-wide role and per-namespace bindings
- **PostgreSQL as source of truth** — all user state (RBAC config, cert PEM, private key) stored in postgres; **Sync** button recreates any missing k8s objects from DB
- Kubeconfig API server address configurable at runtime from the Settings UI (no redeploy needed)
- Private keys stored in cluster Secrets and postgres — never logged or exposed raw
- Simple username/password auth backed by PostgreSQL

## Structure

```
cmd/server/        # entrypoint
internal/
  api/             # HTTP handlers (Gin)
  auth/            # JWT
  cert/            # x509 key + CSR generation
  config/          # env config
  db/              # PostgreSQL (pgx)
  k8s/             # CSR, RBAC, Secret helpers
  kubeconfig/      # kubeconfig builder
  models/          # shared types
web/               # Vue 3 frontend (embedded in binary)
charts/kubevalet/  # Helm chart (includes bundled PostgreSQL)
```

## Build

**Prerequisites:** Docker with buildx, a builder instance.

Set your own image repo in `Makefile`:
```makefile
IMAGE := your-dockerhub-user/kubevalet
```

Then build & push:
```bash
# One-time buildx setup
make buildx-setup

# Build & push multi-arch image (linux/amd64 + linux/arm64)
TAG=0.1.0 make docker-buildx-push
```

And point the Helm chart at your image:
```bash
helm install kubevalet ./charts/kubevalet \
  --set image.repository=your-dockerhub-user/kubevalet \
  --set image.tag=0.1.0 \
  ...
```

Single-arch local build:
```bash
make web-build   # build Vue frontend
make build       # compile Go binary → bin/kubevalet
```

## Install (Helm)

```bash
helm install kubevalet ./charts/kubevalet \
  --namespace kubevalet --create-namespace \
  --set cluster.server=https://<your-api-server>:6443 \
  --set auth.adminPassword=changeme
```

Upgrade:
```bash
helm upgrade kubevalet ./charts/kubevalet --namespace kubevalet
```

Access UI:
```bash
kubectl port-forward svc/kubevalet 8080:80 -n kubevalet
# http://localhost:8080
```

## Key values to change

| Value | Default | Description |
|---|---|---|
| `image.tag` | `0.3.6` | Image tag |
| `cluster.server` | `https://kubernetes.default.svc.cluster.local` | API server URL embedded in kubeconfigs — set to the external address users will connect to (can also be changed at runtime in Settings UI) |
| `cluster.name` | `kubernetes` | Cluster name in kubeconfig context |
| `auth.adminPassword` | `admin` | Initial admin password |
| `auth.jwtSecret` | _(auto-generated)_ | JWT signing secret — auto-generated on first install, preserved across upgrades |
| `postgres.persistence.enabled` | `false` | Enable PVC for PostgreSQL (requires a StorageClass) |
| `ingress.enabled` | `false` | Expose via Ingress |
| `ingress.host` | `kubevalet.example.com` | Ingress hostname |

## Local run (without Helm)

The binary is configured via environment variables. For production use Helm values instead — they map to the same vars automatically.

```bash
export POSTGRES_DSN=postgres://kubevalet:pass@localhost:5432/kubevalet
export JWT_SECRET=changeme
export CLUSTER_SERVER=https://your-api:6443   # URL that goes into kubeconfigs
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD=changeme
# optional:
export KUBECONFIG=/path/to/kubeconfig         # defaults to in-cluster service account
export NAMESPACE=kubevalet                    # namespace where key Secrets are stored
export TOKEN_TTL=24h

make build
./bin/kubevalet
```
