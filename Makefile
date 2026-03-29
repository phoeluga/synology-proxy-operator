## ─── Configuration ──────────────────────────────────────────────────────────

# Image repository & tag (override on CLI: make docker-build IMG=myrepo/myimage:tag)
IMG            ?= synology-proxy-operator:latest
HELM_RELEASE   ?= synology-proxy-operator
HELM_NAMESPACE ?= synology-proxy-operator

# controller-gen & envtest versions
CONTROLLER_GEN_VERSION ?= v0.16.1
ENVTEST_VERSION        ?= release-0.19

# Local kubeconfig
KUBECONFIG ?= $(HOME)/.kube/config

## ─── Help ───────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

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
	KUBEBUILDER_ASSETS="$(shell $(LOCALBIN)/setup-envtest use $(ENVTEST_VERSION) --bin-path $(LOCALBIN)/envtest -p path)" \
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
		crd webhook paths="./..." \
		output:crd:artifacts:config=config/crd/bases \
		output:rbac:artifacts:config=config/rbac

## ─── Docker ──────────────────────────────────────────────────────────────────

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push Docker image
	docker push $(IMG)

.PHONY: docker-buildx
docker-buildx: ## Build and push multi-arch Docker image via buildx
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMG) --push .

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
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)
