package csi

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StorageType string
type VolumeIDType string

const (
	StorageTypeShared    StorageType = "shared"
	StorageTypeMigrating StorageType = "migrating"
)

const (
	NameAsVolumeID  VolumeIDType = "name"
	UUIDAsVolumeID  VolumeIDType = "uuid"
	InvalidVolumeID VolumeIDType = "invalid"
)

type storageParmaters struct {
	Type    StorageType
	SRUUID  string
	SRUUIDs []string
}

func LoadStorageParametersFromVolumeContext(volumeContext map[string]string) (*storageParmaters, error) {
	storageParams := &storageParmaters{}
	storageType := StorageType(volumeContext["type"])
	storageParams.Type = storageType
	switch storageType {
	case StorageTypeMigrating:
		storageParams.SRUUIDs = strings.Split(volumeContext["srUUIDs"], ",")
	case StorageTypeShared:
		storageParams.SRUUID = volumeContext["srUUID"]
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
	case StorageTypeMigrating:
		d["srUUIDs"] = strings.Join(s.SRUUIDs, ",")
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
	case StorageTypeMigrating:
		srUUIDs := parameters["srUUIDs"]
		if srUUIDs == "" {
			// TODO: Automode
			// storageParams.SRUUIDs = []string{}
			return nil, status.Errorf(codes.InvalidArgument, "srUUIDs is required")
		} else {
			storageParams.SRUUIDs = strings.Split(srUUIDs, ",")
			if len(storageParams.SRUUIDs) == 0 {
				return nil, status.Errorf(codes.InvalidArgument, "srUUIDs is required")
			}
		}
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

func (s *storageParmaters) getSRUUIDForCreation() string {
	if s.Type == StorageTypeMigrating {
		return s.SRUUIDs[0]
	} else {
		return s.SRUUID
	}
}

func (s *storageParmaters) getSRUUIDs() ([]string, error) {
	if s.Type == StorageTypeMigrating {
		return s.SRUUIDs, nil
	} else {
		return []string{s.SRUUID}, nil
	}
}

func (s *storageParmaters) VolumeIDType() VolumeIDType {
	switch s.Type {
	case StorageTypeMigrating:
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
