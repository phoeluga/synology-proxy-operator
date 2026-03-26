# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY main.go main.go
COPY api/ api/
COPY internal/ internal/

# Build the binary with CGO disabled for a fully static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a \
    -ldflags="-s -w" \
    -o manager \
    .

# Runtime stage — distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
