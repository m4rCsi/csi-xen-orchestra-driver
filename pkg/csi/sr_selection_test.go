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
)

func TestPickSRForLocalMigrating(t *testing.T) {
	srs := []xoa.SR{
		{
			UUID:  "sr-on-host1",
			Usage: 100,
			Size:  1000,
			Host:  "host1",
			Pool:  "pool1",
		},
		{
			UUID:  "sr-on-host2",
			Usage: 200,
			Size:  1000,
			Host:  "host2",
			Pool:  "pool1",
		},
	}

	capacity := int64(100)
	accessibilityRequirements := &csi.TopologyRequirement{
		Requisite: []*csi.Topology{
			{
				Segments: map[string]string{"host": "host1", "pool": "pool1"},
			},
			{
				Segments: map[string]string{"host": "host2", "pool": "pool1"},
			},
		},
		Preferred: []*csi.Topology{
			{
				Segments: map[string]string{"host": "host2", "pool": "pool1"},
			},
		},
	}

	sr, err := pickSRForLocalMigrating(srs, capacity, accessibilityRequirements)
	if err != nil {
		t.Fatalf("failed to pick SR: %v", err)
	}

	assert.Equal(t, sr.UUID, "sr-on-host2")

}
