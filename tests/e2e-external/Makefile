SHELL = bash

K8S_TEST_VERSION ?= v1.30.1
GINKGO = bin/ginkgo
E2E = bin/e2e.test

TEST_ARGS = -v -nodes=5
STRESS_ARGS = -v


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

test: bin
	$(check_e2e_kubeconfig)
	$(GINKGO) $(TEST_ARGS) \
		--focus 'csi.xen-orchestra.marcsi.ch' \
		--skip 'Disruptive' \
		--skip "volumeMode.*should not mount / map unused volumes" \
		--skip "multiple pods should access different volumes repeatedly" \
		$(E2E) -- \
		-storage.testdriver=$(TESTDRIVER) \
		--kubeconfig=$(E2EKUBECONFIG)

stress: bin
	$(check_e2e_kubeconfig)
	$(GINKGO) $(STRESS_ARGS) \
		--focus 'csi.xen-orchestra.marcsi.ch.*multiple pods should access different volumes repeatedly' \
		$(E2E) -- \
		-storage.testdriver=$(TESTDRIVER) \
		--kubeconfig=$(E2EKUBECONFIG)

test-shared: TESTDRIVER=../shared/testdriver.yaml
test-shared: test

stress-shared: TESTDRIVER=../shared/testdriver.yaml
stress-shared: stress

test-localmigrating: TESTDRIVER=../localmigrating/testdriver.yaml
test-localmigrating: test

stress-localmigrating: TESTDRIVER=../localmigrating/testdriver.yaml
stress-localmigrating: stress

# During Development, we can run the test with a specific focus
# test-directed: bin
# 	$(check_e2e_kubeconfig)
# 	$(GINKGO) $(GINKGO_ARGS) \
# 		--focus 'csi.xen-orchestra.marcsi.ch.*should fail in binding dynamic provisioned PV to PVC' \
# 		--skip 'Disruptive' \
# 		$(E2E) -- \
# 		-storage.testdriver=$(CURDIR)/shared/testdriver.yaml \
# 		--kubeconfig=$(E2EKUBECONFIG)

test-all: test-shared test-localmigrating
stress-all: stress-shared stress-localmigrating
all: test-all stress-all