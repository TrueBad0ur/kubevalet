BINARY    := kubevalet
CMD       := ./cmd/server
IMAGE     := truebad0ur/kubevalet
TAG       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PLATFORMS := linux/amd64,linux/arm64

.PHONY: run build tidy test lint \
        web-build \
        docker-build docker-push docker-buildx docker-buildx-push \
        buildx-setup \
        helm-lint helm-package

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
	  -t $(IMAGE):$(TAG) \
	  -t $(IMAGE):latest \
	  --push \
	  .

## ── Helm ─────────────────────────────────────────────────────────────────────

helm-lint:
	helm lint charts/kubevalet

helm-package:
	helm package charts/kubevalet -d dist/
