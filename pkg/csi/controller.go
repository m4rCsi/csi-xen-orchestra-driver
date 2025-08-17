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
	"context"
	"errors"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	xoa "github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"
)

type ControllerService struct {
	csi.UnimplementedControllerServer
	driver            *Driver
	xoaClient         xoa.Client
	diskNameGenerator *DiskNameGenerator
	creationLock      *CreationLock
}

func NewControllerService(driver *Driver, xoaClient xoa.Client, diskNameGenerator *DiskNameGenerator, creationLock *CreationLock) *ControllerService {
	return &ControllerService{
		driver:            driver,
		xoaClient:         xoaClient,
		diskNameGenerator: diskNameGenerator,
		creationLock:      creationLock,
	}
}

func (cs *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	_, ctx = LogAndExpandContext(ctx, "req", req)
	volumeName := req.GetName()

	if volumeName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "disk name is required")
	}

	if req.VolumeContentSource != nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume content source is not supported")
	}

	storageParams, err := LoadStorageParameters(req.GetParameters())
	if errors.Is(err, ErrInvalidStorageParameters) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid storage parameters: %v", err)
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load storage parameters: %v", err)
	}

	capabilities := req.GetVolumeCapabilities()
	if len(capabilities) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "volume capabilities are required")
	}

	if !isValidVolumeCapabilities(capabilities) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capabilities")
	}

	var capacity int64
	if req.GetCapacityRange() != nil {
		capacity = req.GetCapacityRange().GetRequiredBytes()
	} else {
		// default to 1GB
		capacity = 1024 * 1024 * 1024
	}

	storageSelection := storageSelectionFromParameters(storageParams)
	err = storageSelection.findSRs(ctx, cs.xoaClient)
	if errors.Is(err, ErrNoSRFound) {
		return nil, status.Errorf(codes.FailedPrecondition, "no SRs found for storage selection")
	} else if errors.Is(err, ErrInconsistentSRs) {
		return nil, status.Errorf(codes.FailedPrecondition, "inconsistent SRs found for storage selection")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find SRs: %v", err)
	}

	var vdi *xoa.VDI

	// Check first, if the disk with the right name already exists
	diskName := cs.diskNameGenerator.FromVolumeName(volumeName)
	foundVdi, err := cs.xoaClient.GetVDIByName(ctx, diskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		// volume does not exist
		// continue with creation
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		return nil, status.Errorf(codes.AlreadyExists, "multiple disks found with same name already exist")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	} else {
		// volume exists, check if size matches
		if foundVdi.Size != capacity {
			return nil, status.Errorf(codes.AlreadyExists, "volume already exists but size mismatch")
		}

		vdi = foundVdi
	}

	if vdi == nil {
		createdVDI, err := cs.createDisk(ctx, storageSelection, volumeName, capacity, req.AccessibilityRequirements)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create disk: %v", err)
		}
		vdi = createdVDI
	}

	// Ensure the VDI has the right storage info stored in the description
	storageInfo := storageSelection.toStorageInfo()
	err = cs.xoaClient.EditVDI(ctx, vdi.UUID, nil, ptr.To(storageInfo.ToVDIDescription()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to edit disk: %v", err)
	}

	// Find out the topology for the VDI
	csiTopology, err := storageSelection.getTopologyForVDI(vdi)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get topology: %v", err)
	}

	// Which reference we use for this type of volume
	var volumeID string
	switch storageSelection.VolumeIDType() {
	case NameAsVolumeID:
		volumeID = CreateVolumeIDWithName(volumeName)
	case UUIDAsVolumeID:
		volumeID = CreateVolumeIDWithUUID(vdi.UUID)
	default:
		return nil, status.Errorf(codes.Internal, "invalid volume ID type")
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           volumeID,
			CapacityBytes:      capacity,
			AccessibleTopology: csiTopology,
		},
	}, nil
}

func (cs *ControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	_, ctx = LogAndExpandContext(ctx, "req", req)

	vidType, volumeIDValue, err := ParseVolumeID(req.GetVolumeId())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// If the volume ID is invalid, we can just return an empty response
			// because the volume is already deleted
			return &csi.DeleteVolumeResponse{}, nil
		} else {
			return nil, err
		}
	}

	var vdiUUIDToDelete string
	switch vidType {
	case NameAsVolumeID:
		diskName := cs.diskNameGenerator.FromVolumeName(volumeIDValue)
		vdi, err := cs.xoaClient.GetVDIByName(ctx, diskName)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			// volume already deleted
			return &csi.DeleteVolumeResponse{}, nil
		} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
			return nil, status.Errorf(codes.Internal, "multiple VDIs found with same name")
		} else if err != nil {
			return nil, status.Errorf(codes.NotFound, "failed to get volume: %v", err)
		}
		vdiUUIDToDelete = vdi.UUID
	case UUIDAsVolumeID:
		vdiUUID := volumeIDValue
		_, err := cs.xoaClient.GetVDIByUUID(ctx, vdiUUID)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			// volume already deleted
			return &csi.DeleteVolumeResponse{}, nil
		} else if err != nil {
			return nil, status.Errorf(codes.NotFound, "failed to get volume: %v", err)
		}
		vdiUUIDToDelete = vdiUUID
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID type")
	}

	err = cs.xoaClient.DeleteVDI(ctx, vdiUUIDToDelete)
	if errors.Is(err, xoa.ErrNoSuchObject) {
		// volume already deleted
		return &csi.DeleteVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete volume: %v", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *ControllerService) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	log, ctx := LogAndExpandContext(ctx, "req", req)

	vmUUID := req.GetNodeId()
	if vmUUID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "node ID is required")
	}

	if !isValidCapability(req.GetVolumeCapability()) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capability")
	}

	vdi, storageInfo, err := cs.findDisk(ctx, req.GetVolumeId())
	if err != nil {
		return nil, err
	}

	vm, err := cs.xoaClient.GetVMByUUID(ctx, vmUUID)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		return nil, status.Errorf(codes.NotFound, "VM not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get VM: %v", err)
	}

	vbd, err := cs.checkDiskAttachment(ctx, vdi, vm)
	if err != nil {
		return nil, err
	}
	if vbd != nil {
		// Already attached, return the publish context
		return &csi.ControllerPublishVolumeResponse{
			PublishContext: publishContextFromVBD(vbd),
		}, nil
	}

	storageSelection := storageSelectionFromStorageInfo(storageInfo, vdi)
	err = storageSelection.findSRs(ctx, cs.xoaClient)
	if errors.Is(err, ErrNoSRFound) {
		return nil, status.Errorf(codes.FailedPrecondition, "no SRs found for storage selection")
	} else if errors.Is(err, ErrInconsistentSRs) {
		return nil, status.Errorf(codes.FailedPrecondition, "inconsistent SRs found for storage selection")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find SRs: %v", err)
	}

	log.Info("VDI is not attached to VM", "vdiUUID", vdi.UUID, "vmUUID", vmUUID)

	needsMigration, err := storageSelection.needsMigration(vm)
	if errors.Is(err, ErrSRNotValidForHost) {
		return nil, status.Errorf(codes.FailedPrecondition, "SR is not valid for host %s", vm.Host)
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if migration is needed: %v", err)
	}

	if needsMigration {
		migratedvdi, _, err := cs.migrateVDI(ctx, vdi, vm, storageSelection)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to migrate VDI: %v", err)
		}
		vdi = migratedvdi
	}

	// Attach the VDI to the VM in RW mode
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	vbd, err = cs.xoaClient.AttachVDIAndWaitForDevice(ctxWithTimeout, vm.UUID, vdi.UUID, "RW")
	if err != nil {
		if errors.Is(err, xoa.ErrNoSuchObject) {
			return nil, status.Errorf(codes.NotFound, "not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to publish disk: %v", err)
	}

	if vbd.Device == "" {
		return nil, status.Errorf(codes.Internal, "failed to publish disk: no device found")
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"device": vbd.Device,
			"vbd":    vbd.UUID,
		},
	}, nil
}

func (cs *ControllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	log, ctx := LogAndExpandContext(ctx, "req", req)

	vidType, volumeIDValue, err := ParseVolumeID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	var vdiUUID string
	switch vidType {
	case NameAsVolumeID:
		diskName := cs.diskNameGenerator.FromVolumeName(volumeIDValue)
		vdi, err := cs.xoaClient.GetVDIByName(ctx, diskName)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			// VDI not found, nothing to do
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
			return nil, status.Errorf(codes.Unavailable, "multiple VDIs found with same name, likely migration in progress, please try again later")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get VDI: %v", err)
		}
		vdiUUID = vdi.UUID
	case UUIDAsVolumeID:
		vdiUUID = volumeIDValue
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID type")
	}

	vmUUID := req.GetNodeId()
	var vbds []xoa.VBD
	if vmUUID == "" {
		allvbds, err := cs.xoaClient.GetVBDsByVDI(ctx, vdiUUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get VBDs: %v", err)
		}

		vbds = allvbds
	} else {
		attachedVBDs, err := cs.xoaClient.GetVBDsByVMAndVDI(ctx, vmUUID, vdiUUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get VBDs: %v", err)
		}
		vbds = attachedVBDs
	}
	for _, vbd := range vbds {
		log.V(2).Info("Deleting VBD", "vbd", vbd)
		err := cs.xoaClient.DeleteVBD(ctx, vbd.UUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to delete VBD: %v", err)
		}
	}
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (cs *ControllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	vidType, volumeIDValue, err := ParseVolumeID(req.GetVolumeId())
	if err != nil {
		return nil, err
	}

	switch vidType {
	case NameAsVolumeID:
		diskName := cs.diskNameGenerator.FromVolumeName(volumeIDValue)
		_, err := cs.xoaClient.GetVDIByName(ctx, diskName)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return nil, status.Errorf(codes.NotFound, "volume not found")
		} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
			return nil, status.Errorf(codes.Unavailable, "multiple VDIs found with same name, likely migration in progress, please try again later")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
		}
	case UUIDAsVolumeID:
		vdiUUID := volumeIDValue
		_, err := cs.xoaClient.GetVDIByUUID(ctx, vdiUUID)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return nil, status.Errorf(codes.NotFound, "volume not found")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID type")
	}

	volumeCapabilities := req.GetVolumeCapabilities()
	if len(volumeCapabilities) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "volume capabilities are required")
	}

	if !isValidVolumeCapabilities(volumeCapabilities) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capabilities")
	}

	confirmed := &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volumeCapabilities}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func (cs *ControllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	_, ctx = LogAndExpandContext(ctx, "req", req)

	vidType, volumeIDValue, err := ParseVolumeID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	var vdi *xoa.VDI = nil
	switch vidType {
	case NameAsVolumeID:
		diskName := cs.diskNameGenerator.FromVolumeName(volumeIDValue)
		foundVdi, err := cs.xoaClient.GetVDIByName(ctx, diskName)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return nil, status.Errorf(codes.NotFound, "volume not found")
		} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
			return nil, status.Errorf(codes.Unavailable, "multiple VDIs found with same name, likely migration in progress, please try again later")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
		}
		vdi = foundVdi
	case UUIDAsVolumeID:
		vdiUUID := volumeIDValue
		foundVdi, err := cs.xoaClient.GetVDIByUUID(ctx, vdiUUID)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return nil, status.Errorf(codes.NotFound, "volume not found")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
		}
		vdi = foundVdi
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID type")
	}

	if req.GetVolumeCapability() != nil {
		if !isValidCapability(req.GetVolumeCapability()) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid volume capability")
		}
	}

	if req.GetCapacityRange() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "capacity range is required")
	}

	metadata, err := EmbeddedVDIMetadataFromDescription(vdi.NameDescription)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get migration data from VDI description: %v", err)
	}
	switch m := metadata.(type) {
	case *StorageInfo:
		if yes, targetSRUUID := m.HasOngoingMigration(); yes {
			return nil, status.Errorf(codes.Unavailable, "VDI is being migrated to %s, we can not expand it", targetSRUUID)
		}
	default:
		return nil, status.Errorf(codes.Internal, "unhandled VDI metadata type: %T", m)
	}

	newCapacity := req.GetCapacityRange().GetRequiredBytes()

	err = cs.xoaClient.ResizeVDI(ctx, vdi.UUID, newCapacity)
	if errors.Is(err, xoa.ErrVDIInUse) {
		return nil, status.Errorf(codes.FailedPrecondition, "volume is in use")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize volume: %v", err)
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         newCapacity,
		NodeExpansionRequired: false, // TODO: We need to expand the disk on the node, however this is currently done in the NodeStageVolume method (i.e. during/after mount)
	}, nil
}

func (cs *ControllerService) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	_, ctx = LogAndExpandContext(ctx, "req", req)

	vidType, volumeIDValue, err := ParseVolumeID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	var vdi *xoa.VDI = nil
	switch vidType {
	case NameAsVolumeID:
		diskName := cs.diskNameGenerator.FromVolumeName(volumeIDValue)
		foundVdi, err := cs.xoaClient.GetVDIByName(ctx, diskName)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
		}
		vdi = foundVdi
	case UUIDAsVolumeID:
		vdiUUID := volumeIDValue
		foundVdi, err := cs.xoaClient.GetVDIByUUID(ctx, vdiUUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
		}
		vdi = foundVdi
	}

	metadata, err := EmbeddedVDIMetadataFromDescription(vdi.NameDescription)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get storage info from VDI description: %v", err)
	}

	var storageInfo *StorageInfo = nil
	switch m := metadata.(type) {
	case *StorageInfo:
		storageInfo = m
	case *DeletionCandidate:
		return nil, status.Errorf(codes.Internal, "VDI is a deletion candidate, but should not be")
	case *NoMetadata:
		return nil, status.Errorf(codes.Internal, "VDI has no metadata, but should have")
	}

	storageSelection := storageSelectionFromStorageInfo(storageInfo, vdi)
	csiTopology, err := storageSelection.getTopologyForVDI(vdi)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get topology: %v", err)
	}

	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           req.GetVolumeId(),
			CapacityBytes:      vdi.Size,
			AccessibleTopology: csiTopology,
		},
	}, nil
}

func (cs *ControllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_GET_VOLUME,
					},
				},
			},
		},
	}, nil
}

// createDisk: createDisk based on volumeName, and with capacity and storageSelection
// We create a disk with a temporary name as a first step.
// A disk creation can take a long time and we might reach the timeout.
// If this is the case, a new creation disk will be started, and we don't have a good way to know if there is one already underway.
// With this prefix, we will be able to see if there are any temporary disks and remove them.
func (cs *ControllerService) createDisk(ctx context.Context, storageSelection *storageSelection, volumeName string, capacity int64, topologyRequirement *csi.TopologyRequirement) (*xoa.VDI, error) {
	log, ctx := LogAndExpandContext(ctx, "action", "createDisk", "volumeName", volumeName, "capacity", capacity)

	cs.creationLock.CreationLock()
	defer cs.creationLock.CreationUnlock()

	var vdiUUID string

	diskName := cs.diskNameGenerator.FromVolumeName(volumeName)
	temporaryDiskName := cs.diskNameGenerator.TemporaryFromVolumeName(volumeName)

	foundTemporaryVDI, err := cs.xoaClient.GetVDIByName(ctx, temporaryDiskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		// volume does not exist
		// continue with creation
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		foundTemporaryVDI, err = cs.getOneDiskAndDeleteRest(ctx, temporaryDiskName)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get one disk and delete rest: %v", err)
		}
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	}

	pickedSR, err := storageSelection.pickSRForTopology(capacity, topologyRequirement)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to pick SR for disk: %v", err)
	}

	if foundTemporaryVDI == nil {
		vdicreatedUUID, err := cs.xoaClient.CreateVDI(ctx, temporaryDiskName, pickedSR.UUID, capacity)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create disk: %v", err)
		}
		vdiUUID = vdicreatedUUID
		log.Info("Successfully created disk", "diskName", temporaryDiskName, "vdiUUID", vdiUUID)
	} else {
		vdiUUID = foundTemporaryVDI.UUID
	}

	err = cs.xoaClient.EditVDI(ctx, vdiUUID, ptr.To(diskName), ptr.To(""))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to edit disk: %v", err)
	}

	createdVDI, err := cs.xoaClient.GetVDIByUUID(ctx, vdiUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get created VDI: %v", err)
	}
	return createdVDI, nil
}

func (cs *ControllerService) findDisk(ctx context.Context, volumeID string) (*xoa.VDI, *StorageInfo, error) {
	log, ctx := LogAndExpandContext(ctx, "action", "findDisk", "volumeID", volumeID)

	vidType, volumeIDValue, err := ParseVolumeID(volumeID)
	if err != nil {
		return nil, nil, err
	}

	var vdi *xoa.VDI = nil
	// var storageInfo *StorageInfo = nil
	switch vidType {
	case NameAsVolumeID:
		diskName := cs.diskNameGenerator.FromVolumeName(volumeIDValue)
		foundVdi, err := cs.xoaClient.GetVDIByName(ctx, diskName)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return nil, nil, status.Errorf(codes.NotFound, "VDI not found")
		} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
			// This may happen if the VDI is being migrated
			return nil, nil, status.Errorf(codes.Unavailable, "multiple VDIs found with same name, likely migration in progress, please try again later")
		} else if err != nil {
			return nil, nil, status.Errorf(codes.Internal, "failed to get VDI: %v", err)
		}
		vdi = foundVdi

	case UUIDAsVolumeID:
		vdiUUID := volumeIDValue
		foundVdi, err := cs.xoaClient.GetVDIByUUID(ctx, vdiUUID)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return nil, nil, status.Errorf(codes.NotFound, "VDI not found")
		} else if err != nil {
			return nil, nil, status.Errorf(codes.Internal, "failed to get VDI: %v", err)
		}
		vdi = foundVdi
	default:
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid volume ID type")
	}

	metadata, err := EmbeddedVDIMetadataFromDescription(vdi.NameDescription)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed to get migration data from VDI description: %v", err)
	}

	var storageInfo *StorageInfo
	switch m := metadata.(type) {
	case *StorageInfo:
		if yes, targetSRUUID := m.HasOngoingMigration(); yes {
			if targetSRUUID != vdi.SR {
				// Migration is still in progress, so we can't use the VDI.
				return nil, nil, status.Errorf(codes.Unavailable, "VDI is being migrated to %s", targetSRUUID)
			}

			// At this point only one VDI exists, and it is already on the right SR. So we can remove the migration description
			// and continue
			m.Migrating.EndMigration()
			err = cs.xoaClient.EditVDI(ctx, vdi.UUID, nil, ptr.To(m.ToVDIDescription()))
			if err != nil {
				return nil, nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
			}
			log.Info("VDI finished migration", "vdiUUID", vdi.UUID, "targetSRUUID", targetSRUUID)
		}
		storageInfo = m
	case *DeletionCandidate:
		// This should not happen. There is a chance this happens when the cleanup process runs and attaches a deletion candidate on the temporary disk,
		// but the CreateVolume call is called at the same time.
		// We need to give up
		return nil, nil, status.Errorf(codes.Internal, "VDI is a deletion candidate, but should not be")
	case *NoMetadata:
		return nil, nil, status.Errorf(codes.Internal, "VDI has no metadata, but should have")
	default:
		return nil, nil, status.Errorf(codes.Internal, "unhandled VDI metadata type: %T", m)
	}

	return vdi, storageInfo, nil
}

func publishContextFromVBD(vbd *xoa.VBD) map[string]string {
	return map[string]string{
		"device": vbd.Device,
		"vbd":    vbd.UUID,
	}
}

// checkDiskAttachment: Check if the VDI is already attached to the VM
// If it is, return the VBD
// If it is not, connect the VBD and return the VBD
// If the VBD is not attached after 10 seconds, return an error
func (cs *ControllerService) checkDiskAttachment(ctx context.Context, vdi *xoa.VDI, vm *xoa.VM) (*xoa.VBD, error) {
	log, ctx := LogAndExpandContext(ctx, "action", "checkDiskAttachment", "vdiUUID", vdi.UUID, "vmUUID", vm.UUID)

	vbds, err := cs.xoaClient.GetVBDsByVMAndVDI(ctx, vm.UUID, vdi.UUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get VBDs: %v", err)
	}

	var selectedVBD *xoa.VBD = nil
	if len(vbds) > 0 {
		for i, vbd := range vbds {
			if vbd.Attached {
				selectedVBD = &vbds[i]
				break
			}
		}

		if selectedVBD == nil {
			selectedVBD = &vbds[0]
		}
	}

	if selectedVBD != nil {
		if selectedVBD.Attached && selectedVBD.Device != "" {
			return selectedVBD, nil
		}

		log.Info("Connecting VBD", "vbdUUID", selectedVBD.UUID)
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		connectedVBD, err := cs.xoaClient.ConnectVBDAndWaitForDevice(ctxWithTimeout, selectedVBD.UUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to connect VBD: %v", err)
		}

		return connectedVBD, nil
	}

	return nil, nil
}

func (cs *ControllerService) migrateVDI(ctx context.Context, vdi *xoa.VDI, vm *xoa.VM, storageSelection *storageSelection) (*xoa.VDI, *xoa.SR, error) {
	log, ctx := LogAndExpandContext(ctx, "action", "migrateVDI", "vdiUUID", vdi.UUID, "vmUUID", vm.UUID)

	pickedSR, err := storageSelection.pickSRForHost(vm.Host, vdi.Size)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed to pick SR: %v", err)
	}

	vbds, err := cs.xoaClient.GetVBDsByVDI(ctx, vdi.UUID)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed to get VBDs: %v", err)
	}
	if len(vbds) > 0 {
		return nil, nil, status.Errorf(codes.Unavailable, "VDI is attached to at least one VM, we can not migrate it, while it is attached")
	}

	storageInfo := storageSelection.toStorageInfo()
	storageInfo.Migrating.StartMigration(pickedSR.UUID)
	err = cs.xoaClient.EditVDI(ctx, vdi.UUID, nil, ptr.To(storageInfo.ToVDIDescription()))
	if err != nil {
		log.Error(err, "failed to set VDI description")
		return nil, nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
	}

	log.Info("Migrating VDI", "vdiUUID", vdi.UUID, "targetSRUUID", pickedSR.UUID)
	newVdiUUID, err := cs.xoaClient.MigrateVDI(ctx, vdi.UUID, pickedSR.UUID)
	if err != nil {
		log.Error(err, "failed to migrate VDI")
		return nil, nil, status.Errorf(codes.Internal, "failed to migrate VDI: %v", err)
	}
	log.Info("VDI migrated", "vdiUUID", vdi.UUID, "targetSRUUID", pickedSR.UUID, "newVdiUUID", newVdiUUID)

	foundVdi, err := cs.xoaClient.GetVDIByUUID(ctx, newVdiUUID)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		return nil, nil, status.Errorf(codes.NotFound, "Migrated VDI not found")
	} else if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed to get VDI: %v", err)
	}

	// Remove the migration description from the new VDI
	storageInfo.Migrating.EndMigration()
	err = cs.xoaClient.EditVDI(ctx, foundVdi.UUID, nil, ptr.To(storageInfo.ToVDIDescription()))
	if err != nil {
		log.Error(err, "failed to set VDI description")
		return nil, nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
	}

	log.Info("VDI finished migration", "vdiUUID", foundVdi.UUID, "targetSRUUID", pickedSR.UUID)
	return foundVdi, pickedSR, nil
}

func (cs *ControllerService) getOneDiskAndDeleteRest(ctx context.Context, name string) (*xoa.VDI, error) {
	log, ctx := LogAndExpandContext(ctx, "action", "getOneDiskAndDeleteRest", "name", name)
	vdis, err := cs.xoaClient.GetVDIs(ctx, map[string]any{
		"name_label": name,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get disks: %v", err)
	}

	if len(vdis) == 0 {
		return nil, status.Errorf(codes.NotFound, "no disks found with name: %s", name)
	}

	selectedVdi := &vdis[0]

	for _, vdi := range vdis[1:] {
		err = cs.xoaClient.DeleteVDI(ctx, vdi.UUID)
		if err != nil {
			// Temporary disks will be cleaned up by the temp cleanup process anyway
			log.Error(err, "failed to delete disk")
		}
	}

	return selectedVdi, nil
}
