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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type NodeMetadataGetter interface {
	GetNodeId() (string, error)
}

type NodeMetadata struct {
	nodeName string
}

func NewNodeMetadata(nodeName string) *NodeMetadata {
	return &NodeMetadata{
		nodeName: nodeName,
	}
}

func (n *NodeMetadata) GetNodeId() (string, error) {
	// TODO: is systemUUID actually the right way to detect the VM UUID?
	return getNodeIdFromSystemUUID(n.nodeName)
}

func getNodeIdFromSystemUUID(nodeName string) (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("failed to create Kubernetes client: %v", err)
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create Kubernetes client: %v", err)
	}

	klog.Infof("Getting node ID for %s", nodeName)
	node, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to get node: %v", err)
	}
	return node.Status.NodeInfo.SystemUUID, nil
}
