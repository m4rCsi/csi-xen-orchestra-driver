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
	"fmt"
	"os"
	"strings"
)

type NodeMetadataGetter interface {
	GetNodeId() (string, error)
}

type NodeMetadata struct {
}

func NewNodeMetadata() *NodeMetadata {
	return &NodeMetadata{}
}

func (n *NodeMetadata) GetNodeId() (string, error) {
	return getNodeIdFromDmiProductUUID()
}

// This should give us the VM UUID as reported in Xen Orchestra
// We are using `/sys/class/dmi/id/product_uuid`. However, I am not sure if there are more reliable ways to get the VM UUID.
// Another option is to use `xenstore-ls`
// There is also `/sys/hypervisor/uuid` but it's not clear if that's the same as the VM UUID
func getNodeIdFromDmiProductUUID() (string, error) {
	productUUID, err := os.ReadFile("/sys/class/dmi/id/product_uuid")
	if err != nil {
		return "", fmt.Errorf("failed to read product UUID: %w", err)
	}

	uuid := strings.TrimSpace(string(productUUID))
	if uuid == "" {
		return "", fmt.Errorf("failed to get product UUID: product UUID is empty")
	}

	return uuid, nil
}
