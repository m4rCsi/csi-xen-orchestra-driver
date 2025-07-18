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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/m4rCsi/csi-xen-orchestra-driver/pkg/csi"
	"github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	"k8s.io/klog/v2"
)

var (
	endpoint     = flag.String("endpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	controller   = flag.Bool("controller", false, "Run as controller service")
	node         = flag.Bool("node", false, "Run as node service")
	nameOverride = flag.String("driver-name-override", "", "Driver name override")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	var mode csi.Mode
	if *controller && !*node {
		mode = csi.ControllerMode
	} else if *node && !*controller {
		mode = csi.NodeMode
	} else if *controller && *node {
		mode = csi.AllMode
	} else {
		klog.Fatal("Either --controller=true or --node=true must be specified")
	}

	// Optional dependencies, depending on mode
	var xoaClient xoa.Client = nil
	var nodeMetadata NodeMetadataGetter = nil
	var mounter csi.Mounter = nil

	if mode == csi.ControllerMode || mode == csi.AllMode {
		xoaToken := os.Getenv("XOA_TOKEN")
		xoaURL := os.Getenv("XOA_URL")
		if xoaURL == "" || xoaToken == "" {
			klog.Fatal("XOA_URL and XOA_TOKEN are required")
		}
		xc, err := xoa.NewJSONRPCClient(xoa.ClientConfig{
			BaseURL: xoaURL,
			Token:   xoaToken,
		})
		if err != nil {
			klog.Fatalf("failed to create XOA API client: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := xc.Connect(ctx); err != nil {
			klog.Fatalf("failed to connect to XOA API: %v", err)
		}
		xoaClient = xc
		defer xoaClient.Close()
	}
	if mode == csi.NodeMode || mode == csi.AllMode {
		nodeMetadata = NewNodeMetadata()
		mounter = csi.NewSafeMounter()
	}

	driver := csi.NewDriver(
		&csi.DriverOptions{
			Endpoint:           *endpoint,
			Mode:               mode,
			DriverNameOverride: *nameOverride,
		},
		xoaClient,
		nodeMetadata,
		mounter,
	)

	// Start server
	go func() {
		klog.InfoS("Starting CSI driver server", "endpoint", *endpoint, "mode", mode)
		if err := driver.Run(); err != nil {
			klog.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	klog.Info("Shutting down CSI driver server...")
	driver.Stop()
}
