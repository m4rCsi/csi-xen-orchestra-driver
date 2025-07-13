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
	"fmt"
	"strings"

	csisanity "github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	"github.com/m4rCsi/csi-xen-orchestra-driver/pkg/csi"
	"k8s.io/klog/v2"
)

var _ csi.Mounter = &FakeMounter{}

// FakeMounter implements the csi.Mounter interface for testing purposes
// It is used to simulate a real mounter for testing purposes
// TODO: Unsure what level of testing we are meant to do for the sanity tests.
//
//	For now, this abstracts away the mounter and just tracks the directories that are mounted.
type FakeMounter struct {
	dirs map[string]bool
}

func NewFakeMounter() *FakeMounter {
	return &FakeMounter{
		dirs: make(map[string]bool),
	}
}

func (f *FakeMounter) FormatAndMount(source, target, fstype string, options []string) error {
	klog.V(5).InfoS("Formatting and mounting", "source", source, "target", target, "fstype", fstype, "options", options)

	if !strings.HasPrefix(source, "/dev/") {
		if _, ok := f.dirs[source]; !ok {
			return fmt.Errorf("source directory does not exist")
		}
	}

	f.dirs[target] = true
	return nil
}

func (f *FakeMounter) Unmount(target string) error {
	klog.V(5).InfoS("Unmounting", "target", target)
	delete(f.dirs, target)
	return nil
}

func (f *FakeMounter) Mount(source, target, fstype string, options []string) error {
	klog.V(5).InfoS("Mounting", "source", source, "target", target)

	if !strings.HasPrefix(source, "/dev/") {
		if _, ok := f.dirs[source]; !ok {
			return fmt.Errorf("source directory does not exist")
		}
	}

	f.dirs[target] = true
	return nil
}

func (f *FakeMounter) CheckPath(path string) (csisanity.PathKind, error) {
	klog.V(5).InfoS("Checking path", "path", path)
	if _, ok := f.dirs[path]; ok {
		return csisanity.PathIsDir, nil
	}
	return csisanity.PathIsNotFound, nil
}

func (f *FakeMounter) CreateDir(path string) (string, error) {
	klog.V(5).InfoS("Creating dir", "path", path)
	f.dirs[path] = true
	return path, nil
}

func (f *FakeMounter) RemovePath(path string) error {
	klog.V(5).InfoS("Removing dir", "path", path)
	delete(f.dirs, path)
	return nil
}

func (f *FakeMounter) FindDevicePath(deviceName string, vbdUUID string) (string, error) {
	klog.V(5).InfoS("Finding device path", "deviceName", deviceName, "vbdUUID", vbdUUID)
	return "/dev/" + deviceName, nil
}

func (f *FakeMounter) Resize(devicePath, deviceMountPath string) (bool, error) {
	klog.V(5).InfoS("Resizing", "devicePath", devicePath, "deviceMountPath", deviceMountPath)
	return true, nil
}

func (f *FakeMounter) NeedResize(devicePath, deviceMountPath string) (bool, error) {
	klog.V(5).InfoS("Need resizing", "devicePath", devicePath, "deviceMountPath", deviceMountPath)
	return false, nil
}
