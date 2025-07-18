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
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"

	xoa "github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

type NodeMetadataGetter interface {
	GetNodeId() (string, error)
}

type DriverOptions struct {
	Endpoint           string
	Mode               Mode
	DriverNameOverride string
}

// Mode is the operating mode of the CSI driver.
type Mode string

const (
	// ControllerMode is the mode that only starts the controller service.
	ControllerMode Mode = "controller"
	// NodeMode is the mode that only starts the node service.
	NodeMode Mode = "node"
	// AllMode is the mode that starts both the controller and the node service.
	AllMode Mode = "all"
)

type Driver struct {
	driverName string

	options      *DriverOptions
	xoaClient    xoa.Client
	nodeMetadata NodeMetadataGetter
	mounter      Mounter

	server     *grpc.Server
	controller *ControllerService
	identity   *IdentityService
	node       *NodeService
}

func NewDriver(opts *DriverOptions, xoaClient xoa.Client, nodeMetadata NodeMetadataGetter, mounter Mounter) *Driver {
	name := driverName
	if opts.DriverNameOverride != "" {
		name = opts.DriverNameOverride
	}

	klog.InfoS("Driver", "name", name, "version", driverVersion)
	d := &Driver{
		driverName:   name,
		options:      opts,
		xoaClient:    xoaClient,
		nodeMetadata: nodeMetadata,
		mounter:      mounter,
	}
	d.identity = NewIdentityService(d)
	return d
}

func (d *Driver) Run() error {
	u, err := url.Parse(d.options.Endpoint)
	if err != nil {
		return fmt.Errorf("unable to parse address: %q", err)
	}

	grpcAddr := path.Join(u.Host, filepath.FromSlash(u.Path))
	if u.Host == "" {
		grpcAddr = filepath.FromSlash(u.Path)
	}

	// CSI plugins talk only over UNIX sockets currently
	if u.Scheme != "unix" {
		return fmt.Errorf("currently only unix domain sockets are supported, have: %s", u.Scheme)
	}

	// remove the socket if it's already there. This can happen if we
	// deploy a new version and the socket was created from the old running plugin.
	if err := os.Remove(grpcAddr); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unix domain socket file %s, error: %s", grpcAddr, err)
	}

	// Create gRPC server
	d.server = grpc.NewServer()
	csi.RegisterIdentityServer(d.server, d.identity)

	if d.options.Mode != AllMode && d.options.Mode != ControllerMode && d.options.Mode != NodeMode {
		return fmt.Errorf("unknown mode: %s", d.options.Mode)
	}

	if d.options.Mode == AllMode || d.options.Mode == ControllerMode {
		if d.xoaClient == nil {
			return fmt.Errorf("xoaClient is required for controller mode")
		}
		d.controller = NewControllerService(d, d.xoaClient)
		csi.RegisterControllerServer(d.server, d.controller)
	}

	if d.options.Mode == AllMode || d.options.Mode == NodeMode {
		if d.nodeMetadata == nil {
			return fmt.Errorf("nodeMetadata is required for node mode")
		}
		nodeID, err := d.nodeMetadata.GetNodeId()
		if err != nil {
			return fmt.Errorf("failed to get node ID: %v", err)
		}

		if d.mounter == nil {
			return fmt.Errorf("mounter is required for node mode")
		}

		d.node = NewNodeService(d, d.mounter, nodeID)
		csi.RegisterNodeServer(d.server, d.node)
	}

	grpcListener, err := net.Listen(u.Scheme, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	defer grpcListener.Close()

	// Start server
	klog.InfoS("Starting CSI driver server", "endpoint", d.options.Endpoint)
	return d.server.Serve(grpcListener)
}

func (d *Driver) Stop() {
	d.server.Stop()
}

func (d *Driver) GetName() string {
	return d.driverName
}

const (
	driverName    = "csi.xen-orchestra.marcsi.ch"
	driverVersion = "0.1.0"
)
