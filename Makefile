## ─── Configuration ──────────────────────────────────────────────────────────

# Image repository & tag (override on CLI: make docker-build IMG=myrepo/myimage:tag)
IMG            ?= synology-proxy-operator:latest
HELM_RELEASE   ?= synology-proxy-operator
HELM_NAMESPACE ?= synology-proxy-operator

# Container tool: docker or podman
CONTAINER_TOOL ?= docker

# Local cluster tool: minikube or kind
CLUSTER_TOOL ?= minikube

# Minikube driver: docker (macOS default), qemu2, virtualbox, ...
# Avoid the podman driver on macOS — its VM lacks cpuset cgroups.
MINIKUBE_DRIVER ?= docker

# Minikube profile (allows multiple clusters)
MINIKUBE_PROFILE ?= synology-dev

# controller-gen & envtest versions
CONTROLLER_GEN_VERSION ?= v0.17.3
# Version of the setup-envtest tool itself (semver tag from controller-runtime)
ENVTEST_TOOL_VERSION   ?= latest
# Kubernetes version for envtest binaries (passed to setup-envtest use)
ENVTEST_K8S_VERSION    ?= 1.32.x

# Local kubeconfig
KUBECONFIG ?= $(HOME)/.kube/config

## ─── Help ───────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-26s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

## ─── Development ─────────────────────────────────────────────────────────────

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint to be installed)
	golangci-lint run ./...

.PHONY: test
test: fmt vet envtest ## Run unit tests
	KUBEBUILDER_ASSETS="$(shell $(LOCALBIN)/setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN)/envtest -p path)" \
		go test ./... -coverprofile cover.out -v

.PHONY: build
build: fmt vet ## Build manager binary locally
	go build -o bin/manager ./cmd/main.go

.PHONY: run
run: fmt vet ## Run the operator locally against the current kubeconfig cluster
	go run ./cmd/main.go \
		--synology-url="$(SYNOLOGY_URL)" \
		--synology-user="$(SYNOLOGY_USER)" \
		--synology-password="$(SYNOLOGY_PASSWORD)" \
		--synology-skip-tls-verify="$(SYNOLOGY_SKIP_TLS_VERIFY)" \
		--default-domain="$(DEFAULT_DOMAIN)" \
		--default-acl-profile="$(DEFAULT_ACL_PROFILE)" \
		--rule-namespace="$(HELM_NAMESPACE)"

## ─── Code generation ─────────────────────────────────────────────────────────

.PHONY: generate
generate: controller-gen ## Re-run controller-gen (deepcopy, CRD manifests)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests
	$(CONTROLLER_GEN) rbac:roleName=synology-proxy-operator-role \
		crd paths="./api/..." \
		output:crd:artifacts:config=config/crd/bases \
		output:rbac:artifacts:config=config/rbac

## ─── Container image ─────────────────────────────────────────────────────────

.PHONY: image-build
image-build: ## Build container image (CONTAINER_TOOL=docker|podman)
	$(CONTAINER_TOOL) build -t $(IMG) .

.PHONY: image-push
image-push: ## Push container image
	$(CONTAINER_TOOL) push $(IMG)

.PHONY: image-buildx
image-buildx: ## Build and push multi-arch image via buildx (docker only)
	$(CONTAINER_TOOL) buildx build --platform linux/amd64,linux/arm64 -t $(IMG) --push .

# Convenience aliases that keep old targets working.
.PHONY: docker-build docker-push docker-buildx
docker-build: image-build
docker-push:  image-push
docker-buildx: image-buildx

## ─── Cluster image loading ───────────────────────────────────────────────────

.PHONY: image-load
image-load: image-build ## Build image and load it into the local cluster (minikube or kind)
ifeq ($(CLUSTER_TOOL),minikube)
	minikube image load $(IMG) --profile $(MINIKUBE_PROFILE)
else ifeq ($(CLUSTER_TOOL),kind)
	kind load docker-image $(IMG)
else
	@echo "Unknown CLUSTER_TOOL=$(CLUSTER_TOOL). Use minikube or kind."
	@exit 1
endif

## ─── Minikube dev cluster ────────────────────────────────────────────────────

.PHONY: minikube-start
minikube-start: ## Start the dev minikube cluster
	minikube start \
		--profile $(MINIKUBE_PROFILE) \
		--driver $(MINIKUBE_DRIVER) \
		--cpus 4 \
		--memory 6g \
		--wait=all \
		--addons ingress

.PHONY: minikube-stop
minikube-stop: ## Stop the dev minikube cluster
	minikube stop --profile $(MINIKUBE_PROFILE)

.PHONY: minikube-delete
minikube-delete: ## Delete the dev minikube cluster
	minikube delete --profile $(MINIKUBE_PROFILE)

.PHONY: minikube-context
minikube-context: ## Switch kubectl context to the dev minikube cluster
	minikube update-context --profile $(MINIKUBE_PROFILE)

.PHONY: minikube-tunnel
minikube-tunnel: ## Start minikube tunnel to expose LoadBalancer services (run in separate terminal)
	minikube tunnel --profile $(MINIKUBE_PROFILE)

## ─── Local dev environment setup ─────────────────────────────────────────────

.PHONY: dev-argocd
dev-argocd: ## Install ArgoCD into the dev cluster
	kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
	@echo ""
	@echo "Waiting for ArgoCD to become ready..."
	kubectl wait --for=condition=available --timeout=120s deployment/argocd-server -n argocd
	@echo ""
	@echo "ArgoCD admin password:"
	@kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
	@echo ""

.PHONY: dev-install
dev-install: manifests ## Install CRDs and RBAC into current cluster (no operator deployment)
	kubectl create namespace $(HELM_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f config/crd/bases/
	kubectl apply -f config/rbac/

.PHONY: dev-test-app
dev-test-app: ## Deploy test nginx app and ArgoCD Application into dev cluster
	kubectl apply -f hack/dev/

.PHONY: dev-setup
dev-setup: dev-install dev-argocd dev-test-app ## Full dev environment setup (CRDs + ArgoCD + test app)
	@echo ""
	@echo "Dev environment ready."
	@echo "Next: copy .env.local.example to .env.local, fill in your Synology credentials,"
	@echo "      then run:  make dev-run   (or use VSCode debugger)"

.PHONY: dev-run
dev-run: fmt vet ## Run the operator locally in dev mode (reads .env.local if present)
	@if [ -f .env.local ]; then \
		export $$(grep -v '^#' .env.local | xargs); \
	fi; \
	go run ./cmd/main.go \
		--synology-url="$${SYNOLOGY_URL}" \
		--synology-user="$${SYNOLOGY_USER}" \
		--synology-password="$${SYNOLOGY_PASSWORD}" \
		--synology-skip-tls-verify="$${SYNOLOGY_SKIP_TLS_VERIFY:-false}" \
		--default-domain="$${DEFAULT_DOMAIN:-}" \
		--default-acl-profile="$${DEFAULT_ACL_PROFILE:-}" \
		--rule-namespace="$${RULE_NAMESPACE:-synology-proxy-operator}" \
		--enable-argo-watcher="$${ENABLE_ARGO_WATCHER:-true}" \
		--zap-devel=true

.PHONY: dev-clean
dev-clean: ## Remove test resources from dev cluster
	kubectl delete -f hack/dev/ --ignore-not-found=true
	kubectl delete -f config/rbac/ --ignore-not-found=true
	kubectl delete -f config/crd/bases/ --ignore-not-found=true

## ─── Kubernetes deployment ───────────────────────────────────────────────────

.PHONY: install
install: manifests ## Install CRDs into the current cluster
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Remove CRDs from the current cluster
	kubectl delete -f config/crd/bases/ --ignore-not-found=true

.PHONY: deploy
deploy: manifests ## Deploy operator to cluster using kubectl (for quick testing)
	kubectl apply -f config/crd/bases/
	kubectl apply -f config/rbac/
	kubectl apply -f config/manager/

.PHONY: undeploy
undeploy: ## Remove operator from cluster
	kubectl delete -f config/manager/ --ignore-not-found=true
	kubectl delete -f config/rbac/ --ignore-not-found=true

## ─── Helm ────────────────────────────────────────────────────────────────────

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart
	helm lint helm/synology-proxy-operator

.PHONY: helm-template
helm-template: ## Render the Helm chart to stdout (dry-run)
	helm template $(HELM_RELEASE) helm/synology-proxy-operator \
		--namespace $(HELM_NAMESPACE) \
		--set synology.url="$(SYNOLOGY_URL)" \
		--set synology.username="$(SYNOLOGY_USER)" \
		--set synology.password="$(SYNOLOGY_PASSWORD)"

.PHONY: helm-install
helm-install: ## Install or upgrade the Helm release
	helm upgrade --install $(HELM_RELEASE) helm/synology-proxy-operator \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--set synology.url="$(SYNOLOGY_URL)" \
		--set synology.username="$(SYNOLOGY_USER)" \
		--set synology.password="$(SYNOLOGY_PASSWORD)" \
		--set synology.skipTLSVerify="$(SYNOLOGY_SKIP_TLS_VERIFY)" \
		--set operator.defaultDomain="$(DEFAULT_DOMAIN)" \
		--set operator.defaultACLProfile="$(DEFAULT_ACL_PROFILE)"

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall the Helm release
	helm uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-package
helm-package: ## Package the Helm chart into a .tgz
	helm package helm/synology-proxy-operator --destination dist/

## ─── Local tooling ───────────────────────────────────────────────────────────

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

.PHONY: envtest
envtest: ## Download setup-envtest locally
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_TOOL_VERSION)
