# syntax=docker/dockerfile:1

# ─── Build stage ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Cache module downloads before copying source.
COPY go.mod go.sum ./
RUN go mod download

COPY api/       api/
COPY cmd/       cmd/
COPY internal/  internal/

# Build with CGO disabled for a fully static binary.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w" -o manager ./cmd/main.go

# ─── Runtime stage ──────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
