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
	"strings"
)

const (
	DefaultDiskNamePrefix = "csi-"
)

type DiskNameGenerator struct {
	prefix     string
	tempPrefix string
}

func NewDiskNameGenerator(prefix string) *DiskNameGenerator {
	if prefix == "" {
		prefix = DefaultDiskNamePrefix
	}
	return &DiskNameGenerator{prefix: prefix, tempPrefix: prefix + "temp-"}
}

func (v *DiskNameGenerator) FromVolumeName(volumeName string) string {
	return v.prefix + volumeName
}

func (v *DiskNameGenerator) TemporaryFromVolumeName(volumeName string) string {
	return v.tempPrefix + volumeName
}

func (v *DiskNameGenerator) IsTemporaryDisk(diskName string) bool {
	// If the temp prefix is empty, we can't check for it.
	// Should never happen, but we'll be defensive.
	if v.tempPrefix == "" {
		return false
	}
	return strings.HasPrefix(diskName, v.tempPrefix)
}
