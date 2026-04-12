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
        commit release chart-release

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

# Usage: make commit MSG="your commit message"
commit:
ifndef MSG
	$(error MSG is required, e.g.: make commit MSG="fix login button")
endif
	git add .
	git diff --cached --quiet || git commit -m "$(MSG)"
	git push

# Usage: make release MSG="your message" VER=0.3.2
# Commits, pushes, then creates both tags:
#   v<VER>       → GitHub Actions builds Docker image → pushes to DockerHub
#   chart-v<VER> → GitHub Actions packages Helm chart → pushes to ghcr.io → Artifact Hub
release:
ifndef MSG
	$(error MSG is required, e.g.: make release MSG="release 0.3.3" VER=0.3.3)
endif
ifndef VER
	$(error VER is required, e.g.: make release MSG="release 0.3.3" VER=0.3.3)
endif
	git add .
	git diff --cached --quiet || git commit -m "$(MSG)"
	git push
	git tag v$(VER)
	git push origin v$(VER)
	git tag chart-v$(VER)
	git push origin chart-v$(VER)

# Usage: make chart-release MSG="your commit message" CHART_TAG=0.3.2
# Commits, pushes, then tags the chart release only (triggers GitHub Actions → ghcr.io → Artifact Hub)
chart-release:
ifndef MSG
	$(error MSG is required, e.g.: make chart-release MSG="release chart 0.3.2" CHART_TAG=0.3.2)
endif
ifndef CHART_TAG
	$(error CHART_TAG is required, e.g.: make chart-release MSG="release chart 0.3.2" CHART_TAG=0.3.2)
endif
	git add .
	git diff --cached --quiet || git commit -m "$(MSG)" && git push
	git tag chart-v$(CHART_TAG)
	git push origin chart-v$(CHART_TAG)

## ── Helm ─────────────────────────────────────────────────────────────────────

helm-lint:
	helm lint charts/kubevalet

helm-package:
	helm package charts/kubevalet -d dist/
