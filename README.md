# kubevalet

<p align="center">
  <img src="assets/kubevalet-icon.svg" width="80" alt="kubevalet"/>
</p>

[![Release](https://github.com/TrueBad0ur/kubevalet/actions/workflows/release.yml/badge.svg)](https://github.com/TrueBad0ur/kubevalet/actions/workflows/release.yml)
[![Docker Image Version](https://img.shields.io/docker/v/truebad0ur/kubevalet?label=docker&sort=date)](https://hub.docker.com/r/truebad0ur/kubevalet)
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

## Screenshots

| Users | Groups |
|-------|--------|
| ![Users](assets/screenshot1.png) | ![Groups](assets/screenshot2.png) |

| Graph — all users | Graph — group filter |
|-------------------|----------------------|
| ![Graph all](assets/screenshot3.png) | ![Graph group](assets/screenshot4.png) |

![Settings](assets/screenshot5.png)

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

## Development flow

### Branch naming convention

Feature branches must follow the format `x.x.x-suffix`, e.g.:

```
0.3.13-dev
0.3.13-feature-oidc
0.3.13-fix-login
```

This is enforced by a pre-push git hook. Activate it once after cloning:

```bash
make hooks-setup
```

After that it activates automatically on every `make commit` and `make release` — no need to remember.

### Work in progress — feature branch

```bash
git checkout -b 0.3.13-dev

# iterate freely — no version bumps, no file changes needed
make commit MSG="add feature X"
make commit MSG="fix bug Y"
```

Every push to a branch matching `x.x.x-*` builds a Docker image tagged with the branch name:

```
truebad0ur/kubevalet:0.3.13-dev
truebad0ur/kubevalet:0.3.13-feature-oidc
```

Deploy a branch image to test before releasing:

```bash
KUBECONFIG=~/.kube/local-config helm upgrade kubevalet ./charts/kubevalet \
  -n kubevalet --reuse-values --set image.tag=0.3.13-dev
```

### Reviewing PRs

#### Your own PRs

1. Open a PR — **`pr-validate`** runs automatically on every push: go build, tests, helm lint, docker build.
2. When all checks pass, `pr-validate` automatically adds the `ok-to-test` label.
3. **`pr-image`** triggers and pushes a test image tagged `<next-version>-<branch-name>` to DockerHub (e.g. `0.3.19-my-feature`).
4. On the next push the label is re-cycled automatically — no manual action needed.
5. When satisfied — squash merge.

> **Requires:** a `PAT_TOKEN` secret in repo Settings → Secrets → Actions (classic PAT with `repo` scope). This is needed because GitHub does not fire webhook events for labels added by the built-in `GITHUB_TOKEN`.

#### External contributor PRs

1. **`pr-validate`** runs automatically — go build, tests, helm lint, docker build (no push). No secrets, safe for forks.
2. Review the code. If it looks good, add the label **`ok-to-test`** manually.
3. **`pr-image`** triggers and pushes the test image to DockerHub.
4. If the contributor pushes new commits, `ok-to-test` is removed automatically — re-review and re-label to build again.
5. When satisfied — squash merge.

### Release — when ready to ship

Version lives **only in the git tag** — no version bumping in files.
CI reads the tag and injects it into the Docker image and Helm chart at build time.

1. Open a PR → squash merge to `main`.

2. On `main` after merge — tag and trigger CI:
```bash
make release VER=0.3.13   # patch
make release VER=0.4.0    # minor — new feature set, no breaking changes
make release VER=1.0.0    # major — breaking changes
```
CI always injects whatever version you pass. The decision of patch/minor/major is yours — it has no effect on the build process itself.

GitHub Actions builds in parallel:
- Docker image `truebad0ur/kubevalet:0.3.13` + `latest` → DockerHub
- Helm chart `0.3.13` → ghcr.io → Artifact Hub

3. Deploy:
```bash
KUBECONFIG=~/.kube/local-config helm upgrade kubevalet ./charts/kubevalet \
  -n kubevalet --reuse-values --set image.tag=0.3.13
KUBECONFIG=~/.kube/local-config kubectl rollout status deployment/kubevalet -n kubevalet
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
| `image.tag` | `0.3.16` | Image tag |
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

## Custom RBAC rules — API groups

Each rule covers exactly one API group. Resources from different groups must be split into separate rules:

| Resource | API Group |
|---|---|
| `pods`, `secrets`, `configmaps`, `services` | `` (empty = core) |
| `deployments`, `replicasets`, `statefulsets`, `daemonsets` | `apps` |
| `cronjobs`, `jobs` | `batch` |
| `ingresses` | `networking.k8s.io` |
| `clusterroles`, `roles`, `rolebindings` | `rbac.authorization.k8s.io` |

**Wrong** — `pods` won't work because it's not in the `apps` group:
```
API Groups: apps    Resources: pods, deployments
```

**Correct** — two separate rules:
```
Rule 1 — API Groups: (empty)   Resources: pods
Rule 2 — API Groups: apps      Resources: deployments
```

## Known limitations

- **JWT role changes take effect only after token expiry.** When an admin demotes another admin to viewer (or changes any role), the existing JWT is not invalidated — the affected user retains their previous role until their token expires (default TTL: 24 h). To force immediate effect, the user must log out and log in again. This is an inherent trade-off of stateless JWT auth; adding server-side token blacklisting would require a shared revocation store.

## Roadmap

- [x] Screenshots in README
- [x] RBAC for local users (is_admin flag → admin can manage all users/passwords, regular users can only change own password)
- [ ] Keycloak / OIDC integration
- [ ] Multi-cluster support
- [ ] Audit log (who created/deleted which user and when)
- [x] User expiry / certificate rotation reminders
- [ ] Role templates (save and reuse custom RBAC configs)
- [ ] LDAP sync
- [ ] CVE scanning in CI (Trivy)
- [x] CI workflow for external PRs: `pull_request` (go build + test + helm lint + docker build --no-push, no secrets) and `pull_request_target` gated by `ok-to-test` label (build + push `pr-{N}` image to DockerHub, auto-remove label on new commits)
- [ ] Write tests for all
