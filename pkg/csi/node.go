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
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

type NodeService struct {
	csi.UnimplementedNodeServer
	driver  *Driver
	nodeID  string
	mounter Mounter
}

func NewNodeService(driver *Driver, mounter Mounter, nodeID string) *NodeService {
	return &NodeService{
		driver:  driver,
		nodeID:  nodeID,
		mounter: mounter,
	}
}

func (ns *NodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(2).InfoS("NodeStageVolume: called with args", "req", req)
	device := "/dev/" + req.GetPublishContext()["device"]
	target := req.GetStagingTargetPath()

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume id is required")
	}

	if device == "" {
		return nil, status.Errorf(codes.InvalidArgument, "device is not set")
	}

	if target == "" {
		return nil, status.Errorf(codes.InvalidArgument, "staging target path is required")
	}

	if req.VolumeCapability == nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume capability is required")
	}

	// Format device if needed
	if err := ns.mounter.FormatAndMount(device, target, "ext4", []string{}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ensure filesystem: %v", err)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *NodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(2).InfoS("NodeUnstageVolume: called with args", "req", req)

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume id is required")
	}

	if req.GetStagingTargetPath() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "staging target path is required")
	}

	err := ns.mounter.Unmount(req.GetStagingTargetPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount device: %v", err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *NodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.V(2).InfoS("NodePublishVolume: called with args", "req", req)

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "volume id is required")
	}

	if req.VolumeCapability == nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume capability is required")
	}

	source := req.GetStagingTargetPath()
	target := req.GetTargetPath()

	if target == "" {
		return nil, status.Errorf(codes.InvalidArgument, "target path is required")
	}

	os.MkdirAll(target, 0755)

	if err := ns.mounter.Mount(source, target, "ext4", []string{"bind"}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount device: %v", err)
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *NodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(2).InfoS("NodeUnpublishVolume: called with args", "req", req)

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
	klog.V(2).InfoS("NodeGetInfo: called with args", "req", req)
	klog.V(2).InfoS("NodeGetInfo: node", "nodeID", ns.nodeID)
	return &csi.NodeGetInfoResponse{
		NodeId: ns.nodeID,
	}, nil
}

func (ns *NodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(2).InfoS("NodeGetCapabilities: called with args", "req", req)

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
