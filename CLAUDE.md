# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

kubevalet automates Kubernetes user management: x509 certificate creation via the CSR API, RBAC bindings, and kubeconfig generation. Stack: Go (Gin) + Vue 3 + PostgreSQL + Helm.

---

## Commands

### Verify before every build
```bash
go build ./...
```
There is no local frontend build step — `vue-tsc` and `vite build` run inside Docker. TypeScript errors **will break the Docker build**, so read TS changes carefully before pushing.

### Build + push multi-arch image (amd64 + arm64)
```bash
make docker-buildx-push TAG=<version>
```

### Deploy to Kubernetes
```bash
KUBECONFIG=/home/kick/.kube/local-config helm upgrade kubevalet ./charts/kubevalet -n kubevalet --reuse-values --set image.tag=<version>
KUBECONFIG=/home/kick/.kube/local-config kubectl rollout status deployment/kubevalet -n kubevalet
```

### Run tests
```bash
go test ./...                        # all tests
go test ./internal/api/... -run TestName  # single test
```

### Other
```bash
make helm-lint                       # lint Helm chart
go mod tidy                          # tidy dependencies
```

---

## Iteration checklist (every code change)

1. Edit `internal/` (Go) and/or `web/src/` (Vue/TS) and/or `charts/kubevalet/`
2. `go build ./...` — must be clean
3. Bump `image.tag` in `charts/kubevalet/values.yaml` (current version is the value already there)
4. `make docker-buildx-push TAG=<new-tag>`
5. `KUBECONFIG=/home/kick/.kube/local-config helm upgrade kubevalet ./charts/kubevalet -n kubevalet --reuse-values --set image.tag=<new-tag>`
6. `KUBECONFIG=/home/kick/.kube/local-config kubectl rollout status deployment/kubevalet -n kubevalet`

---

## Architecture

### User state lives on the CSR, not in the database

There is no users table. Every managed Kubernetes user is represented by a `CertificateSigningRequest` object named `kubevalet-<username>`. All user metadata is stored as annotations on that CSR. `ListUsers` lists CSRs with label `app.kubernetes.io/managed-by=kubevalet` and reconstructs user structs from annotations.

Annotations (`kubevalet.io/*`):
| Annotation | Value |
|---|---|
| `groups` | comma-separated x509 Organization fields |
| `cluster-role` | built-in ClusterRole name (e.g. `edit`) |
| `custom-role` | `"true"` when a kubevalet-managed ClusterRole was created |
| `namespace-bindings` | JSON `[{namespace, role?, customRole?}]` — current format |
| `namespace`, `role` | legacy single-namespace format — read-only backward compat |

The PostgreSQL database is only used for admin user authentication (not for k8s users).

### All managed k8s objects share one naming convention

Every k8s object created for a user (CSR, Secret, ClusterRole, ClusterRoleBinding, RoleBinding, Role) is named `kubevalet-<username>` and carries labels:
```
app.kubernetes.io/managed-by=kubevalet
kubevalet.io/username=<username>
```
`DeleteAllNamespaceBindings` uses `kubevalet.io/username=<username>` as a label selector to find and delete RoleBindings/Roles across all namespaces.

### Groups are baked into the x509 certificate

x509 groups → cert `Organization` field. If groups change during RBAC edit (`PUT /users/:name/rbac`):
1. New keypair + CSR PEM generated (`internal/cert`)
2. Old CSR deleted, 300ms sleep
3. New CSR submitted + auto-approved + polled until cert issued (`internal/k8s/csr.go`)
4. New private key stored in Secret (`kubevalet-<username>` in kubevalet namespace)
5. New kubeconfig returned in `UpdateRBACResponse.Kubeconfig`

If groups are unchanged, only CSR annotations are updated (`UpdateCSRAnnotations` clears all `kubevalet.io/*` and rewrites from `newAnn`).

### UpdateCSRAnnotations rewrites all kubevalet annotations

When building `newAnn` in `UpdateUserRBAC`, every field that needs to persist must be explicitly added. `UpdateCSRAnnotations` wipes then sets — nothing is preserved implicitly.

### Frontend is embedded in the Go binary

`web/dist/` is embedded via `web/embed.go`. The Gin server serves the Vue SPA from `embed.FS`. The Docker build runs `npm install && npm run build` in stage 1, then copies `web/dist` into the Go build stage.

### ClusterRole for the service account (`charts/kubevalet/templates/clusterrole.yaml`)

Requires `bind` + `escalate` on both `clusterroles` and `roles` (needed to create custom role bindings), `update`/`patch` on CSRs (needed for `UpdateCSRAnnotations`), and `list` on `roles` at cluster scope (needed by `DeleteAllNamespaceBindings`). **Check this file whenever adding new k8s operations.**

---

## API routes

All under `/api/v1`. Public: `POST /auth/login`. Everything else requires `Authorization: Bearer <jwt>`.

```
GET    /users
POST   /users
PUT    /users/:name/rbac
DELETE /users/:name
GET    /users/:name/kubeconfig   ← returns YAML blob (responseType:'blob' on frontend)
```

---

## Known gotchas

- **kubeconfig**: Always use `KUBECONFIG=/home/kick/.kube/local-config`. The default context points to a decommissioned EKS cluster.
- **Helm namespace**: The release lives in the `kubevalet` namespace. `helm upgrade` without `-n kubevalet` silently operates on the wrong release.
- **kubeconfig endpoint returns a blob**: The axios client must use `{ responseType: 'blob' }`. Error responses on blob requests are also Blobs — call `.text()` then `JSON.parse()` to extract the error message.
- **Backward compat annotations**: Users created before multi-namespace support have `kubevalet.io/namespace` + `kubevalet.io/role` on their CSR. `ListUsers` reads both. After the first RBAC edit, the user is migrated to the JSON `namespace-bindings` format.
