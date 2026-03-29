# syntax=docker/dockerfile:1

# ─── Build stage ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Declare the Docker build arg so buildx injects the target architecture.
# This enables native Go cross-compilation instead of QEMU emulation,
# making the arm64 build as fast as the amd64 build.
ARG TARGETARCH=amd64

WORKDIR /workspace

# Cache module downloads before copying source.
COPY go.mod go.sum ./
RUN go mod download

COPY api/       api/
COPY cmd/       cmd/
COPY internal/  internal/

# Cross-compile for the target platform. CGO is disabled so no C toolchain
# is needed for arm64, and the binary is fully static.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o manager ./cmd/main.go

# ─── Runtime stage ──────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
