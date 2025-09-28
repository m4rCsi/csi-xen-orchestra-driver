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
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sourcegraph/jsonrpc2"
	jsonrpc2websocket "github.com/sourcegraph/jsonrpc2/websocket"
	"k8s.io/klog/v2"
)

// apiClient represents a Xen Orchestra WebSocket JSON-RPC client
type jsonRPCClient struct {
	baseURL string
	token   string
	config  ClientConfig
	conn    *jsonrpc2.Conn
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewClient creates a new Xen Orchestra WebSocket JSON-RPC client
func NewJSONRPCClient(config ClientConfig) (*jsonRPCClient, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required: %w", ErrInvalidArgument)
	}
	if config.Token == "" {
		return nil, fmt.Errorf("authentication token is required: %w", ErrInvalidArgument)
	}

	// Set default values
	if config.Timeout == 0 {
		config.Timeout = 300 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &jsonRPCClient{
		baseURL: config.BaseURL,
		token:   config.Token,
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
	}
	return client, nil
}

// Connect establishes a WebSocket connection to the Xen Orchestra API
func (c *jsonRPCClient) Connect(ctx context.Context) error {
	log := klog.FromContext(ctx)

	if c.conn != nil {
		return ErrAlreadyConnected
	}

	// Convert HTTP URL to WebSocket URL
	wsURL, err := c.getWebSocketURL()
	if err != nil {
		return fmt.Errorf("%w: failed to create WebSocket URL: %w", ErrConnectionError, err)
	}

	log.V(4).Info("Connecting to WebSocket", "wsURL", wsURL)

	// Create WebSocket connection
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to connect to WebSocket: %w", ErrConnectionError, err)
	}

	objectStream := jsonrpc2websocket.NewObjectStream(wsConn)

	conn := jsonrpc2.NewConn(ctx, objectStream, c)
	c.conn = conn

	// Authenticate the session
	if err := c.authenticate(ctx); err != nil {
		_ = c.Close() // Ensure connection is closed on auth failure
		return fmt.Errorf("authentication failed: %w", err)
	}

	log.V(4).Info("Successfully connected and authenticated to Xen Orchestra WebSocket API")
	return nil
}

func (h *jsonRPCClient) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// We will get notifications from the server, but we don't need to handle them
	// If we don't set a handler, we get a SegFault from the library
}

// Close closes the WebSocket connection
func (c *jsonRPCClient) Close() error {
	if c.conn == nil {
		return nil
	}

	c.cancel()
	err := c.conn.Close()
	c.conn = nil
	return err
}

// Call makes a JSON-RPC call and waits for the response
func (c *jsonRPCClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	callctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	var result json.RawMessage
	err := c.conn.Call(callctx, method, params, &result)
	if err != nil {
		switch err := err.(type) {
		case *jsonrpc2.Error:
			return nil, ConvertJSONRPCError(err)
		default:
			return nil, err
		}
	}
	return result, nil
}

// getWebSocketURL converts the HTTP URL to a WebSocket URL
func (c *jsonRPCClient) getWebSocketURL() (string, error) {
	parsedURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	// Convert to WebSocket scheme
	if parsedURL.Scheme == "https" {
		parsedURL.Scheme = "wss"
	} else {
		parsedURL.Scheme = "ws"
	}

	// Set the WebSocket endpoint path to /api/
	parsedURL.Path = "/api/"

	// Authentication is now done via cookie in the handshake.
	return parsedURL.String(), nil
}

// authenticate performs authentication with the server
func (c *jsonRPCClient) authenticate(ctx context.Context) error {
	log := klog.FromContext(ctx)
	log.V(4).Info("Authenticating session with token...")
	_, err := c.call(ctx, "session.signInWithToken", map[string]any{
		"token": c.token,
	})
	if err != nil {
		return fmt.Errorf("session.signInWithToken failed: %w", err)
	}

	log.V(4).Info("Session successfully authenticated")
	return nil
}

// GetVMs retrieves all virtual machines
func (c *jsonRPCClient) GetVMs(ctx context.Context, filter map[string]any) ([]VM, error) {
	if filter == nil {
		filter = make(map[string]any)
	}
	filter["type"] = "VM"
	result, err := c.call(ctx, "xo.getAllObjects", map[string]any{
		"filter": filter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs: %w", err)
	}

	var vmMap map[string]VM
	if err := json.Unmarshal(result, &vmMap); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal VMs: %w", ErrUnmarshalError, err)
	}

	vms := make([]VM, 0, len(vmMap))
	for _, vm := range vmMap {
		vms = append(vms, vm)
	}

	return vms, nil
}

func (c *jsonRPCClient) GetOneVM(ctx context.Context, filter map[string]any) (*VM, error) {
	vms, err := c.GetVMs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(vms) == 0 {
		return nil, ErrObjectNotFound
	}

	if len(vms) > 1 {
		return nil, fmt.Errorf("%w: multiple VMs found", ErrMultipleObjectsFound)
	}

	return &vms[0], nil
}

// GetVM retrieves a specific virtual machine by UUID
func (c *jsonRPCClient) GetVMByUUID(ctx context.Context, uuid string) (*VM, error) {
	return c.GetOneVM(ctx, map[string]any{
		"uuid": uuid,
	})
}

// GetVDIs retrieves all virtual disk images
func (c *jsonRPCClient) GetVDIs(ctx context.Context, filter map[string]any) ([]VDI, error) {
	if filter == nil {
		filter = make(map[string]any)
	}
	filter["type"] = "VDI"
	result, err := c.call(ctx, "xo.getAllObjects", map[string]any{
		"filter": filter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VDIs: %w", err)
	}

	var vdiMap map[string]VDI
	if err := json.Unmarshal(result, &vdiMap); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal VDIs: %w", ErrUnmarshalError, err)
	}

	vdis := make([]VDI, 0, len(vdiMap))
	for _, vdi := range vdiMap {
		vdis = append(vdis, vdi)
	}

	return vdis, nil
}

func (c *jsonRPCClient) GetOneVDI(ctx context.Context, filter map[string]any) (*VDI, error) {
	vdis, err := c.GetVDIs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(vdis) == 0 {
		return nil, ErrObjectNotFound
	}

	if len(vdis) > 1 {
		return nil, fmt.Errorf("%w: multiple VDIs found with filter %v", ErrMultipleObjectsFound, filter)
	}

	return &vdis[0], nil
}

func (c *jsonRPCClient) GetVDIByName(ctx context.Context, name string) (*VDI, error) {
	return c.GetOneVDI(ctx, map[string]any{
		"name_label": name,
	})
}

// GetVDI retrieves a specific virtual disk image by UUID
func (c *jsonRPCClient) GetVDIByUUID(ctx context.Context, uuid string) (*VDI, error) {
	return c.GetOneVDI(ctx, map[string]any{
		"uuid": uuid,
	})
}

func (c *jsonRPCClient) EditVDI(ctx context.Context, uuid string, name, description *string) error {
	log := klog.FromContext(ctx)

	params := map[string]any{
		"id": uuid,
	}

	if description != nil {
		params["name_description"] = *description
	}

	if name != nil {
		params["name_label"] = *name
	}

	log.V(4).Info("Editing VDI", "uuid", uuid, "params", params)

	result, err := c.call(ctx, "vdi.set", params)
	if err != nil {
		return fmt.Errorf("failed to set VDI description: %w", err)
	}

	log.V(4).Info("Set VDI description", "uuid", uuid, "result", string(result))
	return nil
}

// GetSRs retrieves all storage repositories
func (c *jsonRPCClient) GetSRs(ctx context.Context, filter map[string]any) ([]SR, error) {
	if filter == nil {
		filter = make(map[string]any)
	}
	filter["type"] = "SR"
	result, err := c.call(ctx, "xo.getAllObjects", map[string]any{
		"filter": filter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get SRs: %w", err)
	}

	var srMap map[string]SR
	if err := json.Unmarshal(result, &srMap); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal SRs: %w", ErrUnmarshalError, err)
	}

	srs := make([]SR, 0, len(srMap))
	for _, sr := range srMap {
		srs = append(srs, sr)
	}

	return srs, nil
}

func (c *jsonRPCClient) GetOneSR(ctx context.Context, filter map[string]any) (*SR, error) {
	srs, err := c.GetSRs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(srs) == 0 {
		return nil, ErrObjectNotFound
	}

	if len(srs) > 1 {
		return nil, fmt.Errorf("%w: multiple SRs found with filter %v", ErrMultipleObjectsFound, filter)
	}

	return &srs[0], nil
}

func (c *jsonRPCClient) GetSRsWithTag(ctx context.Context, tag string) ([]SR, error) {
	return c.GetSRs(ctx, map[string]any{
		"tags": []string{tag},
	})
}

// GetSR retrieves a specific storage repository by UUID
func (c *jsonRPCClient) GetSRByUUID(ctx context.Context, uuid string) (*SR, error) {
	return c.GetOneSR(ctx, map[string]any{
		"uuid": uuid,
	})
}

// CreateVDI creates a new virtual disk image
func (c *jsonRPCClient) CreateVDI(ctx context.Context, nameLabel, srUUID string, size int64) (string, error) {
	result, err := c.call(ctx, "disk.create", map[string]any{
		"name": nameLabel,
		"size": size,
		"sr":   srUUID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create VDI: %w", err)
	}

	var vdiID string
	if err := json.Unmarshal(result, &vdiID); err != nil {
		return "", fmt.Errorf("%w: failed to unmarshal created VDI ID: %w", ErrUnmarshalError, err)
	}

	return vdiID, nil
}

func (c *jsonRPCClient) ResizeVDI(ctx context.Context, uuid string, size int64) error {
	log := klog.FromContext(ctx)

	result, err := c.call(ctx, "vdi.set", map[string]any{
		"id":   uuid,
		"size": size,
	})

	if err != nil {
		return fmt.Errorf("failed to resize VDI: %w", err)
	}

	log.V(4).Info("Resized VDI", "uuid", uuid, "result", string(result))
	return nil
}

// DeleteVDI deletes a virtual disk image
func (c *jsonRPCClient) DeleteVDI(ctx context.Context, uuid string) error {
	log := klog.FromContext(ctx)

	resp, err := c.call(ctx, "vdi.delete", map[string]any{
		"id": uuid,
	})
	if err != nil {
		return fmt.Errorf("failed to delete VDI %s: %w", uuid, err)
	}

	var ok bool
	if err := json.Unmarshal(resp, &ok); err != nil {
		return fmt.Errorf("%w: failed to unmarshal delete VDI response: %w", ErrUnmarshalError, err)
	}

	if !ok {
		return fmt.Errorf("failed to delete VDI %s", uuid)
	}

	log.V(4).Info("Deleted VDI", "uuid", uuid)
	return nil
}

// AttachVDI attaches a VDI (disk) to a VM
func (c *jsonRPCClient) AttachVDI(ctx context.Context, vmUUID, vdiUUID string, mode string) (*bool, error) {
	result, err := c.call(ctx, "vm.attachDisk", map[string]any{
		"vm":   vmUUID,
		"vdi":  vdiUUID,
		"mode": mode, // "RW" or "RO"
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach VDI: %w", err)
	}

	// Try to unmarshal as bool (XO returns true on success)
	var ok bool
	if err := json.Unmarshal(result, &ok); err == nil {
		return &ok, nil // Success, but no VBD ID returned
	}
	return nil, fmt.Errorf("unexpected response from vm.attachDisk: %s", string(result))
}

func (c *jsonRPCClient) ConnectVBDAndWaitForDevice(ctx context.Context, vbdUUID string) (*VBD, error) {
	log := klog.FromContext(ctx)

	// Check if VBD is already connected
	vbd, err := c.GetVBDByUUID(ctx, vbdUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VBD: %w", err)
	}

	if vbd.Attached && vbd.Device != "" {
		return vbd, nil
	}

	err = c.ConnectVBD(ctx, vbdUUID)
	if err != nil {
		return nil, err
	}

	// Poll every 500ms until VBD is created or context is cancelled
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for VBD: %w", ctx.Err())
		case <-ticker.C:
			// Check if VBD has been created
			vbd, err := c.GetVBDByUUID(ctx, vbdUUID)
			if err != nil {
				return nil, fmt.Errorf("failed to get VBD: %w", err)
			}

			if vbd.Attached && vbd.Device != "" {
				log.V(4).Info("VBD is connected to device", "vbdUUID", vbdUUID, "device", vbd.Device)
				return vbd, nil
			}

			// log.V(5).Info("VBD not yet created, continuing to poll...", "vbdUUID", vbdUUID)
		}
	}
}

func (c *jsonRPCClient) AttachVDIAndWaitForDevice(ctx context.Context, vmUUID, vdiUUID string, mode string) (*VBD, error) {
	log := klog.FromContext(ctx)

	ok, err := c.AttachVDI(ctx, vmUUID, vdiUUID, mode)
	if err != nil {
		return nil, err
	}

	if !*ok {
		return nil, fmt.Errorf("failed to attach VDI")
	}

	// Poll every 500ms until VBD is created or context is cancelled
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for VBD: %w", ctx.Err())
		case <-ticker.C:
			// Check if VBD has been created
			vbds, err := c.GetVBDsByVMAndVDI(ctx, vmUUID, vdiUUID)
			if err != nil {
				return nil, fmt.Errorf("failed to get VBDs: %w", err)
			}

			for _, vbd := range vbds {
				if vbd.Attached && vbd.Device != "" {
					log.V(4).Info("VBD is attached to device", "vbdUUID", vbd.UUID, "device", vbd.Device)
					return &vbd, nil
				}
			}

			// log.V(5).Info("VBD not yet created, continuing to poll...", "vmUUID", vmUUID, "vdiUUID", vdiUUID)
		}
	}
}

// GetVBDs retrieves all Virtual Block Devices (VBDs)
func (c *jsonRPCClient) GetVBDs(ctx context.Context, filter map[string]any) ([]VBD, error) {
	if filter == nil {
		filter = make(map[string]any)
	}
	filter["type"] = "VBD"
	result, err := c.call(ctx, "xo.getAllObjects", map[string]any{
		"filter": filter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VBDs: %w", err)
	}

	var vbdMap map[string]VBD
	if err := json.Unmarshal(result, &vbdMap); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal VBDs: %w", ErrUnmarshalError, err)
	}

	vbds := make([]VBD, 0, len(vbdMap))
	for _, vbd := range vbdMap {
		vbds = append(vbds, vbd)
	}

	return vbds, nil
}

func (c *jsonRPCClient) GetVBDsByVMAndVDI(ctx context.Context, vmUUID, vdiUUID string) ([]VBD, error) {
	return c.GetVBDs(ctx, map[string]any{
		"VM":  vmUUID,
		"VDI": vdiUUID,
	})
}

func (c *jsonRPCClient) GetVBDsByVDI(ctx context.Context, vdiUUID string) ([]VBD, error) {
	return c.GetVBDs(ctx, map[string]any{
		"VDI": vdiUUID,
	})
}

func (c *jsonRPCClient) GetOneVBD(ctx context.Context, filter map[string]any) (*VBD, error) {
	vbds, err := c.GetVBDs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(vbds) == 0 {
		return nil, ErrObjectNotFound
	}

	if len(vbds) > 1 {
		return nil, fmt.Errorf("%w: multiple VBDs found with filter %v", ErrMultipleObjectsFound, filter)
	}

	return &vbds[0], nil
}

func (c *jsonRPCClient) GetVBDByUUID(ctx context.Context, vbdUUID string) (*VBD, error) {
	return c.GetOneVBD(ctx, map[string]any{
		"uuid": vbdUUID,
	})
}

func (c *jsonRPCClient) DisconnectVBD(ctx context.Context, vbdUUID string) error {
	log := klog.FromContext(ctx)

	resp, err := c.call(ctx, "vbd.disconnect", map[string]any{
		"id": vbdUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to disconnect VBD %s: %w", vbdUUID, err)
	}

	log.V(4).Info("Disconnected VBD", "vbdUUID", vbdUUID, "result", string(resp))
	return nil
}

func (c *jsonRPCClient) ConnectVBD(ctx context.Context, vbdUUID string) error {
	log := klog.FromContext(ctx)

	resp, err := c.call(ctx, "vbd.connect", map[string]any{
		"id": vbdUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to connect VBD %s: %w", vbdUUID, err)
	}

	var ok bool
	if err := json.Unmarshal(resp, &ok); err != nil {
		return fmt.Errorf("%w: failed to unmarshal connect VBD response: %w", ErrUnmarshalError, err)
	}
	if !ok {
		return fmt.Errorf("failed to connect VBD %s", vbdUUID)
	}

	log.V(4).Info("Connected VBD", "vbdUUID", vbdUUID)
	return nil
}

func (c *jsonRPCClient) DeleteVBD(ctx context.Context, vbdUUID string) error {
	resp, err := c.call(ctx, "vbd.delete", map[string]any{
		"id": vbdUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete VBD %s: %w", vbdUUID, err)
	}

	var ok bool
	if err := json.Unmarshal(resp, &ok); err != nil {
		return fmt.Errorf("%w: failed to unmarshal delete VBD response: %w", ErrUnmarshalError, err)
	}
	if !ok {
		return fmt.Errorf("failed to delete VBD %s", vbdUUID)
	}

	return nil
}

func (c *jsonRPCClient) MigrateVDI(ctx context.Context, vdiUUID, srUUID string) (string, error) {
	log := klog.FromContext(ctx)

	resp, err := c.call(ctx, "vdi.migrate", map[string]any{
		"id":    vdiUUID,
		"sr_id": srUUID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to migrate VDI: %w", err)
	}

	var newVdiUUID string
	if err := json.Unmarshal(resp, &newVdiUUID); err != nil {
		return "", fmt.Errorf("%w: failed to unmarshal new VDI UUID: %w", ErrUnmarshalError, err)
	}

	log.V(4).Info("Migrated VDI", "vdiUUID", vdiUUID, "srUUID", srUUID, "result", string(resp))
	return newVdiUUID, nil
}

// Compile-time check to ensure Client implements XOAClient interface
var _ Client = (*jsonRPCClient)(nil)
