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
// See the specific language governing permissions and
// limitations under the License.

package csi

import (
	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"
	xoa "github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
)

var (
	ErrNoSRFound = errors.New("no SR found")
	ErrNoSpace   = errors.New("no SR has enough space for the disk")
)

func pickSrFromPool(srs []xoa.SR, capacity int64) (*xoa.SR, error) {
	if len(srs) == 0 {
		return nil, ErrNoSRFound
	}

	bestSR := srs[0]
	for _, sr := range srs {
		if sr.Usage < bestSR.Usage {
			bestSR = sr
		}
	}

	// Check if the best SR has enough space for the disk
	if bestSR.Usage+capacity > bestSR.Size {
		return nil, ErrNoSpace
	}

	return &bestSR, nil
}

func filterSrsForHost(srs []xoa.SR, host string) []xoa.SR {
	filteredSrs := make([]xoa.SR, 0)
	for _, sr := range srs {
		if sr.Host == host {
			filteredSrs = append(filteredSrs, sr)
		}
	}
	return filteredSrs
}

func filterSrsByTopology(srs []xoa.SR, topologies []*csi.Topology) []xoa.SR {
	var filteredSrs []xoa.SR
	for _, topology := range topologies {
		for _, sr := range srs {
			if sr.Host == topology.Segments["host"] && sr.Pool == topology.Segments["pool"] {
				filteredSrs = append(filteredSrs, sr)
			}
		}
	}
	return filteredSrs
}

func pickSRForLocalMigrating(srs []xoa.SR, capacity int64, accessibilityRequirements *csi.TopologyRequirement) (*xoa.SR, error) {
	// If we have preferred topo, let's first try to find a SR that matches it
	var filteredPreferredSrs []xoa.SR
	if accessibilityRequirements != nil {
		var bestError = ErrNoSRFound
		filteredPreferredSrs = filterSrsByTopology(srs, accessibilityRequirements.Preferred)
		bestSR, err := pickSrFromPool(filteredPreferredSrs, capacity)
		if errors.Is(err, ErrNoSRFound) {
			// continue
		} else if errors.Is(err, ErrNoSpace) {
			bestError = err
		} else {
			return bestSR, nil
		}

		filteredRequisiteSrs := filterSrsByTopology(srs, accessibilityRequirements.Requisite)
		bestSR, err = pickSrFromPool(filteredRequisiteSrs, capacity)
		if errors.Is(err, ErrNoSRFound) {
			// continue
		} else if errors.Is(err, ErrNoSpace) {
			bestError = err
		} else {
			return bestSR, nil
		}
		return nil, bestError
	}

	// If we don't have a preferred topology, let's try to find a SR that matches the requisite topology
	return pickSrFromPool(srs, capacity)
}
