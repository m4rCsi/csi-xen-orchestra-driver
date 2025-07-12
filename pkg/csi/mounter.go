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
}

type SafeMounter struct {
	mounter     mountutils.Interface
	safeMounter *mountutils.SafeFormatAndMount
}

func NewSafeMounter() *SafeMounter {
	mounter := mountutils.New("")
	return &SafeMounter{
		mounter: mounter,
		safeMounter: &mountutils.SafeFormatAndMount{
			Interface: mounter,
			Exec:      utilexec.New(),
		},
	}
}

func (s *SafeMounter) FormatAndMount(source, target, fstype string, options []string) error {
	return s.safeMounter.FormatAndMount(source, target, fstype, options)
}

func (s *SafeMounter) Unmount(target string) error {
	return s.mounter.Unmount(target)
}

func (s *SafeMounter) Mount(source, target, fstype string, options []string) error {
	return s.mounter.Mount(source, target, fstype, options)
}
