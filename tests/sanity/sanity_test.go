// Copyright 2025 Marc Siegenthaler
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sanity

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"testing"

	csisanity "github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	"github.com/m4rCsi/csi-xen-orchestra-driver/pkg/csi"
	"github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
)

var (
	csiSocketPath string
	fakeMounter   *FakeMounter
	fakeClient    *FakeClient
	driver        *csi.Driver
	tmpDirectory  string
)

const (
	// Test data

	// In the context of the test, these are the valid values for the SR and Node ID
	// the driver will accept these values as valid.
	// The node driver will be running as if it were on this FakeNodeID
	FakeSRUUID = "fake-existing-sr-uuid"
	FakeNodeID = "fake-existing-node-id"
)

func setupLogging() {
	// Disable default klog outputs, and ignore klog flags
	// https://github.com/kubernetes/klog/issues/87#issuecomment-1671820147
	flags := &flag.FlagSet{}
	klog.InitFlags(flags)
	_ = flags.Set("logtostderr", "false")
	_ = flags.Set("alsologtostderr", "false")
	_ = flags.Set("stderrthreshold", "4")
	_ = flags.Set("v", "4")

	// Send klog output to GinkgoWriter
	klog.SetOutput(GinkgoWriter)
}

func setup() {
	tmpDir, err := os.MkdirTemp("", "csi-sanity-*")
	if err != nil {
		Fail("Failed to create sanity temp working dir")
	}
	tmpDirectory = tmpDir
	csiSocketPath = fmt.Sprintf("unix:%s/csi.sock", tmpDir)

	fakeClient = NewFakeClient()
	if err := fakeClient.Connect(context.Background()); err != nil {
		Fail("Failed to connect to fake client")
	}

	fakeClient.InjectSR(&xoa.SR{UUID: FakeSRUUID, Host: "fake-host-id", Pool: "fake-pool-id", Tags: []string{"k8s-local"}, Size: 1024 * 1024 * 1024 * 100, Usage: 0}) // SR with 100GB of free space
	fakeClient.InjectVM(&xoa.VM{UUID: FakeNodeID, Host: "fake-host-id", Pool: "fake-pool-id"})
	fakeNodeMetadata := &FakeNodeMetadata{NodeID: FakeNodeID, HostID: "fake-host-id", PoolID: "fake-pool-id"}

	fakeMounter = NewFakeMounter()

	// Create CSI driver
	driver = csi.NewDriver(
		&csi.DriverOptions{
			Endpoint: csiSocketPath,
			Mode:     csi.AllMode,
		},
		fakeClient,
		fakeNodeMetadata,
		fakeMounter,
	)

	go func() {
		if err := driver.Run(); err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		klog.Infof("Driver started")
	}()
}

func cleanup() {
	_ = fakeClient.Close()
	driver.Stop()
	_ = os.RemoveAll(tmpDirectory)
}

func TestSanity(t *testing.T) {
	setupLogging()
	setup()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Sanity Suite")

	cleanup()
}

var _ = Describe("Xen CSI Driver", func() {

	Describe("Sanity 'local' with migrating Storage", func() {
		config := csisanity.NewTestConfig()
		// Set configuration options as needed
		config.Address = csiSocketPath
		config.TargetPath = path.Join(tmpDirectory, "mount")
		config.StagingPath = path.Join(tmpDirectory, "staging")
		config.TestVolumeSize = 1024 * 1024 * 1024 * 10 // 10GB
		config.TestVolumeAccessType = "mount"
		config.TestVolumeParameters = map[string]string{
			"type":       "local",
			"srsWithTag": "k8s-local",
			"migrating":  "true",
		}

		config.CheckPath = func(path string) (csisanity.PathKind, error) {
			return fakeMounter.CheckPath(path)
		}
		config.CreateTargetDir = func(path string) (string, error) {
			return fakeMounter.CreateDir(path)
		}
		config.RemoveTargetPath = func(path string) error {
			return fakeMounter.RemovePath(path)
		}
		config.CreateStagingDir = func(path string) (string, error) {
			return fakeMounter.CreateDir(path)
		}
		config.RemoveStagingPath = func(path string) error {
			return fakeMounter.RemovePath(path)
		}

		csisanity.GinkgoTest(&config)
	})

	Describe("Sanity 'local' Storage", func() {
		config := csisanity.NewTestConfig()
		// Set configuration options as needed
		config.Address = csiSocketPath
		config.TargetPath = path.Join(tmpDirectory, "mount")
		config.StagingPath = path.Join(tmpDirectory, "staging")
		config.TestVolumeSize = 1024 * 1024 * 1024 * 10 // 10GB
		config.TestVolumeAccessType = "mount"
		config.TestVolumeParameters = map[string]string{
			"type":       "local",
			"srsWithTag": "k8s-local",
		}

		config.CheckPath = func(path string) (csisanity.PathKind, error) {
			return fakeMounter.CheckPath(path)
		}
		config.CreateTargetDir = func(path string) (string, error) {
			return fakeMounter.CreateDir(path)
		}
		config.RemoveTargetPath = func(path string) error {
			return fakeMounter.RemovePath(path)
		}
		config.CreateStagingDir = func(path string) (string, error) {
			return fakeMounter.CreateDir(path)
		}
		config.RemoveStagingPath = func(path string) error {
			return fakeMounter.RemovePath(path)
		}

		csisanity.GinkgoTest(&config)
	})

	Describe("Sanity 'shared' Storage", func() {
		config := csisanity.NewTestConfig()
		// Set configuration options as needed
		config.Address = csiSocketPath
		config.TargetPath = path.Join(tmpDirectory, "mount")
		config.StagingPath = path.Join(tmpDirectory, "staging")
		config.TestVolumeSize = 1024 * 1024 * 1024 * 10 // 10GB
		config.TestVolumeAccessType = "mount"

		config.TestVolumeParameters = map[string]string{
			"type":   "shared",
			"srUUID": FakeSRUUID,
		}

		config.CheckPath = func(path string) (csisanity.PathKind, error) {
			return fakeMounter.CheckPath(path)
		}
		config.CreateTargetDir = func(path string) (string, error) {
			return fakeMounter.CreateDir(path)
		}
		config.RemoveTargetPath = func(path string) error {
			return fakeMounter.RemovePath(path)
		}
		config.CreateStagingDir = func(path string) (string, error) {
			return fakeMounter.CreateDir(path)
		}
		config.RemoveStagingPath = func(path string) error {
			return fakeMounter.RemovePath(path)
		}

		csisanity.GinkgoTest(&config)
	})

})
