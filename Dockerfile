# syntax=docker/dockerfile:1

# ── Stage 1: Build frontend (always on build host, no cross-compile needed) ──
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# ── Stage 2: Build Go binary (cross-compile on build host) ───────────────────
FROM --platform=$BUILDPLATFORM golang:1.26.2-alpine AS builder
# Injected by `docker buildx build --platform`
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Overwrite dist placeholder with real built frontend
COPY --from=frontend /app/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-w -s" -o /kubevalet ./cmd/server

# ── Stage 3: Minimal runtime (distroless is multi-arch) ─────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /kubevalet /kubevalet
EXPOSE 8080
ENTRYPOINT ["/kubevalet"]
