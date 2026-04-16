BINARY    := kubevalet
CMD       := ./cmd/server
IMAGE     := truebad0ur/kubevalet
TAG       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PLATFORMS := linux/amd64,linux/arm64

.PHONY: run build tidy test lint \
        web-build \
        docker-build docker-push docker-buildx docker-buildx-push \
        buildx-setup \
        helm-lint helm-package \
        hooks-setup commit release

## ── Local dev ────────────────────────────────────────────────────────────────

run:
	go run $(CMD)

build:
	go build -o bin/$(BINARY) $(CMD)

tidy:
	go mod tidy

test:
	go test ./...

lint:
	golangci-lint run ./...

## ── Frontend ─────────────────────────────────────────────────────────────────

web-build:
	cd web && npm install && npm run build

## ── Docker (single-arch, local) ──────────────────────────────────────────────

# Quick local build — current machine arch only, loads into local Docker daemon
docker-build: web-build
	docker build -t $(IMAGE):$(TAG) .

docker-push:
	docker push $(IMAGE):$(TAG)

## ── Docker buildx (multi-arch: amd64 + arm64) ───────────────────────────────

# One-time setup: create a multi-arch capable builder
buildx-setup:
	docker buildx inspect kubevalet-builder >/dev/null 2>&1 || \
	  docker buildx create --name kubevalet-builder --driver docker-container --bootstrap
	docker buildx use kubevalet-builder

# Build multi-arch and load into local daemon (single platform only, for testing)
docker-buildx:
	docker buildx build \
	  --builder kubevalet-builder \
	  --platform $(PLATFORMS) \
	  -t $(IMAGE):$(TAG) \
	  .

# Build multi-arch and push to registry
docker-buildx-push:
	docker buildx build \
	  --builder kubevalet-builder \
	  --platform $(PLATFORMS) \
	  --build-arg VERSION=$(TAG) \
	  -t $(IMAGE):$(TAG) \
	  -t $(IMAGE):latest \
	  --push \
	  .

## ── Git ──────────────────────────────────────────────────────────────────────

# Run once after cloning to activate git hooks
hooks-setup:
	git config core.hooksPath .githooks
	@echo "Git hooks activated (.githooks/pre-push)"

# Usage: make commit MSG="your commit message"
commit:
ifndef MSG
	$(error MSG is required, e.g.: make commit MSG="fix login button")
endif
	git config core.hooksPath .githooks
	git add .
	git diff --cached --quiet || git commit -m "$(MSG)"
	git push --set-upstream origin $$(git branch --show-current)

# Usage: make release VER=0.3.13
# Tags HEAD of main as v<VER> and pushes the tag.
# GitHub Actions picks up the tag and builds:
#   - Docker image (linux/amd64 + arm64) → DockerHub
#   - Helm chart → ghcr.io → Artifact Hub
# Version is injected by CI from the tag — no file changes needed.
release:
ifndef VER
	$(error VER is required, e.g.: make release VER=0.3.13)
endif
	git config core.hooksPath .githooks
	git checkout main
	git pull
	git tag -d v$(VER) 2>/dev/null || true
	git push origin :refs/tags/v$(VER) 2>/dev/null || true
	git tag v$(VER)
	git push origin v$(VER)

## ── Helm ─────────────────────────────────────────────────────────────────────

helm-lint:
	helm lint charts/kubevalet

helm-package:
	helm package charts/kubevalet -d dist/
