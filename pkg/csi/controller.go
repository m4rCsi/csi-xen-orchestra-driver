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
)

type ControllerService struct {
	csi.UnimplementedControllerServer
	driver    *Driver
	xoaClient xoa.Client
}

func NewControllerService(driver *Driver, xoaClient xoa.Client) *ControllerService {
	return &ControllerService{
		driver:    driver,
		xoaClient: xoaClient,
	}
}

func (cs *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(2).InfoS("CreateVolume: called with args", "req", req)
	diskName := req.GetName()

	if diskName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "disk name is required")
	}

	if req.VolumeContentSource != nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume content source is not supported")
	}

	parameters := req.GetParameters()
	srUUID := parameters["srUUID"]
	if srUUID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "srUUID is required")
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
		return nil, status.Errorf(codes.AlreadyExists, "multiple objects found with same name already exist")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	} else {
		// volume exists, check if size matches
		vdiUUID = vdi.UUID

		if vdi.Size != capacity {
			return nil, status.Errorf(codes.AlreadyExists, "volume already exists but size mismatch")
		}
	}

	if vdi == nil {
		vdicreated, err := cs.xoaClient.CreateVDI(ctx, diskName, srUUID, capacity)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create disk: %v", err)
		}
		vdiUUID = vdicreated
		klog.Infof("Successfully created disk '%s' with UUID: %s", diskName, vdiUUID)
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vdiUUID,
			CapacityBytes: capacity,
		},
	}, nil
}

func (cs *ControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(2).InfoS("DeleteVolume: called with args", "req", req)

	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume ID is required")
	}

	// TODO: Probably can just straight up call the DeleteVDI method and ignore error if already deleted
	_, err := cs.xoaClient.GetVDIByUUID(ctx, volumeId)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		// volume already deleted
		return &csi.DeleteVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	}

	err = cs.xoaClient.DeleteVDI(ctx, volumeId)
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
	vdiUUID := req.GetVolumeId()

	if vmUUID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "node ID is required")
	}
	if vdiUUID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume ID is required")
	}

	if !isValidCapability(req.GetVolumeCapability()) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capability")
	}

	// Check if the VDI is already attached to the VM
	vbds, err := cs.xoaClient.GetVBDsByVMAndVDI(ctx, vmUUID, vdiUUID)
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
		if selectedVBD.Attached {
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					"device": selectedVBD.Device,
					"vbd":    selectedVBD.UUID,
				},
			}, nil
		}

		err = cs.xoaClient.ConnectVBD(ctx, selectedVBD.UUID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to connect VBD: %v", err)
		}

		return &csi.ControllerPublishVolumeResponse{
			PublishContext: map[string]string{
				"device": selectedVBD.Device,
				"vbd":    selectedVBD.UUID,
			},
		}, nil
	}

	// Attach the VDI to the VM in RW mode
	vbd, err := cs.xoaClient.AttachVDIAndWaitForDevice(ctx, vmUUID, vdiUUID, "RW")
	if err != nil {
		if errors.Is(err, xoa.ErrNoSuchObject) {
			return nil, status.Errorf(codes.NotFound, "not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to attach disk: %v", err)
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

	vmUUID := req.GetNodeId()
	vdiUUID := req.GetVolumeId()

	if vmUUID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "node ID is required")
	}
	if vdiUUID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume ID is required")
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

	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume ID is required")
	}

	_, err := cs.xoaClient.GetVDIByUUID(ctx, volumeId)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		return nil, status.Errorf(codes.NotFound, "volume not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	}

	volumeCapabilities := req.GetVolumeCapabilities()
	if len(volumeCapabilities) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "volume capabilities are required")
	}

	if !isValidVolumeCapabilities(volumeCapabilities) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capabilities")
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(volumeCapabilities) {
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volumeCapabilities}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
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
		},
	}, nil
}
