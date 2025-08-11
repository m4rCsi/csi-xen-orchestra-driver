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

func TestPickLocalSR(t *testing.T) {
	srs := []xoa.SR{
		{UUID: "sr-on-host1-pool1", Usage: 100, Size: 1000, Host: "host1", Pool: "pool1"},
		{UUID: "sr-on-host2-pool1", Usage: 200, Size: 1000, Host: "host2", Pool: "pool1"},
		{UUID: "sr-on-host3-pool1", Usage: 000, Size: 1000, Host: "host3", Pool: "pool1"},

		{UUID: "sr-on-host4-pool2", Usage: 000, Size: 1000, Host: "host1", Pool: "pool2"},
		{UUID: "sr-on-host5-pool2", Usage: 200, Size: 1000, Host: "host2", Pool: "pool2"},
	}

	testCases := []struct {
		// Test Name
		name string

		// Inputs
		requiredTopology *csi.TopologyRequirement
		volumeScope      VolumeScope // VolumeScopeHost, and VolumeScopeGlobal when migrating
		volumeCapacity   int64

		// Output
		expectedSRUUID string
		expectedError  error
	}{
		{
			name: "host-scoped: pick preferred",
			requiredTopology: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{Segments: map[string]string{"host": "host1", "pool": "pool1"}},
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
				Preferred: []*csi.Topology{
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
			},
			volumeScope:    VolumeScopeHost,
			volumeCapacity: 100,
			expectedSRUUID: "sr-on-host2-pool1",
		},
		{
			name: "host-scoped: preferred not enough space",
			requiredTopology: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{Segments: map[string]string{"host": "host1", "pool": "pool1"}},
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
				Preferred: []*csi.Topology{
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
			},
			volumeScope:    VolumeScopeHost,
			volumeCapacity: 850,
			expectedSRUUID: "sr-on-host1-pool1",
		},
		{
			name: "host-scoped: not enough space in topology",
			requiredTopology: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{Segments: map[string]string{"host": "host1", "pool": "pool1"}},
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
				Preferred: []*csi.Topology{
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
			},
			volumeScope:    VolumeScopeHost,
			volumeCapacity: 950,
			expectedError:  ErrNoSpace,
		},
		{
			name: "global-scoped: not enough space in topology in preferred or requisite",
			requiredTopology: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{Segments: map[string]string{"host": "host1", "pool": "pool1"}},
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
				Preferred: []*csi.Topology{
					{Segments: map[string]string{"host": "host2", "pool": "pool1"}},
				},
			},
			volumeScope:    VolumeScopeGlobal,
			volumeCapacity: 950,
			expectedSRUUID: "sr-on-host3-pool1",
		},
		// TODO: Add tests for more pool scoped and global scoped
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sr, err := pickSR(srs, testCase.volumeCapacity, testCase.requiredTopology, testCase.volumeScope)
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

	assert.Equal(t, storageSelectionTag.getVolumeScope(), VolumeScopeHost)

	storageSelectionUUID := storageSelectionFromParameters(&storageParameters{
		SRUUID: ptr.To("sr-on-host1"),
	})

	assert.Equal(t, storageSelectionUUID.getVolumeScope(), VolumeScopeHost)

	storageSelectionMigrating := storageSelectionFromParameters(&storageParameters{
		SRsWithTag: ptr.To("k8s-local"),
		Migrating:  true,
	})

	assert.Equal(t, storageSelectionMigrating.getVolumeScope(), VolumeScopeGlobal)
}

func TestGetTopologyForVDI(t *testing.T) {
	storageSelectionTag := storageSelectionFromParameters(&storageParameters{
		SRsWithTag: ptr.To("k8s-local"),
	})

	assert.Equal(t, storageSelectionTag.getVolumeScope(), VolumeScopeHost)
}
