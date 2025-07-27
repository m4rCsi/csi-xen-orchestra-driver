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
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	xoa "github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	"k8s.io/klog/v2"
)

func main() {
	flag.Usage = func() {
		fmt.Println("xoa-jsonrpc - Xen Orchestra Command Line Proof of Concept")
		fmt.Println("")
		fmt.Println("This is a command line tool to interact with Xen Orchestra.")
		fmt.Println("It is mainly meant to show and validate the connection to the xoa json-rpc endpoint.")
		fmt.Println("   https://docs.xcp-ng.org/management/manage-at-scale/xo-api/#-json-rpc-over-websockets")
		fmt.Println("")
		fmt.Println("It is not meant to be used in anger. Use xo-cli instead.")
		fmt.Println("   https://docs.xcp-ng.org/management/manage-at-scale/xo-cli/ ")
		fmt.Println("")
		fmt.Println("Usage: xoa-jsonrpc -command <command> [flags]")
		fmt.Println("")
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	// Parse command line flags
	var (
		baseURL = flag.String("url", "", "Xen Orchestra base URL [format: https://xo.example.local], (or XOA_URL environment variable)")
		token   = flag.String("token", "", "Authentication token, (or XOA_TOKEN environment variable)")

		// Command flags
		command  = flag.String("command", "", "Command to execute: list-disks, create-disk, attach-disk, detach-disk, check-attachment, resize-disk")
		diskName = flag.String("disk-name", "", "Name for the disk")
		srUUID   = flag.String("sr-uuid", "", "Storage Repository UUID")
		vmUUID   = flag.String("vm-uuid", "", "Virtual Machine UUID")
	)
	flag.Parse()

	if *command == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Validate required flags
	if *baseURL == "" {
		if *baseURL = os.Getenv("XOA_URL"); *baseURL == "" {
			log.Fatal("-url flag is required")
		}
	}
	if *token == "" {
		if *token = os.Getenv("XOA_TOKEN"); *token == "" {
			log.Fatal("-token flag is required")
		}
	}

	// Initialize klog
	klog.InitFlags(nil)
	flag.Parse()

	// Create client configuration
	config := xoa.ClientConfig{
		BaseURL: *baseURL,
		Token:   *token,
	}

	// Create Xen Orchestra WebSocket JSON-RPC client
	client, err := xoa.NewJSONRPCClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect to the WebSocket API
	if err := client.Connect(context.Background()); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Execute the requested command
	switch *command {
	case "create-disk":
		if *diskName == "" {
			log.Fatal("-disk-name flag is required for create-disk command")
		}
		if *srUUID == "" {
			log.Fatal("-sr-uuid flag is required for create-disk command")
		}
		createDisk(client, *diskName, *srUUID)
	case "delete-disk":
		if *diskName == "" {
			log.Fatal("-disk-name flag is required for delete-disk command")
		}
		deleteDisk(client, *diskName)
	case "attach-disk":
		if *diskName == "" {
			log.Fatal("-disk-name flag is required for attach-disk command")
		}
		if *vmUUID == "" {
			log.Fatal("-vm-uuid flag is required for attach-disk command")
		}
		attachDisk(client, *diskName, *vmUUID)

	case "detach-disk":
		if *diskName == "" {
			log.Fatal("-disk-name flag is required for detach-disk command")
		}
		if *vmUUID == "" {
			log.Fatal("-vm-uuid flag is required for detach-disk command")
		}
		detachDisk(client, *diskName, *vmUUID)

	case "check-attachment":
		if *diskName == "" {
			log.Fatal("-disk-name flag is required for check-attachment command")
		}
		if *vmUUID == "" {
			log.Fatal("-vm-uuid flag is required for detach-disk command")
		}
		checkDiskAttachment(client, *diskName, *vmUUID)

	case "resize-disk":
		if *diskName == "" {
			log.Fatal("-disk-name flag is required for resize-disk command")
		}
		newSize := int64(1024 * 1024 * 1024 * 5) // 5GB
		resizeDisk(client, *diskName, newSize)

	case "migrate-disk":
		if *diskName == "" {
			log.Fatal("-disk-name flag is required for migrate-disk command")
		}
		if *srUUID == "" {
			log.Fatal("-sr-uuid flag is required for migrate-disk command")
		}
		migrateDisk(client, *diskName, *srUUID)

	case "list-disks":
		listDisks(client, diskName)

	default:
		log.Fatalf("Unknown command: %s. Valid commands are: create-disk, attach-disk, detach-disk, check-attachment, resize-disk, list-disks", *command)
	}
}

func listDisks(client xoa.Client, diskName *string) {
	ctx := context.Background()
	filter := make(map[string]interface{})
	if diskName != nil && *diskName != "" {
		filter["name_label"] = *diskName
	}

	vdis, err := client.GetVDIs(ctx, filter)
	if err != nil {
		log.Fatalf("Failed to get VDIs: %v", err)
	}

	for _, vdi := range vdis {
		fmt.Printf("VDI: %+v\n", vdi)
	}
}

// createDisk creates a new disk with the specified name and 1GB size
func createDisk(client xoa.Client, diskName, srUUID string) {
	ctx := context.Background()
	fmt.Printf("Creating disk '%s' with 1GB size in SR %s...\n", diskName, srUUID)

	// 1GB in bytes
	size := int64(1024 * 1024 * 1024)

	vdiUUID, err := client.CreateVDI(ctx, diskName, srUUID, size)
	if err != nil {
		log.Fatalf("Failed to create disk: %v", err)
	}

	fmt.Printf("Successfully created disk '%s' with UUID: %s\n", diskName, vdiUUID)
}

// deleteDisk deletes a disk
func deleteDisk(client xoa.Client, diskName string) {
	ctx := context.Background()
	vdi, err := client.GetVDIByName(ctx, diskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		fmt.Printf("Disk '%s' not found\n", diskName)
		os.Exit(0)
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		fmt.Printf("Multiple disks found with name '%s'\n", diskName)
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("Failed to get VDIs: %v", err)
	}

	fmt.Printf("Deleting disk '%s'...\n", diskName)

	err = client.DeleteVDI(ctx, vdi.UUID)
	if errors.Is(err, xoa.ErrNoSuchObject) {
		fmt.Printf("Disk '%s' not found\n", diskName)
		os.Exit(0)
	} else if err != nil {
		log.Fatalf("Failed to delete VDI: %v", err)
	}

	fmt.Printf("Successfully deleted disk '%s'\n", diskName)
}

// attachDisk attaches a disk to a VM
func attachDisk(client xoa.Client, diskName, vmUUID string) {
	ctx := context.Background()

	// Find the VDI by name
	vdi, err := client.GetVDIByName(ctx, diskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		fmt.Printf("Disk '%s' not found\n", diskName)
		os.Exit(0)
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		fmt.Printf("Multiple disks found with name '%s'\n", diskName)
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("Failed to get VDIs: %v", err)
	}

	// Check if the VDI is already attached to the VM
	vbds, err := client.GetVBDsByVMAndVDI(ctx, vmUUID, vdi.UUID)
	if err != nil {
		log.Fatalf("Failed to get VBDs: %v", err)
	}

	if len(vbds) > 0 {
		attached := false
		for _, vbd := range vbds {
			if vbd.Attached {
				attached = true
				break
			}
		}

		if attached {
			fmt.Printf("Disk '%s' is already attached to VM %s\n", diskName, vmUUID)
			os.Exit(0)
		}

		fmt.Printf("Attachment found, connecting VBD...\n")

		err = client.ConnectVBD(ctx, vbds[0].UUID)
		if err != nil {
			log.Fatalf("Failed to connect VBD: %v", err)
		}

		fmt.Printf("Successfully attached disk '%s' to VM %s\n", diskName, vmUUID)
		os.Exit(0)
	}

	fmt.Printf("Attaching and connecting disk '%s' to VM %s...\n", diskName, vmUUID)

	// Attach the VDI to the VM in RW mode
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	vbd, err := client.AttachVDIAndWaitForDevice(tctx, vmUUID, vdi.UUID, "RW")
	if errors.Is(err, xoa.ErrNoSuchObject) {
		fmt.Printf("Not found: %s\n", err)
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("Failed to attach disk: %v", err)
	}

	fmt.Printf("Successfully attached disk '%s' (VDI UUID: %s) to VM %s\n", diskName, vdi.UUID, vmUUID)
	fmt.Printf("VBD %+v\n", vbd)
}

// detachDisk detaches a disk from a VM
func detachDisk(client xoa.Client, diskName, vmUUID string) {
	ctx := context.Background()

	vdi, err := client.GetVDIByName(ctx, diskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		fmt.Printf("Disk '%s' not found\n", diskName)
		os.Exit(0)
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		fmt.Printf("Multiple disks found with name '%s'\n", diskName)
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("Failed to get VDIs: %v", err)
	}

	vbds, err := client.GetVBDsByVMAndVDI(ctx, vmUUID, vdi.UUID)
	if err != nil {
		log.Fatalf("Failed to get VBDs: %v", err)
	}

	if len(vbds) == 0 {
		fmt.Printf("No VBDs found for disk '%s' attached to VM %s\n", diskName, vmUUID)
		os.Exit(0)
	}

	fmt.Printf("Detaching disk '%s' from VM %s...\n", diskName, vmUUID)
	for _, vbd := range vbds {
		fmt.Printf("Deleting VBD: %+v\n", vbd)
		err := client.DeleteVBD(ctx, vbd.UUID)
		if errors.Is(err, xoa.ErrNoSuchObject) {
			fmt.Printf("VBD '%s' not found\n", vbd.UUID)
			os.Exit(0)
		} else if err != nil {
			log.Fatalf("Failed to delete VBD: %v", err)
		}
		fmt.Printf("Successfully detached disk '%s' from VM %s\n", diskName, vmUUID)
	}
	os.Exit(0)
}

// checkDiskAttachment checks if a disk is currently attached to any VM
func checkDiskAttachment(client xoa.Client, diskName, vmUUID string) {
	ctx := context.Background()

	vdi, err := client.GetVDIByName(ctx, diskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		fmt.Printf("Disk '%s' not found\n", diskName)
		os.Exit(0)
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		fmt.Printf("Multiple disks found with name '%s'\n", diskName)
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("Failed to get VDIs: %v", err)
	}

	fmt.Printf("Checking attachment status for disk '%s'...\n", diskName)
	vbds, err := client.GetVBDsByVMAndVDI(ctx, vmUUID, vdi.UUID)
	if err != nil {
		log.Fatalf("Failed to get VBDs: %v", err)
	}

	attached := false
	for _, vbd := range vbds {
		fmt.Printf("VBD: %+v\n", vbd)
		if vbd.Attached {
			attached = true
			break
		}
	}

	if attached {
		fmt.Printf("Disk '%s' is attached to VM %s\n", diskName, vmUUID)
	} else {
		fmt.Printf("Disk '%s' is not attached to VM %s\n", diskName, vmUUID)
	}
	os.Exit(0)
}

func migrateDisk(client xoa.Client, diskName, srUUID string) {
	ctx := context.Background()

	vdi, err := client.GetVDIByName(ctx, diskName)
	if err != nil {
		log.Fatalf("Failed to get VDIs: %v", err)
	}

	newVdiUUID, err := client.MigrateVDI(ctx, vdi.UUID, srUUID)
	if err != nil {
		log.Fatalf("Failed to migrate VDI: %v", err)
	}

	fmt.Printf("Successfully migrated disk '%s' to SR %s (new VDI UUID: %s)\n", diskName, srUUID, newVdiUUID)
}

func resizeDisk(client xoa.Client, diskName string, newSize int64) {
	ctx := context.Background()
	fmt.Printf("Resizing disk '%s' to %d bytes...\n", diskName, newSize)

	vdi, err := client.GetVDIByName(ctx, diskName)
	if errors.Is(err, xoa.ErrObjectNotFound) {
		fmt.Printf("Disk '%s' not found\n", diskName)
		os.Exit(0)
	} else if errors.Is(err, xoa.ErrMultipleObjectsFound) {
		fmt.Printf("Multiple disks found with name '%s'\n", diskName)
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("Failed to get VDIs: %v", err)
	}

	err = client.ResizeVDI(ctx, vdi.UUID, newSize)
	if errors.Is(err, xoa.ErrNoSuchObject) {
		fmt.Printf("Not found: %s\n", err)
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("Failed to resize VDI: %v", err)
	}

	fmt.Printf("Successfully resized disk '%s' to %d bytes\n", diskName, newSize)
}
