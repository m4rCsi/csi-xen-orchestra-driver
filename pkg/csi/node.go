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
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeService struct {
	csi.UnimplementedNodeServer
	driver       *Driver
	mounter      Mounter
	nodeMetadata NodeMetadataGetter
}

func NewNodeService(driver *Driver, mounter Mounter, nodeMetadata NodeMetadataGetter) *NodeService {
	return &NodeService{
		driver:       driver,
		nodeMetadata: nodeMetadata,
		mounter:      mounter,
	}
}

func (ns *NodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	log, _ := LogAndExpandContext(ctx, "req", req)

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume id is required")
	}

	target := req.GetStagingTargetPath()
	if target == "" {
		return nil, status.Errorf(codes.InvalidArgument, "staging target path is required")
	}

	device := req.GetPublishContext()["device"]
	if device == "" {
		return nil, status.Errorf(codes.InvalidArgument, "device is not set")
	}

	vbdUUID := req.GetPublishContext()["vbdUUID"]
	devicePath, err := ns.mounter.FindDevicePath(device, vbdUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get device path from device name: %v", err)
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume capability is required")
	}

	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capability")
	}

	mount := volCap.GetMount()
	if mount == nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume capability is not a mount")
	}

	fsType := mount.GetFsType()
	if fsType == "" {
		fsType = DefaultFsType
	}

	// Check if a device is already mounted at our target
	currentDevice, _, err := ns.mounter.GetDeviceNameFromMount(target)
	if err != nil {
		msg := fmt.Sprintf("failed to check if volume is already mounted: %v", err)
		return nil, status.Error(codes.Internal, msg)
	}

	log.V(4).Info("NodeStageVolume: checking if volume is already staged", "device", device, "currentDevice", currentDevice, "target", target)
	if currentDevice == device {
		log.V(2).Info("NodeStageVolume: volume already staged", "device", device, "target", target)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	// Format device if needed
	log.V(2).Info("Formatting and mounting device", "devicePath", devicePath, "target", target, "fsType", fsType)
	if err := ns.mounter.FormatAndMount(devicePath, target, fsType, []string{}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ensure filesystem: %v", err)
	}

	// Resize volume if needed
	// According to https://github.com/kubernetes/mount-utils/blob/master/resizefs_linux.go#L133-L136
	// it is not recommended to check if the volume needs resizing and let resize2fs/xfs_growfs do the check for us.
	// We might need to change this if we want to support other filesystems in the future.
	resized, err := ns.mounter.Resize(devicePath, target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize volume: %v", err)
	}
	if resized {
		log.V(2).Info("Volume resized", "devicePath", devicePath, "target", target)
	}

	log.V(2).Info("NodeStageVolume: successfully staged volume", "devicePath", devicePath, "target", target, "fstype", fsType)
	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *NodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	log, _ := LogAndExpandContext(ctx, "req", req)

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume id is required")
	}

	target := req.GetStagingTargetPath()
	if target == "" {
		return nil, status.Errorf(codes.InvalidArgument, "staging target path is required")
	}

	device, refCount, err := ns.mounter.GetDeviceNameFromMount(target)
	if err != nil {
		msg := fmt.Sprintf("failed to check if target %q is a mount point: %v", target, err)
		return nil, status.Error(codes.Internal, msg)
	}

	if refCount == 0 {
		log.V(2).Info("NodeUnstageVolume: target not mounted", "target", target)
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	if refCount > 1 {
		log.Info("NodeUnstageVolume: found references to device mounted at target path", "refCount", refCount, "device", device, "target", target)
	}

	err = ns.mounter.Unmount(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount device: %v", err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *NodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume id is required")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume capability is required")
	}

	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capability")
	}

	mount := volCap.GetMount()
	if mount == nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume capability is not a mount")
	}

	source := req.GetStagingTargetPath()
	target := req.GetTargetPath()

	if target == "" {
		return nil, status.Errorf(codes.InvalidArgument, "target path is required")
	}

	fsType := mount.GetFsType()
	if fsType == "" {
		fsType = DefaultFsType
	}

	err := os.MkdirAll(target, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, status.Errorf(codes.Internal, "failed to ensure target path: %v", err)
	}

	if err := ns.mounter.Mount(source, target, fsType, []string{"bind"}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount device: %v", err)
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *NodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if req.GetTargetPath() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "target path is required")
	}

	err := ns.mounter.Unmount(req.GetTargetPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount device: %v", err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *NodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	nodeMetadata, err := ns.nodeMetadata.GetNodeMetadata()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get node id: %v", err)
	}

	segments := map[string]string{}
	if nodeMetadata.HostId != "" {
		segments["host"] = nodeMetadata.HostId
	}

	if nodeMetadata.PoolId != "" {
		segments["pool"] = nodeMetadata.PoolId
	}

	return &csi.NodeGetInfoResponse{
		NodeId: nodeMetadata.NodeId,
		AccessibleTopology: &csi.Topology{
			Segments: segments,
		},
	}, nil
}

func (ns *NodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
		},
	}, nil
}
