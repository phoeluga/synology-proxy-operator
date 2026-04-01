# syntax=docker/dockerfile:1

# ─── Build stage ────────────────────────────────────────────────────────────
# Always build on the native runner platform (amd64).
# TARGETARCH is injected by buildx and tells Go which arch to cross-compile for.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETARCH

WORKDIR /workspace

# Cache module downloads before copying source.
COPY go.mod go.sum ./
RUN go mod download

COPY api/       api/
COPY cmd/       cmd/
COPY internal/  internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o manager ./cmd/main.go

# ─── Runtime stage ──────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
