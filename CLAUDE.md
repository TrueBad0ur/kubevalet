# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

kubevalet automates Kubernetes user management: x509 certificate creation via the CSR API, RBAC bindings, and kubeconfig generation. Stack: Go (Gin) + Vue 3 + PostgreSQL + Helm.

Users can belong to groups (x509 O field) or have no direct RBAC (groups-only). PostgreSQL is the source of truth for all user state (cert_pem, private_key_pem, RBAC config). The Sync button recreates any missing k8s objects from the DB.

---

## MANDATORY rules (never skip)

1. **After every change — bump versions** in all three places to the same value:
   - `image.tag` in `charts/kubevalet/values.yaml`
   - `version` + `appVersion` in `charts/kubevalet/Chart.yaml`
   - `image.tag` row in `README.md` key values table
   - version references in `charts/kubevalet/README.md`

2. **After every session — print the release command:**
   ```bash
   make release MSG="<describe changes>" VER=<new-version>
   ```

---

## Commands

### Verify before every build
```bash
go build ./...
```
There is no local frontend build step — `vue-tsc` and `vite build` run inside Docker. TypeScript errors **will break the Docker build**, so read TS changes carefully before pushing.

### Commit and push (no release)
```bash
make commit MSG="your message"
```

### Full release — image + chart (CI does all building)
```bash
make release MSG="release 0.3.10" VER=0.3.10
```
Creates one tag `v<VER>` → GitHub Actions (`release.yml`) runs two parallel jobs:
- `docker` → multi-arch Docker image → DockerHub
- `chart` → Helm chart → ghcr.io → Artifact Hub

Nothing is built locally. After CI finishes, deploy manually:
```bash
KUBECONFIG=/home/kick/.kube/local-config helm upgrade kubevalet ./charts/kubevalet -n kubevalet --reuse-values --set image.tag=<version>
KUBECONFIG=/home/kick/.kube/local-config kubectl rollout status deployment/kubevalet -n kubevalet
```

### Run tests
```bash
go test ./...                             # all tests
go test ./internal/api/... -run TestName  # single test
```

### Other
```bash
make helm-lint    # lint Helm chart
go mod tidy       # tidy dependencies
```

---

## Iteration checklist (every code change)

1. Edit `internal/` (Go) and/or `web/src/` (Vue/TS) and/or `charts/kubevalet/`
2. `go build ./...` — must be clean
3. Bump version everywhere to the same value: `image.tag` in `values.yaml`, and both `version` + `appVersion` in `Chart.yaml` — always all three together

**If releasing a new version (building happens entirely in CI):**
```
make release MSG="<describe changes>" VER=<new-version>
```
Then wait for GitHub Actions to finish, and deploy:
```
KUBECONFIG=/home/kick/.kube/local-config helm upgrade kubevalet ./charts/kubevalet -n kubevalet --reuse-values --set image.tag=<new-version>
KUBECONFIG=/home/kick/.kube/local-config kubectl rollout status deployment/kubevalet -n kubevalet
```

**If just committing without a release:**
```
make commit MSG="<describe changes>"
```

After every session always print the appropriate command above.

---

## Architecture

### PostgreSQL is the source of truth

All user state is stored in the `users` table: `cert_pem`, `private_key_pem`, groups, RBAC config. The `Sync` endpoint (`POST /users/:name/sync`) recreates any missing k8s objects (Secret, RBAC) from DB.

On `ListUsers`, any CSR-annotated user not yet in the DB is auto-migrated via `importCSRUser()` (reads cert from CSR status, key from k8s Secret, inserts into DB).

Tables:
- `admin_users` — admin authentication
- `users` — k8s user state (cert, key, RBAC)
- `groups` — k8s Group subjects with RBAC
- `app_settings` — runtime config (e.g. `cluster_server`)

### User RBAC is optional — groups-only users are allowed

A user may have no direct RBAC (`clusterRole`, `rules`, `namespaceBindings` all empty) and belong only to groups. The backend no longer requires RBAC on user creation.

### CSR annotations (legacy + current)

Every managed CSR carries `kubevalet.io/*` annotations as a secondary record. New users are DB-first; annotations are kept for compatibility and CSR status (Active/Pending/Denied).

Annotations:
| Annotation | Value |
|---|---|
| `groups` | comma-separated x509 Organization fields |
| `cluster-role` | built-in ClusterRole name (e.g. `edit`) |
| `custom-role` | `"true"` when a kubevalet-managed ClusterRole was created |
| `namespace-bindings` | JSON `[{namespace, role?, customRole?}]` — current format |
| `namespace`, `role` | legacy single-namespace format — read-only backward compat |

### All managed k8s objects share one naming convention

Users: named `kubevalet-<username>`, label `kubevalet.io/username=<username>`
Groups: named `kubevalet-group-<name>`, label `kubevalet.io/group=<name>`

Every object also carries `app.kubernetes.io/managed-by=kubevalet`.

### Groups are baked into the x509 certificate

x509 groups → cert `Organization` field. If groups change during RBAC edit (`PUT /users/:name/rbac`):
1. New keypair + CSR PEM generated (`internal/cert`)
2. Old CSR deleted, 300ms sleep
3. New CSR submitted + auto-approved + polled until cert issued (`internal/k8s/csr.go`)
4. New private key stored in Secret (`kubevalet-<username>` in kubevalet namespace) and in DB
5. New kubeconfig returned in `UpdateRBACResponse.Kubeconfig`

If groups are unchanged, only CSR annotations are updated (`UpdateCSRAnnotations` clears all `kubevalet.io/*` and rewrites from `newAnn`).

### UpdateCSRAnnotations rewrites all kubevalet annotations

When building `newAnn` in `UpdateUserRBAC`, every field that needs to persist must be explicitly added. `UpdateCSRAnnotations` wipes then sets — nothing is preserved implicitly.

### Frontend is embedded in the Go binary

`web/dist/` is embedded via `web/embed.go`. The Gin server serves the Vue SPA from `embed.FS`. The Docker build runs `npm install && npm run build` in stage 1, then copies `web/dist` into the Go build stage.

### ClusterRole for the service account (`charts/kubevalet/templates/clusterrole.yaml`)

Requires `bind` + `escalate` on both `clusterroles` and `roles` (needed to create custom role bindings), `update`/`patch` on CSRs (needed for `UpdateCSRAnnotations`), and `list` on `roles` at cluster scope (needed by `DeleteAllNamespaceBindings`). **Check this file whenever adding new k8s operations.**

### Security hardening in place

- JWT_SECRET validated non-empty on startup (fatal if missing)
- HTTP security headers on all responses: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`
- Login rate limiting: 10 attempts/minute per IP (in-memory, `internal/api/auth.go`)
- Input validation: username and group names must match DNS-label format
- pgx-native unique violation detection (`pgconn.PgError` code `23505`)
- DB connection pool: MaxConns=20, MaxConnLifetime=30m, HealthCheckPeriod=1m
- Helm: `readOnlyRootFilesystem: true` on all containers, init container resource limits
- Helm: postgres password auto-generated on first install (stored in Secret, preserved across upgrades)
- Helm: NetworkPolicy template (disabled by default, enable with `networkPolicy.enabled=true`)

---

## API routes

All under `/api/v1`. Public: `POST /auth/login`. Everything else requires `Authorization: Bearer <jwt>`.

```
GET    /users
POST   /users
PUT    /users/:name/rbac
DELETE /users/:name
GET    /users/:name/kubeconfig     ← returns YAML blob (responseType:'blob' on frontend)
POST   /users/:name/sync

GET    /groups
POST   /groups
PUT    /groups/:name
DELETE /groups/:name
POST   /groups/:name/sync

GET    /settings
PUT    /settings
PUT    /settings/password
GET    /auth/me
```

---

## Helm chart + Artifact Hub

Chart is published to `ghcr.io/truebad0ur/kubevalet` (OCI) and indexed by Artifact Hub.

To release a new chart version:
1. Bump `version` and `appVersion` in `charts/kubevalet/Chart.yaml`
2. `make chart-release MSG="..." CHART_TAG=<version>`

GitHub Actions (`.github/workflows/chart-release.yml`) triggers on `v*-chart` tags, packages the chart and pushes to ghcr.io. Artifact Hub polls automatically.

---

## Known gotchas

- **kubeconfig**: Always use `KUBECONFIG=/home/kick/.kube/local-config`. The default context points to a decommissioned EKS cluster.
- **Helm namespace**: The release lives in the `kubevalet` namespace. `helm upgrade` without `-n kubevalet` silently operates on the wrong release.
- **kubeconfig endpoint returns a blob**: The axios client must use `{ responseType: 'blob' }`. Error responses on blob requests are also Blobs — call `.text()` then `JSON.parse()` to extract the error message.
- **Backward compat annotations**: Users created before multi-namespace support have `kubevalet.io/namespace` + `kubevalet.io/role` on their CSR. `ListUsers` reads both. After the first RBAC edit, the user is migrated to the JSON `namespace-bindings` format.
- **postgres password**: auto-generated on first install. If you need to reset it, delete the Secret and reinstall. Changing `postgres.password` in values after first install has no effect (Secret lookup preserves existing value).
- **`.gitignore` rule**: `kubevalet` entry uses `/kubevalet` (root-anchored) to avoid ignoring `charts/kubevalet/`. Do not remove the leading slash.
- **groups-only users**: users with no RBAC (only groups) are valid. The `Access` column shows `—`, kubeconfig still works via group bindings.
