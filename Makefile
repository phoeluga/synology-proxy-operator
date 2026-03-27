.PHONY: help test build docker-build fmt lint coverage clean run deploy undeploy

# Default target
.DEFAULT_GOAL := help

# Variables
BINARY_NAME=manager
DOCKER_IMAGE=synology-proxy-operator
DOCKER_TAG=latest
NAMESPACE=synology-proxy-operator

# Container runtime (docker or podman)
CONTAINER_RUNTIME ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)

## help: Display this help message
help:
	@echo "Synology Proxy Operator - Makefile targets:"
	@echo ""
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/##//'

## test: Run unit tests
test:
	go test -v -race ./...

## test-coverage: Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## build: Build the operator binary
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/$(BINARY_NAME) main.go
	@echo "Binary built: bin/$(BINARY_NAME)"

## docker-build: Build Docker image (supports Docker and Podman)
docker-build:
	$(CONTAINER_RUNTIME) build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Container image built with $(CONTAINER_RUNTIME): $(DOCKER_IMAGE):$(DOCKER_TAG)"

## docker-push: Push Docker image to registry (supports Docker and Podman)
docker-push:
	$(CONTAINER_RUNTIME) push $(DOCKER_IMAGE):$(DOCKER_TAG)

## fmt: Format Go code
fmt:
	gofmt -w .
	@echo "Code formatted"

## lint: Run linter
lint:
	golangci-lint run
	@echo "Linting complete"

## clean: Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Build artifacts cleaned"

## run: Run the operator locally
run:
	go run main.go \
		--synology-url=https://nas.example.com:5001 \
		--synology-secret-name=synology-credentials \
		--synology-secret-namespace=$(NAMESPACE) \
		--watch-namespaces="*" \
		--log-level=debug

## deploy: Deploy operator to Kubernetes
deploy:
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f config/rbac/
	kubectl apply -f config/manager/
	@echo "Operator deployed to namespace: $(NAMESPACE)"

## undeploy: Remove operator from Kubernetes
undeploy:
	kubectl delete -f config/manager/ --ignore-not-found=true
	kubectl delete -f config/rbac/ --ignore-not-found=true
	@echo "Operator removed from namespace: $(NAMESPACE)"

## logs: View operator logs
logs:
	kubectl logs -f -n $(NAMESPACE) -l app=$(DOCKER_IMAGE)

## install-deps: Install development dependencies
install-deps:
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Dependencies installed"

## verify: Run all verification checks
verify: fmt lint test
	@echo "All verification checks passed"
