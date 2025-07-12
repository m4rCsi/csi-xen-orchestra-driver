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

package sanity

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	"k8s.io/klog/v2"
)

// generateUUID generates a UUID with a prefix
func generateUUID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String())
}

// FakeClient implements the xoa.Client interface for testing purposes
type FakeClient struct {
	mu sync.RWMutex

	// In-memory storage
	vms  map[string]*xoa.VM
	vdis map[string]*xoa.VDI
	srs  map[string]*xoa.SR
	vbds map[string]*xoa.VBD

	// State
	isConnected bool
}

// NewFakeClient creates a new fake XOAClient
// Use InjectVM, InjectVDI, InjectSR, InjectVBD to inject test data.
func NewFakeClient() *FakeClient {
	return &FakeClient{
		vms:  make(map[string]*xoa.VM),
		vdis: make(map[string]*xoa.VDI),
		srs:  make(map[string]*xoa.SR),
		vbds: make(map[string]*xoa.VBD),
	}
}

// AddVM adds a VM to the fake client's storage
func (f *FakeClient) InjectVM(vm *xoa.VM) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vms[vm.UUID] = vm
}

// AddVDI adds a VDI to the fake client's storage
func (f *FakeClient) InjectVDI(vdi *xoa.VDI) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vdis[vdi.UUID] = vdi
}

// AddSR adds an SR to the fake client's storage
func (f *FakeClient) InjectSR(sr *xoa.SR) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.srs[sr.UUID] = sr
}

// AddVBD adds a VBD to the fake client's storage
func (f *FakeClient) InjectVBD(vbd *xoa.VBD) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vbds[vbd.UUID] = vbd
}

// RemoveVM removes a VM from the fake client's storage
func (f *FakeClient) EjectVM(uuid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.vms, uuid)
}

// RemoveVDI removes a VDI from the fake client's storage
func (f *FakeClient) EjectVDI(uuid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.vdis, uuid)
}

// RemoveVBD removes a VBD from the fake client's storage
func (f *FakeClient) EjectVBD(uuid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.vbds, uuid)
}

// Clear clears all data from the fake client
func (f *FakeClient) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vms = make(map[string]*xoa.VM)
	f.vdis = make(map[string]*xoa.VDI)
	f.srs = make(map[string]*xoa.SR)
	f.vbds = make(map[string]*xoa.VBD)
}

// IsConnected returns the current connection state
func (f *FakeClient) IsConnected() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.isConnected
}

// Connect establishes a fake connection
func (f *FakeClient) Connect(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.isConnected {
		return fmt.Errorf("already connected")
	}

	f.isConnected = true
	return nil
}

// Close closes the fake connection
func (f *FakeClient) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isConnected = false
	return nil
}

// GetVMs returns all VMs from the fake storage
func (f *FakeClient) GetVMs(ctx context.Context, filter map[string]any) ([]xoa.VM, error) {
	if filter != nil {
		return nil, fmt.Errorf("fakeClient: filter not supported for GetVMs")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	vms := make([]xoa.VM, 0, len(f.vms))
	for _, vm := range f.vms {
		vms = append(vms, *vm)
	}
	return vms, nil
}

// GetVM returns a specific VM by UUID
func (f *FakeClient) GetVMByUUID(ctx context.Context, uuid string) (*xoa.VM, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	vm, exists := f.vms[uuid]
	if !exists {
		return nil, xoa.ErrObjectNotFound
	}
	return vm, nil
}

func (f *FakeClient) GetOneVM(ctx context.Context, filter map[string]any) (*xoa.VM, error) {
	vms, err := f.GetVMs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(vms) == 0 {
		return nil, xoa.ErrObjectNotFound
	}

	if len(vms) > 1 {
		return nil, fmt.Errorf("%w: multiple VMs found", xoa.ErrMultipleObjectsFound)
	}

	return &vms[0], nil
}

// GetVDIs returns all VDIs from the fake storage
func (f *FakeClient) GetVDIs(ctx context.Context, filter map[string]any) ([]xoa.VDI, error) {
	if filter != nil {
		return nil, fmt.Errorf("fakeClient: filter not supported for GetVDIs")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	vdis := make([]xoa.VDI, 0, len(f.vdis))
	for _, vdi := range f.vdis {
		vdis = append(vdis, *vdi)
	}
	return vdis, nil
}

func (f *FakeClient) GetOneVDI(ctx context.Context, filter map[string]any) (*xoa.VDI, error) {
	vdis, err := f.GetVDIs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(vdis) == 0 {
		return nil, xoa.ErrObjectNotFound
	}

	if len(vdis) > 1 {
		return nil, fmt.Errorf("%w: multiple VDIs found", xoa.ErrMultipleObjectsFound)
	}

	return &vdis[0], nil
}

// GetVDIByUUID returns a specific VDI by UUID
func (f *FakeClient) GetVDIByUUID(ctx context.Context, uuid string) (*xoa.VDI, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	vdi, exists := f.vdis[uuid]
	if !exists {
		return nil, xoa.ErrObjectNotFound
	}
	return vdi, nil
}

// GetVDIByName returns a specific VDI by name
func (f *FakeClient) GetVDIByName(ctx context.Context, name string) (*xoa.VDI, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	for _, vdi := range f.vdis {
		if vdi.NameLabel == name {
			return vdi, nil
		}
	}
	return nil, xoa.ErrObjectNotFound
}

// CreateVDI creates a new VDI in the fake storage
func (f *FakeClient) CreateVDI(ctx context.Context, nameLabel, srUUID string, size int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return "", err
	}

	vdiUUID := generateUUID("vdi")
	vdi := &xoa.VDI{
		UUID:      vdiUUID,
		NameLabel: nameLabel,
		Size:      size,
		Type:      "user",
		SR:        srUUID,
		ReadOnly:  false,
		Sharable:  false,
	}

	f.vdis[vdiUUID] = vdi
	klog.V(2).InfoS("Created fake VDI", "vdi", vdi)

	return vdiUUID, nil
}

// EditVDI edits a VDI in the fake storage
func (f *FakeClient) EditVDI(ctx context.Context, uuid string, nameLabel string, size int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return err
	}

	vdi, exists := f.vdis[uuid]
	if !exists {
		return xoa.ErrNoSuchObject
	}

	if nameLabel != "" {
		vdi.NameLabel = nameLabel
	}
	if size > 0 {
		vdi.Size = size
	}

	return nil
}

// ResizeVDI resizes a VDI in the fake storage
func (f *FakeClient) ResizeVDI(ctx context.Context, uuid string, size int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return err
	}

	vdi, exists := f.vdis[uuid]
	if !exists {
		return xoa.ErrNoSuchObject
	}

	vdi.Size = size
	klog.V(2).InfoS("Resized fake VDI", "uuid", uuid, "size", size)

	return nil
}

// DeleteVDI deletes a VDI from the fake storage
func (f *FakeClient) DeleteVDI(ctx context.Context, uuid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return err
	}

	if _, exists := f.vdis[uuid]; !exists {
		return xoa.ErrNoSuchObject
	}

	delete(f.vdis, uuid)
	klog.V(2).InfoS("Deleted fake VDI", "uuid", uuid)

	return nil
}

// GetSRs returns all SRs from the fake storage
func (f *FakeClient) GetSRs(ctx context.Context, filter map[string]any) ([]xoa.SR, error) {
	if filter != nil {
		return nil, fmt.Errorf("fakeClient: filter not supported for GetSRs")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	srs := make([]xoa.SR, 0, len(f.srs))
	for _, sr := range f.srs {
		srs = append(srs, *sr)
	}
	return srs, nil
}

func (f *FakeClient) GetOneSR(ctx context.Context, filter map[string]any) (*xoa.SR, error) {
	srs, err := f.GetSRs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(srs) == 0 {
		return nil, xoa.ErrObjectNotFound
	}

	if len(srs) > 1 {
		return nil, fmt.Errorf("%w: multiple SRs found", xoa.ErrMultipleObjectsFound)
	}

	return &srs[0], nil
}

// GetSRByUUID returns a specific SR by UUID
func (f *FakeClient) GetSRByUUID(ctx context.Context, uuid string) (*xoa.SR, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	sr, exists := f.srs[uuid]
	if !exists {
		return nil, xoa.ErrObjectNotFound
	}
	return sr, nil
}

// GetVBDs returns all VBDs from the fake storage
func (f *FakeClient) GetVBDs(ctx context.Context, filter map[string]any) ([]xoa.VBD, error) {
	if filter != nil {
		return nil, fmt.Errorf("fakeClient: filter not supported for GetVBDs")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	vbds := make([]xoa.VBD, 0, len(f.vbds))
	for _, vbd := range f.vbds {
		vbds = append(vbds, *vbd)
	}
	return vbds, nil
}

func (f *FakeClient) GetOneVBD(ctx context.Context, filter map[string]any) (*xoa.VBD, error) {
	vbds, err := f.GetVBDs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(vbds) == 0 {
		return nil, xoa.ErrObjectNotFound
	}

	if len(vbds) > 1 {
		return nil, fmt.Errorf("%w: multiple VBDs found", xoa.ErrMultipleObjectsFound)
	}

	return &vbds[0], nil
}

func (f *FakeClient) GetVBDsByVMAndVDI(ctx context.Context, vmUUID, vdiUUID string) ([]xoa.VBD, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	vbds := make([]xoa.VBD, 0, len(f.vbds))
	for _, vbd := range f.vbds {
		if vbd.VM == vmUUID && vbd.VDI == vdiUUID {
			vbds = append(vbds, *vbd)
		}
	}
	return vbds, nil
}

// GetVBDByUUID returns a specific VBD by UUID
func (f *FakeClient) GetVBDByUUID(ctx context.Context, vbdUUID string) (*xoa.VBD, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	vbd, exists := f.vbds[vbdUUID]
	if !exists {
		return nil, xoa.ErrObjectNotFound
	}
	return vbd, nil
}

// AttachVDI attaches a VDI to a VM in the fake storage
func (f *FakeClient) AttachVDI(ctx context.Context, vmUUID, vdiUUID string, mode string) (*bool, error) {
	vbd, err := f.AttachVDIAndWaitForDevice(ctx, vmUUID, vdiUUID, mode)
	if err != nil {
		return nil, err
	}
	return &vbd.Attached, nil
}

func (f *FakeClient) AttachVDIAndWaitForDevice(ctx context.Context, vmUUID, vdiUUID string, mode string) (*xoa.VBD, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return nil, err
	}

	// Check if VM exists
	if _, exists := f.vms[vmUUID]; !exists {
		return nil, xoa.ErrNoSuchObject
	}

	// Check if VDI exists
	if _, exists := f.vdis[vdiUUID]; !exists {
		return nil, xoa.ErrNoSuchObject
	}

	// Create a new VBD
	vbdUUID := generateUUID("vbd")
	vbd := &xoa.VBD{
		UUID:     vbdUUID,
		VM:       vmUUID,
		VDI:      vdiUUID,
		Device:   "xvda", // Default device name
		Mode:     mode,
		Bootable: false,
		Attached: true,
	}

	f.vbds[vbdUUID] = vbd
	klog.V(2).InfoS("Created fake VBD", "vbd", vbd)

	return vbd, nil
}

// DisconnectVBD disconnects a VBD in the fake storage
func (f *FakeClient) DisconnectVBD(ctx context.Context, vbdUUID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return err
	}

	vbd, exists := f.vbds[vbdUUID]
	if !exists {
		return xoa.ErrNoSuchObject
	}

	vbd.Attached = false
	klog.V(2).InfoS("Disconnected fake VBD", "vbdUUID", vbdUUID)

	return nil
}

// ConnectVBD connects a VBD in the fake storage
func (f *FakeClient) ConnectVBD(ctx context.Context, vbdUUID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return err
	}

	vbd, exists := f.vbds[vbdUUID]
	if !exists {
		return xoa.ErrNoSuchObject
	}

	vbd.Attached = true
	klog.V(2).InfoS("Connected fake VBD", "vbdUUID", vbdUUID)

	return nil
}

// DeleteVBD deletes a VBD from the fake storage
func (f *FakeClient) DeleteVBD(ctx context.Context, vbdUUID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkConnection(); err != nil {
		return err
	}

	if _, exists := f.vbds[vbdUUID]; !exists {
		return xoa.ErrNoSuchObject
	}

	delete(f.vbds, vbdUUID)
	klog.V(2).InfoS("Deleted fake VBD", "vbdUUID", vbdUUID)

	return nil
}

// checkConnection checks if the client is connected
func (f *FakeClient) checkConnection() error {
	if !f.isConnected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// Compile-time check to ensure Client implements xoa.Client interface
var _ xoa.Client = (*FakeClient)(nil)
