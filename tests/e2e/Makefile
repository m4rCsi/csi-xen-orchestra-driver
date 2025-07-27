SHELL = bash

K8S_TEST_VERSION ?= v1.30.1
GINKGO = bin/ginkgo
GINKGO_ARGS = -v -nodes=4

E2E = bin/e2e.test

bin:
	mkdir -p bin
	curl -sSL "https://dl.k8s.io/$(K8S_TEST_VERSION)/kubernetes-test-linux-amd64.tar.gz" | \
		tar --strip-components=3 -C bin -zxf - \
			kubernetes/test/bin/e2e.test \
			kubernetes/test/bin/ginkgo

test-shared: bin
	$(GINKGO) $(GINKGO_ARGS) \
		--focus 'csi.xen-orchestra.marcsi.ch' \
		--skip 'Disruptive' \
		--skip "volumeMode.*should not mount / map unused volumes" \
		$(E2E) -- -storage.testdriver=../shared/testdriver.yaml

test-migrating: bin
	$(GINKGO) $(GINKGO_ARGS) \
		--focus 'csi.xen-orchestra.marcsi.ch' \
		--skip 'Disruptive' \
		--skip "volumeMode.*should not mount / map unused volumes" \
		$(E2E) -- -storage.testdriver=../migrating/testdriver.yaml


# During Development, we can run the test with a specific focus
# test-directed: bin
# 	$(GINKGO) $(GINKGO_ARGS) \
# 		--focus 'csi.xen-orchestra.marcsi.ch.*should fail in binding dynamic provisioned PV to PVC' \
# 		--skip 'Disruptive' \
# 		$(E2E) -- -storage.testdriver=$(CURDIR)/shared/testdriver.yaml


test: test-shared test-migrating