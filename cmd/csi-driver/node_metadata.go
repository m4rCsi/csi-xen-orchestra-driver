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
	"fmt"
	"os"
	"strings"

	"github.com/m4rCsi/csi-xen-orchestra-driver/pkg/csi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "k8s.io/client-go/kubernetes"
)

type NodeMetadataFromKubernetes struct {
	client       kclient.Interface
	nodeName     string
	hostTopology bool
}

func NewNodeMetadataFromKubernetes(client kclient.Interface, nodeName string, hostTopology bool) *NodeMetadataFromKubernetes {
	return &NodeMetadataFromKubernetes{
		client:       client,
		nodeName:     nodeName,
		hostTopology: hostTopology,
	}
}

func (n *NodeMetadataFromKubernetes) GetNodeMetadata() (*csi.NodeMetadata, error) {
	nodeId, err := getNodeIdFromDmiProductUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to get node id: %w", err)
	}

	node, err := n.client.CoreV1().Nodes().Get(context.Background(), n.nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	hostId := ""
	if n.hostTopology {
		hostId = node.Labels["topology.k8s.xenorchestra/host_id"]
	}

	poolId := node.Labels["topology.k8s.xenorchestra/pool_id"]

	return &csi.NodeMetadata{
		NodeId: nodeId,
		HostId: hostId,
		PoolId: poolId,
	}, nil
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
