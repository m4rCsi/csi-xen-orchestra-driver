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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StorageType string
type StorageRepositorySelection string

const (
	StorageTypeShared         StorageType = "shared"
	StorageTypeLocalMigrating StorageType = "localmigrating"
	StorageTypeStatic         StorageType = "static"

	StorageRepositorySelectionTag  StorageRepositorySelection = "tag"
	StorageRepositorySelectionUUID StorageRepositorySelection = "uuid"
)

type storageParmaters struct {
	Type       StorageType
	SRUUID     string
	SRsWithTag string
}

func LoadStorageParametersFromVolumeContext(volumeContext map[string]string) (*storageParmaters, error) {
	storageParams := &storageParmaters{}
	storageType := StorageType(volumeContext["type"])
	if storageType == "" {
		storageType = StorageTypeStatic
	}
	storageParams.Type = storageType
	switch storageType {
	case StorageTypeLocalMigrating:
		storageParams.SRsWithTag = volumeContext["srsWithTag"]
		if storageParams.SRsWithTag == "" {
			return nil, status.Errorf(codes.InvalidArgument, "srsWithTag is required")
		}
	case StorageTypeShared:
		storageParams.SRUUID = volumeContext["srUUID"]
		if storageParams.SRUUID == "" {
			return nil, status.Errorf(codes.InvalidArgument, "srUUID is required")
		}
	case StorageTypeStatic:
		// We don't need any parameters
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid storage type: %s", storageType)
	}
	return storageParams, nil
}

func (s *storageParmaters) GenerateVolumeContext() map[string]string {
	d := map[string]string{
		"type": string(s.Type),
	}

	switch s.Type {
	case StorageTypeLocalMigrating:
		d["srsWithTag"] = s.SRsWithTag
	case StorageTypeShared:
		d["srUUID"] = s.SRUUID
	}

	return d
}

func LoadStorageParameters(parameters map[string]string) (*storageParmaters, error) {
	storageParams := &storageParmaters{}

	storageType := StorageType(parameters["type"])
	storageParams.Type = storageType
	switch storageType {
	case StorageTypeLocalMigrating:
		srsWithTag := parameters["srsWithTag"]

		if srsWithTag == "" {
			return nil, status.Errorf(codes.InvalidArgument, "srsWithTag is required")
		}

		storageParams.SRsWithTag = srsWithTag
	case StorageTypeShared:
		srUUID := parameters["srUUID"]
		if srUUID == "" {
			return nil, status.Errorf(codes.InvalidArgument, "srUUID is required")
		} else {
			storageParams.SRUUID = srUUID
		}
	case "":
		return nil, status.Errorf(codes.InvalidArgument, "type is required")
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid storage type: %s", storageType)
	}

	return storageParams, nil
}

func (s *storageParmaters) getSRSelection() (StorageRepositorySelection, string, error) {
	switch s.Type {
	case StorageTypeLocalMigrating:
		return StorageRepositorySelectionTag, s.SRsWithTag, nil
	case StorageTypeShared:
		return StorageRepositorySelectionUUID, s.SRUUID, nil
	default:
		return "", "", status.Errorf(codes.InvalidArgument, "invalid storage type: %s", s.Type)
	}
}

func (s *storageParmaters) VolumeIDType() VolumeIDType {
	switch s.Type {
	case StorageTypeLocalMigrating:
		// Because the UUID changes when we migrate the volume to the other SRs
		// we use the name as the volume ID (which stays the same)
		// However, this is less robust than using the UUID, because the name is not unique
		// and can be changed by the user.
		return NameAsVolumeID
	case StorageTypeShared:
		return UUIDAsVolumeID
	default:
		return UUIDAsVolumeID
	}
}
