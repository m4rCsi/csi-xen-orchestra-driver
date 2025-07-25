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

package xoa

import (
	"context"
	"time"
)

// Config holds the configuration for the Xen Orchestra API client
type ClientConfig struct {
	BaseURL    string
	Token      string
	Timeout    time.Duration
	RetryCount int
	RetryWait  time.Duration
}

// Client defines the interface for Xen Orchestra API operations
type Client interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error

	// VM operations
	GetVMs(ctx context.Context, filter map[string]any) ([]VM, error)
	GetOneVM(ctx context.Context, filter map[string]any) (*VM, error)
	GetVMByUUID(ctx context.Context, uuid string) (*VM, error)

	// VDI (Virtual Disk Image) operations
	GetVDIs(ctx context.Context, filter map[string]any) ([]VDI, error)
	GetOneVDI(ctx context.Context, filter map[string]any) (*VDI, error)
	GetVDIByUUID(ctx context.Context, uuid string) (*VDI, error)
	GetVDIByName(ctx context.Context, name string) (*VDI, error)
	CreateVDI(ctx context.Context, nameLabel, srUUID string, size int64) (string, error)
	EditVDI(ctx context.Context, uuid string, nameLabel string, size int64) error
	ResizeVDI(ctx context.Context, uuid string, size int64) error
	DeleteVDI(ctx context.Context, uuid string) error
	MigrateVDI(ctx context.Context, vdiUUID, srUUID string) error

	// SR (Storage Repository) operations
	GetSRs(ctx context.Context, filter map[string]any) ([]SR, error)
	GetOneSR(ctx context.Context, filter map[string]any) (*SR, error)
	GetSRByUUID(ctx context.Context, uuid string) (*SR, error)

	// VBD (Virtual Block Device) operations
	GetVBDs(ctx context.Context, filter map[string]any) ([]VBD, error)
	GetOneVBD(ctx context.Context, filter map[string]any) (*VBD, error)
	GetVBDsByVMAndVDI(ctx context.Context, vmUUID, vdiUUID string) ([]VBD, error)
	GetVBDByUUID(ctx context.Context, vbdUUID string) (*VBD, error)

	// AttachVDI attaches a VDI to a VM
	AttachVDI(ctx context.Context, vmUUID, vdiUUID string, mode string) (*bool, error)
	AttachVDIAndWaitForDevice(ctx context.Context, vmUUID, vdiUUID string, mode string) (*VBD, error)
	DisconnectVBD(ctx context.Context, vbdUUID string) error
	ConnectVBD(ctx context.Context, vbdUUID string) error
	DeleteVBD(ctx context.Context, vbdUUID string) error
}

// CPUs represents the CPU configuration for a VM
type CPUs struct {
	Cores   int `json:"cores,omitempty"`
	Sockets int `json:"sockets,omitempty"`
}

type Memory struct {
	Size int64 `json:"size"`
}

// VM represents a virtual machine
type VM struct {
	UUID         string `json:"uuid"`
	NameLabel    string `json:"name_label"`
	PowerState   string `json:"power_state"`
	CPUs         CPUs   `json:"cpus"`
	Memory       Memory `json:"memory"`
	VCPUsAtStart int    `json:"vcpus_at_start,omitempty"`
	VCPUsMax     int    `json:"vcpus_max,omitempty"`
	Pool         string `json:"$pool,omitempty"`
	Host         string `json:"$container,omitempty"`
}

// VDI represents a virtual disk image
type VDI struct {
	UUID      string `json:"uuid"`
	NameLabel string `json:"name_label"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	SR        string `json:"$sr,omitempty"`
	Pool      string `json:"$pool,omitempty"`
	ReadOnly  bool   `json:"read_only,omitempty"`
	Sharable  bool   `json:"sharable,omitempty"`
}

// SR represents a storage repository
type SR struct {
	UUID                string `json:"uuid"`
	NameLabel           string `json:"name_label"`
	Type                string `json:"type"`
	PhysicalSize        int64  `json:"physical_size"`
	VirtualAllocation   int64  `json:"virtual_allocation,omitempty"`
	PhysicalUtilisation int64  `json:"physical_utilisation,omitempty"`
	Pool                string `json:"$pool,omitempty"`
	Host                string `json:"$container,omitempty"`
}

// VBD represents a Virtual Block Device (connection between VDI and VM)
type VBD struct {
	UUID      string `json:"uuid"`
	NameLabel string `json:"name_label,omitempty"`
	VM        string `json:"VM,omitempty"`
	VDI       string `json:"VDI,omitempty"`
	Device    string `json:"device,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Bootable  bool   `json:"bootable,omitempty"`
	Attached  bool   `json:"attached,omitempty"`
}
