IMAGE_NAME := ghcr.io/m4rcsi/csi-xen-orchestra-driver
IMAGE_TAG := dev
OVERLAY_DIR := deploy/kustomize/overlays/dev

.PHONY: build
build:
	@echo "Building container image..."
	podman build -t $(IMAGE_NAME):$(IMAGE_TAG) --format=oci .
	@echo "Image built successfully: $(IMAGE_NAME):$(IMAGE_TAG)"

.PHONY: update-kustomization
update-kustomization:
	@echo "Updating kustomization.yaml with image manifest digest..."
	$(eval DIGEST := $(shell skopeo inspect docker://$(IMAGE_NAME):$(IMAGE_TAG) --format '{{.Digest}}'))
	cd $(OVERLAY_DIR) && kustomize edit set image csi-xen-orchestra-driver=$(IMAGE_NAME)@$(DIGEST)
	@echo "Updated kustomization.yaml with digest: $(DIGEST)"

.PHONY: push
push: build
	podman push $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: deploy
deploy: build push update-kustomization
	kubectl apply -k $(OVERLAY_DIR)

build-xoa-jsonrpc:
	go build -o bin/xoa-jsonrpc ./cmd/xoa-jsonrpc/

.PHONY: clean
clean:
	rm -rf bin/

test:
	go test ./...

.PHONY: addlicense
addlicense:
	addlicense -c "Marc Siegenthaler" -l apache cmd pkg tests
