# Set RELEASE=true or RELEASE=1 to use release settings
# Default is development mode
RELEASE ?= false

IMAGE_NAME := ghcr.io/m4rcsi/csi-xen-orchestra-driver
HELM_REPO := ghcr.io/m4rcsi/charts/
VERSION := 0.2.3
IMAGE_TAG := v$(VERSION)
CHART_VERSION := $(VERSION)

ifneq ($(RELEASE),true)
    # Check if DEVELOPMENT_IMAGE_NAME is set for development mode
    ifndef DEVELOPMENT_IMAGE_NAME
        $(error DEVELOPMENT_IMAGE_NAME environment variable is required for development mode)
    endif

    IMAGE_NAME := $(DEVELOPMENT_IMAGE_NAME)
    HELM_REPO := ghcr.io/m4rcsi/charts/
    VERSION := dev
    IMAGE_TAG := dev
    CHART_VERSION := v0.0.0-dev
endif

.PHONY: build
build:
	@echo "Building container image..."
	podman build -t $(IMAGE_NAME):$(IMAGE_TAG) --format=oci .
	@echo "Image built successfully: $(IMAGE_NAME):$(IMAGE_TAG)"

build-xoa-jsonrpc:
	go build -o bin/xoa-jsonrpc ./cmd/xoa-jsonrpc/

.PHONY: push
push: build
	podman push $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: deploy
deploy: push
	@echo "Getting image digest..."
	$(eval DIGEST := $(shell skopeo inspect docker://$(IMAGE_NAME):$(IMAGE_TAG) --format '{{.Digest}}'))
	@echo "Deploying chart..."
	helm upgrade csi-xen-orchestra \
	  		./charts/csi-xen-orchestra-driver \
			--install \
			--namespace kube-system \
			--values charts/csi-xen-orchestra-driver/values-resources.yaml \
			--values dev-values.yaml \
			--set csiXenOrchestraDriver.image.repository=$(IMAGE_NAME) \
			--set csiXenOrchestraDriver.image.digest=$(DIGEST) \
			$(if $(EXTRA_HELM_VALUES),$(EXTRA_HELM_VALUES),)

helm-package:
	helm package charts/csi-xen-orchestra-driver -d dist --app-version v$(VERSION) --version $(CHART_VERSION)

helm-publish: helm-package
	helm push dist/csi-xen-orchestra-driver-$(CHART_VERSION).tgz oci://$(HELM_REPO)

.PHONY: clean
clean:
	rm -rf bin/

test:
	go test ./...

.PHONE: lint
lint:
	golangci-lint run

.PHONY: addlicense
addlicense:
	addlicense -c "Marc Siegenthaler" -l apache -ignore '**/*.yaml' -ignore '**/*.md' -ignore '**/*.json' cmd pkg tests

