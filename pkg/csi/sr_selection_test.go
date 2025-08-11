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
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	xoa "github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestPickSR(t *testing.T) {
	srs := []xoa.SR{
		{UUID: "sr-on-host1-pool1", Usage: 100, Size: 1000, Host: "host1", Pool: "pool1"},
		{UUID: "sr-on-host2-pool1", Usage: 200, Size: 1000, Host: "host2", Pool: "pool1"},
		{UUID: "sr-on-host3-pool1", Usage: 000, Size: 1000, Host: "host3", Pool: "pool1"},
		{UUID: "sr-on-host3-pool2", Usage: 300, Size: 1000, Host: "host1", Pool: "pool2"},
		{UUID: "sr-on-host4-pool2", Usage: 400, Size: 1000, Host: "host2", Pool: "pool2"},
	}

	testCases := []struct {
		// Test Name
		name string

		// Inputs
		topology *csi.TopologyRequirement
		scope    SRFilterScope
		capacity int64

		// Output
		expectedSRUUID string
		expectedError  error
	}{
		{
			name: "host-scoped: pick preferred",
			topology: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{Segments: map[string]string{"host": "host1", "pool": "pool1"}},
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
				Preferred: []*csi.Topology{
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
			},
			scope:          SRFilterScopeHost,
			capacity:       100,
			expectedSRUUID: "sr-on-host2-pool1",
		},
		{
			name: "host-scoped: preferred not enough space",
			topology: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{Segments: map[string]string{"host": "host1", "pool": "pool1"}},
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
				Preferred: []*csi.Topology{
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
			},
			scope:          SRFilterScopeHost,
			capacity:       850,
			expectedSRUUID: "sr-on-host1-pool1",
		},
		{
			name: "host-scoped: not enough space in topology",
			topology: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{Segments: map[string]string{"host": "host1", "pool": "pool1"}},
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
				Preferred: []*csi.Topology{
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
			},
			scope:         SRFilterScopeHost,
			capacity:      950,
			expectedError: ErrNoSpace,
		},
		{
			// TODO: Is this right? Investigate if we can pick a host that is not in the requisite topology if we are scoped by pool
			name: "pool-scoped: not enough space in topology",
			topology: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{Segments: map[string]string{"host": "host1", "pool": "pool1"}},
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
				Preferred: []*csi.Topology{
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
			},
			scope:          SRFilterScopePool,
			capacity:       950,
			expectedSRUUID: "sr-on-host3-pool1",
		},
		// TODO: Add tests for more pool scoped and global scoped
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sr, err := pickSR(srs, testCase.capacity, testCase.topology, testCase.scope)
			if testCase.expectedError != nil {
				assert.ErrorIs(t, err, testCase.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, sr.UUID, testCase.expectedSRUUID)
			}
		})
	}
}

func TestScope(t *testing.T) {
	storageSelectionTag := storageSelectionFromParameters(&storageParameters{
		SRsWithTag: ptr.To("k8s-local"),
	})

	assert.Equal(t, storageSelectionTag.getScope(), SRFilterScopeHost)

	storageSelectionUUID := storageSelectionFromParameters(&storageParameters{
		SRUUID: ptr.To("sr-on-host1"),
	})

	assert.Equal(t, storageSelectionUUID.getScope(), SRFilterScopeHost)

	storageSelectionMigrating := storageSelectionFromParameters(&storageParameters{
		SRsWithTag: ptr.To("k8s-local"),
		Migrating:  true,
	})

	assert.Equal(t, storageSelectionMigrating.getScope(), SRFilterScopeGlobal)
}

func TestGetTopologyForVDI(t *testing.T) {
	storageSelectionTag := storageSelectionFromParameters(&storageParameters{
		SRsWithTag: ptr.To("k8s-local"),
	})

	assert.Equal(t, storageSelectionTag.getScope(), SRFilterScopeHost)
}
