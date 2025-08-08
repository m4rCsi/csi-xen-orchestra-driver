SHELL = bash

K8S_TEST_VERSION ?= v1.30.1
GINKGO = bin/ginkgo
E2E = bin/e2e.test

# add -v to see more logs
TEST_ARGS = -nodes=5
STRESS_ARGS =

# Supported driver configurations
CONFIGS := shared localmigrating

TEST_TARGETS := $(addprefix test-,$(CONFIGS))
STRESS_TARGETS := $(addprefix stress-,$(CONFIGS))


test: $(TEST_TARGETS)
stress: $(STRESS_TARGETS)
localmigrating: test-localmigrating stress-localmigrating
shared: test-shared stress-shared
all: test-all stress-all

# Function to check if E2EKUBECONFIG is set
define check_e2e_kubeconfig
	@if [ -z "$(E2EKUBECONFIG)" ]; then \
		echo "Error: E2EKUBECONFIG environment variable is not set"; \
		echo "Please set E2EKUBECONFIG to the path of a kubeconfig file with a prepared cluster for e2e tests"; \
		exit 1; \
	fi
endef

bin:
	mkdir -p bin
	curl -sSL "https://dl.k8s.io/$(K8S_TEST_VERSION)/kubernetes-test-linux-amd64.tar.gz" | \
		tar --strip-components=3 -C bin -zxf - \
			kubernetes/test/bin/e2e.test \
			kubernetes/test/bin/ginkgo

.PHONY: test-%
test-%: bin
	$(check_e2e_kubeconfig)
	$(GINKGO) $(TEST_ARGS) \
		--focus 'csi.xen-orchestra.marcsi.ch' \
		--skip 'Disruptive' \
		--skip "volumeMode.*should not mount / map unused volumes" \
		--skip "multiple pods should access different volumes repeatedly" \
		$(E2E) -- \
		-storage.testdriver=../$*/testdriver.yaml \
		--kubeconfig=$(E2EKUBECONFIG)

.PHONY: stress-%
stress-%: bin
	$(check_e2e_kubeconfig)
	$(GINKGO) $(STRESS_ARGS) \
		--focus 'csi.xen-orchestra.marcsi.ch.*multiple pods should access different volumes repeatedly' \
		$(E2E) -- \
		-storage.testdriver=../$*/testdriver.yaml \
		--kubeconfig=$(E2EKUBECONFIG)
