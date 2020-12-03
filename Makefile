
.DEFAULT_GOAL:=help

VERSION_SUFFIX ?= -dev
PROD_VERSION ?= 0.7.4${VERSION_SUFFIX}
PROD_BUILD_ID ?= latest

IMG_URL ?= gcr.io/$(shell gcloud config get-value project)/${USER}
IMG_TAG ?= latest
STATIC_IP_IMG ?= ${IMG_URL}/capv-static-ip:${IMG_TAG}
OVERLAY ?= base

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"
COVER_DIR=_build/cov
COVER_PKGS=$(shell go list ./... | grep -vE 'tests|fake|cmd|hack|config' | tr "\n" ",")
MANIFEST_DIR=_build/manifests
export CURRENT_DIR=${CURDIR}


all: generate manifests static bin test

## --------------------------------------
## Help
## --------------------------------------

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

static: fmt vet lint ## Run static code analysis
fmt: ## Run go fmt against code
	go fmt ./...
vet: ## Run go vet against code
	go vet ./...
lint: ## Run golangci-lint  against code
	golangci-lint run    ./...  --timeout 10m  --tests=false

test: test-integration gocovmerge gocover ## Run tests
	$(GOCOVMERGE) $(COVER_DIR)/*.out > $(COVER_DIR)/coverage.out
	go tool cover -func=$(COVER_DIR)/coverage.out -o $(COVER_DIR)/cover.func
	go tool cover -html=$(COVER_DIR)/coverage.out -o $(COVER_DIR)/cover.html
	go tool cover -func $(COVER_DIR)/coverage.out | grep total

test-integration:  ## Run integration tests
	@mkdir -p $(COVER_DIR)
	rm -f $(COVER_DIR)/*
	go test -v -covermode=count -coverprofile=$(COVER_DIR)/int.out --coverpkg=$(COVER_PKGS) ./tests/...


manager: generate fmt vet ## Build manager binary
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${STATIC_IP_IMG}
	kustomize build config/default | kubectl apply -f -

manifests: controller-gen ## Generate manifests e.g. CRD, RBAC etc.
	@mkdir -p $(MANIFEST_DIR)
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cd config/manager && kustomize edit set image controller=${STATIC_IP_IMG}
	kustomize build config/default > $(MANIFEST_DIR)/staticip-manifest.yaml

generate: controller-gen ## Generate code
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

bin: generate ## Generate binaries
	go build -o bin/manager main.go

docker: docker-build docker-push ## Tags docker image and also pushes it to container registry

docker-build: ## Build the docker image for controller-manager
	docker build . -t ${STATIC_IP_IMG}

docker-push: ## Push the docker image
	docker push ${STATIC_IP_IMG}

docker-rmi: ## Remove the local docker image
	docker rmi ${STATIC_IP_IMG}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

gocovmerge:
ifeq (, $(shell which gocovmerge))
	go get github.com/wadey/gocovmerge
	go mod tidy
GOCOVMERGE=$(GOBIN)/gocovmerge
else
GOCOVMERGE=$(shell which gocovmerge)
endif

gocover:
ifeq (, $(shell which cover))
	go get golang.org/x/tools/cmd/cover
	go mod tidy
GOCOVER=$(GOBIN)/cover
else
GOCOVER=$(shell which cover)
endif

version: ## Prints version of current make
	@echo $(PROD_VERSION)
