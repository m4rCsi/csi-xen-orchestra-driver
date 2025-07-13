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

package csi

import (
	mountutils "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
)

// Mounter is an interface that provides methods to mount and unmount volumes.
// It is used to abstract the underlying filesystem implementation.
type Mounter interface {
	FormatAndMount(source string, target string, fstype string, options []string) error
	Unmount(target string) error
	Mount(source, target, fstype string, options []string) error
	FindDevicePath(deviceName string, vbdUUID string) (string, error)
	NeedResize(source, target string) (bool, error)
	Resize(devicePath, deviceMountPath string) (bool, error)
	GetDeviceNameFromMount(mountPath string) (string, int, error)
}

type SafeMounter struct {
	mounter     mountutils.Interface
	safeMounter *mountutils.SafeFormatAndMount
	exec        utilexec.Interface
}

func NewSafeMounter() *SafeMounter {
	mounter := mountutils.New("")
	exec := utilexec.New()
	return &SafeMounter{
		mounter: mounter,
		safeMounter: &mountutils.SafeFormatAndMount{
			Interface: mounter,
			Exec:      exec,
		},
		exec: exec,
	}
}

func (s *SafeMounter) FormatAndMount(source, target, fstype string, options []string) error {
	return s.safeMounter.FormatAndMount(source, target, fstype, options)
}

func (s *SafeMounter) Unmount(target string) error {
	return mountutils.CleanupMountPoint(target, s.mounter, true)
}

func (s *SafeMounter) Mount(source, target, fstype string, options []string) error {
	return s.mounter.Mount(source, target, fstype, options)
}

func (s *SafeMounter) NeedResize(devicePath, deviceMountPath string) (bool, error) {
	return mountutils.NewResizeFs(s.exec).NeedResize(devicePath, deviceMountPath)
}

func (s *SafeMounter) Resize(devicePath, deviceMountPath string) (bool, error) {
	resizeFs := mountutils.NewResizeFs(s.exec)
	return resizeFs.Resize(devicePath, deviceMountPath)
}

func (s *SafeMounter) FindDevicePath(deviceName string, vbdUUID string) (string, error) {
	// Ideally we have a way to figure out the device path from the vbdUUID
	// but we don't have that information here.
	return "/dev/" + deviceName, nil
}

func (s *SafeMounter) GetDeviceNameFromMount(mountPath string) (string, int, error) {
	return mountutils.GetDeviceNameFromMount(s.mounter, mountPath)
}

// Compile time check to ensure SafeMounter implements the Mounter interface
var _ Mounter = &SafeMounter{}
