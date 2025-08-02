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
	"fmt"

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
}

func NewControllerService(driver *Driver, xoaClient xoa.Client, diskNameGenerator *DiskNameGenerator) *ControllerService {
	return &ControllerService{
		driver:            driver,
		xoaClient:         xoaClient,
		diskNameGenerator: diskNameGenerator,
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
	diskName := cs.diskNameGenerator.FromVolumeName(volumeName)
	temporaryDiskName := cs.diskNameGenerator.TemporaryFromVolumeName(volumeName)

	if req.VolumeContentSource != nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume content source is not supported")
	}

	storageParams, err := LoadStorageParameters(req.GetParameters())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to load storage parameters: %v", err)
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

	var vdiUUID string
	vdi, err := cs.xoaClient.GetVDIByName(ctx, diskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		// volume does not exist
		// continue with creation
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		return nil, status.Errorf(codes.AlreadyExists, "multiple disks found with same name already exist")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	} else {
		// volume exists, check if size matches
		if vdi.Size != capacity {
			return nil, status.Errorf(codes.AlreadyExists, "volume already exists but size mismatch")
		}
		vdiUUID = vdi.UUID
	}

	if vdiUUID == "" {
		// Create: Volume does not exist, we need to create it

		// We create a disk with a temporary name as a first step.
		// A disk creation can take a long time and we might reach the timeout.
		// If this is the case, a new creation disk will be started, and we don't have a good way to know if there is one already underway.
		// With this prefix, we will be able to see if there are any temporary disks and remove them.
		// TODO: Implement a cleanup mechanism to remove temporary disks.

		foundTemporaryVDI, err := cs.xoaClient.GetVDIByName(ctx, temporaryDiskName)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			// volume does not exist
			// continue with creation
		} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
			err = cs.cleanupAllDisks(ctx, temporaryDiskName)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "multiple disks found with same name, but failed to cleanup temporary disks: %v", err)
			}
			return nil, status.Errorf(codes.Unavailable, "multiple disks found with same name, but failed to cleanup temporary disks")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
		}

		var pickedSRUUID string = ""
		srSelectionType, srSelectionValue, err := storageParams.getSRSelection()
		switch srSelectionType {
		case StorageRepositorySelectionTag:
			localSrs, err := cs.getSRsWithTag(ctx, srSelectionValue)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get local SRs: %v", err)
			}
			if len(localSrs) == 0 {
				return nil, status.Errorf(codes.Internal, "no local SRs found with tag: %s", srSelectionValue)
			}
			bestSR, err := pickBestSRForDisk(localSrs, capacity)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to pick SR for disk: %v", err)
			}
			pickedSRUUID = bestSR.UUID
		case StorageRepositorySelectionUUID:
			pickedSRUUID = srSelectionValue
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get selected SR UUIDs: %v", err)
		}

		if foundTemporaryVDI == nil {
			vdicreated, err := cs.xoaClient.CreateVDI(ctx, temporaryDiskName, pickedSRUUID, capacity)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create disk: %v", err)
			}
			vdiUUID = vdicreated
			klog.Infof("Successfully created disk '%s' with UUID: %s", temporaryDiskName, vdiUUID)
		} else {
			vdiUUID = foundTemporaryVDI.UUID
		}

		err = cs.xoaClient.EditVDI(ctx, vdiUUID, ptr.To(diskName), ptr.To(""))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to edit disk: %v", err)
		}
	}

	var volumeID string
	switch storageParams.VolumeIDType() {
	case NameAsVolumeID:
		volumeID = CreateVolumeIDWithName(volumeName)
	case UUIDAsVolumeID:
		volumeID = CreateVolumeIDWithUUID(vdiUUID)
	default:
		return nil, status.Errorf(codes.Internal, "invalid volume ID type")
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeID,
			CapacityBytes: capacity,
			VolumeContext: storageParams.GenerateVolumeContext(),
		},
	}, nil
}

func (cs *ControllerService) getSRsWithTag(ctx context.Context, tag string) ([]xoa.SR, error) {
	srs, err := cs.xoaClient.GetSRsWithTag(ctx, tag)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get SRs with tag: %v", err)
	}

	return srs, nil
}

func filterSrsForHost(srs []xoa.SR, host string) []xoa.SR {
	filteredSrs := make([]xoa.SR, 0)
	for _, sr := range srs {
		if sr.Host == host {
			filteredSrs = append(filteredSrs, sr)
		}
	}
	return filteredSrs
}

func pickBestSRForDisk(srs []xoa.SR, capacity int64) (*xoa.SR, error) {
	// Then pick the SR with the most free space
	bestSR := srs[0]
	for _, sr := range srs {
		if sr.Usage < bestSR.Usage {
			bestSR = sr
		}
	}

	// Check if the best SR has enough space for the disk
	if bestSR.Usage+capacity > bestSR.Size {
		return nil, fmt.Errorf("no SR has enough space for the disk: %s", bestSR.UUID)
	}

	return &bestSR, nil
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

	vidType, volumeIDValue, err := ParseVolumeID(req.GetVolumeId())
	if err != nil {
		return nil, err
	}

	vm, err := cs.xoaClient.GetVMByUUID(ctx, vmUUID)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		return nil, status.Errorf(codes.NotFound, "VM not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get VM: %v", err)
	}

	var vdi *xoa.VDI = nil
	switch vidType {
	case NameAsVolumeID:
		diskName := cs.diskNameGenerator.FromVolumeName(volumeIDValue)
		foundVdi, err := cs.xoaClient.GetVDIByName(ctx, diskName)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return nil, status.Errorf(codes.NotFound, "VDI not found")
		} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
			// This may happen if the VDI is being migrated
			return nil, status.Errorf(codes.Unavailable, "multiple VDIs found with same name (temporary error)")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get VDI: %v", err)
		}

		metadata, err := EmbeddedVDIMetadataFromDescription(foundVdi.NameDescription)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get migration data from VDI description: %v", err)
		}
		switch m := metadata.(type) {
		case *Migration:
			if m.TargetSRUUID() != foundVdi.SR {
				// Migration is still in progress, so we can't use the VDI.
				return nil, status.Errorf(codes.Unavailable, "VDI is being migrated to %s", m.TargetSRUUID())
			}

			// At this point only one VDI exists, and it is already on the right SR. So we can remove the migration description.
			err = cs.xoaClient.EditVDI(ctx, foundVdi.UUID, nil, ptr.To(""))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
			}

			klog.Infof("VDI %s finished migration to %s", foundVdi.UUID, m.TargetSRUUID())
			vdi = foundVdi
		case *DeletionCandidate:
			// We can safely ignore/remove this deletion candidate.
			// This can happen when the Cleanup runs and attaches a deletion candidate on the temporary disk, but it is renamed to the final disk name,
			// through the CreateVolume call at the same time.
			err = cs.xoaClient.EditVDI(ctx, foundVdi.UUID, nil, ptr.To(""))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
			}
			vdi = foundVdi
		case *NoMetadata:
			// No metadata, so we can use the VDI.
			// This is the case for a newly created VDI.
			vdi = foundVdi
		default:
			return nil, status.Errorf(codes.Internal, "unhandled VDI metadata type: %T", m)
		}

	case UUIDAsVolumeID:
		vdiUUID := volumeIDValue
		foundVdi, err := cs.xoaClient.GetVDIByUUID(ctx, vdiUUID)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return nil, status.Errorf(codes.NotFound, "VDI not found")
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get VDI: %v", err)
		}

		vdi = foundVdi
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID type")
	}

	// Check if the VDI is already attached to the VM
	vbds, err := cs.xoaClient.GetVBDsByVMAndVDI(ctx, vmUUID, vdi.UUID)
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
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					"device": selectedVBD.Device,
					"vbd":    selectedVBD.UUID,
				},
			}, nil
		}

		klog.Infof("Connecting VBD %s", selectedVBD.UUID)
		connectedVBD, err := cs.xoaClient.ConnectVBDAndWaitForDevice(ctx, selectedVBD.UUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to connect VBD: %v", err)
		}

		return &csi.ControllerPublishVolumeResponse{
			PublishContext: map[string]string{
				"device": connectedVBD.Device,
				"vbd":    connectedVBD.UUID,
			},
		}, nil
	}

	klog.Infof("VDI %s is not attached to VM %s", vdi.UUID, vmUUID)

	storageParams, err := LoadStorageParametersFromVolumeContext(req.GetVolumeContext())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to load storage parameters: %v", err)
	}

	switch storageParams.Type {
	case StorageTypeStatic:
		sr, err := cs.xoaClient.GetSRByUUID(ctx, vdi.SR)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get SR: %v", err)
		}

		// TODO: We need to implement topology
		if sr.Shared {
			// If we are shared, we need to check if the VM is in the same pool as the SR
			if vm.Pool != sr.Pool {
				return nil, status.Errorf(codes.FailedPrecondition, "VDI is not on the same pool as the VM")
			}
		} else {
			// If we are not shared, we need to check if the VM is on the same host as the SR
			if vm.Host != sr.Host {
				return nil, status.Errorf(codes.FailedPrecondition, "VDI is not on the same host as the VM")
			}
		}
	case StorageTypeShared:
		// Nothing to do here, we can use the VDI directly
	case StorageTypeLocalMigrating:
		// Figure out which SR to use for the host
		srSelectionType, srSelectionValue, err := storageParams.getSRSelection()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get SR selection: %v", err)
		}
		if srSelectionType != StorageRepositorySelectionTag {
			return nil, status.Errorf(codes.InvalidArgument, "invalid SR selection type: %s", srSelectionType)
		}
		localSrs, err := cs.getSRsWithTag(ctx, srSelectionValue)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get local SRs: %v", err)
		}

		localSrs = filterSrsForHost(localSrs, vm.Host)
		pickedSR, err := pickBestSRForDisk(localSrs, vdi.Size)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to pick SR for disk: %v", err)
		}

		klog.Infof("SR found for host %s: %+v", vm.Host, pickedSR)

		if vdi.SR == pickedSR.UUID {
			klog.Infof("VDI %s is already on SR %s", vdi.UUID, pickedSR.UUID)
		} else {
			migration := NewMigration(pickedSR.UUID)
			err := cs.xoaClient.EditVDI(ctx, vdi.UUID, nil, ptr.To(migration.ToVDIDescription()))
			if err != nil {
				klog.Errorf("failed to set VDI description: %v", err)
				return nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
			}

			klog.Infof("Migrating VDI %s to SR %s", vdi.UUID, pickedSR.UUID)
			newVdiUUID, err := cs.xoaClient.MigrateVDI(ctx, vdi.UUID, pickedSR.UUID)
			if err != nil {
				klog.Errorf("failed to migrate VDI: %v", err)
				return nil, status.Errorf(codes.Internal, "failed to migrate VDI: %v", err)
			}
			klog.Infof("VDI %s migrated to SR %s (new VDI UUID: %s)", vdi.UUID, pickedSR.UUID, newVdiUUID)

			foundVdi, err := cs.xoaClient.GetVDIByUUID(ctx, newVdiUUID)
			if errors.Is(err, xoa.ErrObjectNotFound) {
				return nil, status.Errorf(codes.NotFound, "Migrated VDI not found")
			} else if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get VDI: %v", err)
			}

			// Remove the migration description from the new VDI
			err = cs.xoaClient.EditVDI(ctx, foundVdi.UUID, nil, ptr.To(""))
			if err != nil {
				klog.Errorf("failed to set VDI description: %v", err)
				return nil, status.Errorf(codes.Internal, "failed to set VDI description: %v", err)
			}

			klog.Infof("VDI %s finished migration to %s", foundVdi.UUID, pickedSR.UUID)

			// Replace the VDI with the new one
			vdi = foundVdi
		}
	}

	// Attach the VDI to the VM in RW mode
	vbd, err := cs.xoaClient.AttachVDIAndWaitForDevice(ctx, vmUUID, vdi.UUID, "RW")
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
		},
	}, nil
}
