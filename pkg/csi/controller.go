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

	"github.com/container-storage-interface/spec/lib/go/csi"
	xoa "github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
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

func (cs *ControllerService) cleanupAllDisks(ctx context.Context, name string) error {
	vdis, err := cs.xoaClient.GetVDIs(ctx, map[string]any{
		"name_label": name,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get disks: %v", err)
	}

	for _, vdi := range vdis {
		err = cs.xoaClient.DeleteVDI(ctx, vdi.UUID)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to delete disk: %v", err)
		}
	}

	return nil
}

func (cs *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(2).InfoS("CreateVolume: called with args", "req", req)

	volumeName := req.GetName()

	if volumeName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "disk name is required")
	}

	if req.VolumeContentSource != nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume content source is not supported")
	}

	storageParams, err := LoadStorageParameters(req.GetParameters())
	if errors.Is(err, ErrInvalidStorageParameters) {
		// TODO: find out what the right way to deal with this is.
		// Should this be an error (i.e. if the parameters are invalid, or should we degrade gracefully?)
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
		capacity = 1024 * 1024 * 1024 // 1GB
	}

	storageSelection := storageSelectionFromParameters(storageParams)
	err = storageSelection.findSRs(ctx, cs.xoaClient)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find SRs: %v", err)
	}

	var vdi *xoa.VDI // the vdi object

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
	klog.V(2).InfoS("DeleteVolume: called with args", "req", req)

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
	klog.V(2).InfoS("ControllerPublishVolume: called with args", "req", req)

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
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find SRs: %v", err)
	}

	klog.Infof("VDI %s is not attached to VM %s", vdi.UUID, vmUUID)

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
	vbd, err = cs.xoaClient.AttachVDIAndWaitForDevice(ctx, vm.UUID, vdi.UUID, "RW")
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
	klog.V(2).InfoS("ControllerUnpublishVolume: called with args", "req", req)

	vidType, volumeIDValue, err := ParseVolumeID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	vmUUID := req.GetNodeId()
	if vmUUID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "node ID is required")
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
			return nil, status.Errorf(codes.Internal, "multiple VDIs found with same name")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get VDI: %v", err)
		}
		vdiUUID = vdi.UUID
	case UUIDAsVolumeID:
		vdiUUID = volumeIDValue
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID type")
	}

	vbds, err := cs.xoaClient.GetVBDsByVMAndVDI(ctx, vmUUID, vdiUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get VBDs: %v", err)
	}

	for _, vbd := range vbds {
		klog.V(2).InfoS("Deleting VBD", "vbd", vbd)
		err := cs.xoaClient.DeleteVBD(ctx, vbd.UUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to delete VBD: %v", err)
		}
	}
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (cs *ControllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(2).InfoS("ValidateVolumeCapabilities: called with args", "req", req)

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
			return nil, status.Errorf(codes.Internal, "multiple VDIs found with same name")
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
	klog.V(2).InfoS("ControllerExpandVolume: called with args", "req", req)

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
			return nil, status.Errorf(codes.Internal, "multiple VDIs found with same name")
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
	klog.V(2).InfoS("ControllerGetVolume: called with args", "req", req)

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
	klog.V(2).InfoS("ControllerGetCapabilities: called with args", "req", req)

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
func (cs *ControllerService) createDisk(ctx context.Context, storageSelection *storageSelection, volumeName string, capacity int64, topologyRequirement *csi.TopologyRequirement) (*xoa.VDI, error) {
	cs.creationLock.CreationLock()
	defer cs.creationLock.CreationUnlock()

	// We create a disk with a temporary name as a first step.
	// A disk creation can take a long time and we might reach the timeout.
	// If this is the case, a new creation disk will be started, and we don't have a good way to know if there is one already underway.
	// With this prefix, we will be able to see if there are any temporary disks and remove them.
	// TODO: Implement a cleanup mechanism to remove temporary disks.
	var vdiUUID string

	diskName := cs.diskNameGenerator.FromVolumeName(volumeName)
	temporaryDiskName := cs.diskNameGenerator.TemporaryFromVolumeName(volumeName)

	foundTemporaryVDI, err := cs.xoaClient.GetVDIByName(ctx, temporaryDiskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		// volume does not exist
		// continue with creation
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		// TODO, just pick up the first one and continue
		err = cs.cleanupAllDisks(ctx, temporaryDiskName)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "multiple disks found with same name, but failed to cleanup temporary disks: %v", err)
		}
		return nil, status.Errorf(codes.Unavailable, "multiple disks found with same name, but failed to cleanup temporary disks")
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
		klog.Infof("Successfully created disk '%s' with UUID: %s", temporaryDiskName, vdiUUID)
	} else {
		vdiUUID = foundTemporaryVDI.UUID
	}

	err = cs.xoaClient.EditVDI(ctx, vdiUUID, ptr.To(diskName), ptr.To(""))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to edit disk: %v", err)
	}

	// TODO: Refactor, we should not need to look it up again.
	createdVDI, err := cs.xoaClient.GetVDIByUUID(ctx, vdiUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get created VDI: %v", err)
	}
	return createdVDI, nil
}

func (cs *ControllerService) findDisk(ctx context.Context, volumeID string) (*xoa.VDI, *StorageInfo, error) {
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
			return nil, nil, status.Errorf(codes.Unavailable, "multiple VDIs found with same name (temporary error)")
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

	var storageInfo *StorageInfo = nil
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
			klog.Infof("VDI %s finished migration to %s", vdi.UUID, targetSRUUID)
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

func (cs *ControllerService) checkDiskAttachment(ctx context.Context, vdi *xoa.VDI, vm *xoa.VM) (*xoa.VBD, error) {
	// Check if the VDI is already attached to the VM
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

		klog.Infof("Connecting VBD %s", selectedVBD.UUID)
		connectedVBD, err := cs.xoaClient.ConnectVBDAndWaitForDevice(ctx, selectedVBD.UUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to connect VBD: %v", err)
		}

		return connectedVBD, nil
	}

	return nil, nil
}

func (cs *ControllerService) migrateVDI(ctx context.Context, vdi *xoa.VDI, vm *xoa.VM, storageSelection *storageSelection) (*xoa.VDI, *xoa.SR, error) {
	pickedSR, err := storageSelection.pickSRForHost(vm.Host, vdi.Size)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed to pick SR: %v", err)
	}

	storageInfo := storageSelection.toStorageInfo()
	storageInfo.Migrating.StartMigration(pickedSR.UUID)
	err = cs.xoaClient.EditVDI(ctx, vdi.UUID, nil, ptr.To(storageInfo.ToVDIDescription()))
	if err != nil {
		klog.Errorf("failed to set VDI description: %v", err)
		return nil, nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
	}

	klog.Infof("Migrating VDI %s to SR %s", vdi.UUID, pickedSR.UUID)
	newVdiUUID, err := cs.xoaClient.MigrateVDI(ctx, vdi.UUID, pickedSR.UUID)
	if err != nil {
		klog.Errorf("failed to migrate VDI: %v", err)
		return nil, nil, status.Errorf(codes.Internal, "failed to migrate VDI: %v", err)
	}
	klog.Infof("VDI %s migrated to SR %s (new VDI UUID: %s)", vdi.UUID, pickedSR.UUID, newVdiUUID)

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
		klog.Errorf("failed to set VDI description: %v", err)
		return nil, nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
	}

	klog.Infof("VDI %s finished migration to %s", foundVdi.UUID, pickedSR.UUID)
	return foundVdi, pickedSR, nil
}
