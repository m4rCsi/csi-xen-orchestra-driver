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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type VolumeIDType string

const (
	NameAsVolumeID  VolumeIDType = "name"
	UUIDAsVolumeID  VolumeIDType = "uuid"
	InvalidVolumeID VolumeIDType = "invalid"
)
const (
	VDIDiskPrefix          = "csi-"
	VDIDiskPrefixTemporary = "csi-temp-"
)

func CreateVolumeIDWithName(name string) string {
	return "name:" + name
}

func CreateVolumeIDWithUUID(uuid string) string {
	return "uuid:" + uuid
}

func ParseVolumeID(volumeID string) (VolumeIDType, string, error) {
	if volumeID == "" {
		return InvalidVolumeID, volumeID, status.Errorf(codes.InvalidArgument, "volume ID is required")
	}

	if strings.HasPrefix(volumeID, "name:") {
		return NameAsVolumeID, volumeID[5:], nil
	} else if strings.HasPrefix(volumeID, "uuid:") {
		return UUIDAsVolumeID, volumeID[5:], nil
	} else {
		return InvalidVolumeID, volumeID, status.Errorf(codes.NotFound, "invalid volume ID: %s", volumeID)
	}
}

func DiskNameFromVolumeName(volumeName string) string {
	return VDIDiskPrefix + volumeName
}

func TemporaryDiskNameFromVolumeName(volumeName string) string {
	return VDIDiskPrefixTemporary + volumeName
}
