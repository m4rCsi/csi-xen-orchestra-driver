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

type NodeMetadata struct {
	NodeId string
	HostId string
	PoolId string
}

type NodeMetadataGetter interface {
	GetNodeMetadata() (*NodeMetadata, error)
}

type DriverOptions struct {
	Endpoint           string
	Mode               Mode
	DriverNameOverride string

	TempCleanup    bool
	DiskNamePrefix string
	HostTopology   bool
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

	server      *grpc.Server
	controller  *ControllerService
	identity    *IdentityService
	node        *NodeService
	tempCleanup *TempCleanup

	diskNameGenerator *DiskNameGenerator
	creationLock      *CreationLock
}

func NewDriver(opts *DriverOptions, xoaClient xoa.Client, nodeMetadata NodeMetadataGetter, mounter Mounter) *Driver {
	name := driverName
	if opts.DriverNameOverride != "" {
		name = opts.DriverNameOverride
	}

	klog.InfoS("Driver", "name", name, "version", driverVersion)
	d := &Driver{
		driverName:        name,
		options:           opts,
		xoaClient:         xoaClient,
		nodeMetadata:      nodeMetadata,
		mounter:           mounter,
		diskNameGenerator: NewDiskNameGenerator(opts.DiskNamePrefix),
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

	// log errors and expand the log context
	grpcInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		log := klog.FromContext(ctx).WithValues("method", info.FullMethod)
		ctx = klog.NewContext(ctx, log)
		log.V(2).Info(fmt.Sprintf("Method called: %s", info.FullMethod))

		resp, err := handler(ctx, req)
		if err != nil {
			log.Error(err, "method failed", "method", info.FullMethod)
		}
		return resp, err
	}

	// Create gRPC server
	d.server = grpc.NewServer(grpc.UnaryInterceptor(grpcInterceptor))
	csi.RegisterIdentityServer(d.server, d.identity)

	if d.options.Mode != AllMode && d.options.Mode != ControllerMode && d.options.Mode != NodeMode {
		return fmt.Errorf("unknown mode: %s", d.options.Mode)
	}

	if d.options.Mode == AllMode || d.options.Mode == ControllerMode {
		d.creationLock = NewCreationLock()

		if d.xoaClient == nil {
			return fmt.Errorf("xoaClient is required for controller mode")
		}
		klog.InfoS("Starting controller service")
		d.controller = NewControllerService(d, d.xoaClient, d.diskNameGenerator, d.creationLock, d.options.HostTopology)
		csi.RegisterControllerServer(d.server, d.controller)

		if d.options.TempCleanup {
			klog.InfoS("Starting temp cleanup service")
			d.tempCleanup = NewTempCleanup(d.xoaClient, d.diskNameGenerator, d.creationLock)
			go d.tempCleanup.Run()
		}
	}

	if d.options.Mode == AllMode || d.options.Mode == NodeMode {
		if d.nodeMetadata == nil {
			return fmt.Errorf("nodeMetadata is required for node mode")
		}

		if d.mounter == nil {
			return fmt.Errorf("mounter is required for node mode")
		}

		klog.InfoS("Starting node service")
		d.node = NewNodeService(d, d.mounter, d.nodeMetadata)
		csi.RegisterNodeServer(d.server, d.node)
	}

	grpcListener, err := net.Listen(u.Scheme, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	defer func() {
		if err := grpcListener.Close(); err != nil {
			klog.Errorf("failed to close grpc listener: %v", err)
		}
	}()

	// Start server
	klog.InfoS("Starting CSI driver server", "endpoint", d.options.Endpoint)
	return d.server.Serve(grpcListener)
}

func (d *Driver) Stop() {
	if d.tempCleanup != nil {
		d.tempCleanup.Stop()
	}
	d.server.Stop()
}

func (d *Driver) GetName() string {
	return d.driverName
}

const (
	driverName    = "csi.xen-orchestra.marcsi.ch"
	driverVersion = "0.1.0"
)
