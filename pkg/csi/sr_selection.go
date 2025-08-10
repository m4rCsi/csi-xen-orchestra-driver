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
	"context"
	"errors"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	xoa "github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
)

var (
	ErrNoSRFound         = errors.New("no SR found")
	ErrNoSpace           = errors.New("no SR has enough space for the disk")
	ErrInconsistentSRs   = errors.New("SRs have different shared values")
	ErrSRNotValidForHost = errors.New("SR is not valid for host")
)

type StorageRepositorySelection string

const (
	StorageRepositorySelectionTag  StorageRepositorySelection = "tag"
	StorageRepositorySelectionUUID StorageRepositorySelection = "uuid"
)

type SRFilterScope string

const (
	SRFilterScopeHost   SRFilterScope = "host"
	SRFilterScopePool   SRFilterScope = "pool"
	SRFilterScopeGlobal SRFilterScope = "global"
)

type storageSelection struct {
	// Inputs
	SRUUID               *string
	SRsWithTag           *string
	StorageSelectionType StorageRepositorySelection
	Migrating            bool

	// Current
	CurrentVDI *xoa.VDI
	CurrentSR  *xoa.SR

	// Available SRs
	SRs      []xoa.SR
	SharedSR bool
}

func storageSelectionFromParameters(parameters *storageParameters) *storageSelection {
	sr := &storageSelection{
		SRUUID:     parameters.SRUUID,
		SRsWithTag: parameters.SRsWithTag,
		Migrating:  parameters.Migrating,
	}

	if parameters.SRUUID != nil {
		sr.StorageSelectionType = StorageRepositorySelectionUUID
	} else if parameters.SRsWithTag != nil {
		sr.StorageSelectionType = StorageRepositorySelectionTag
	}

	return sr
}

func (s *storageSelection) VolumeIDType() VolumeIDType {
	if s.Migrating {
		return NameAsVolumeID
	}

	if s.SharedSR {
		return UUIDAsVolumeID
	}

	return NameAsVolumeID
}

func (s *storageSelection) toStorageInfo() *StorageInfo {
	si := &StorageInfo{
		SRsWithTag: s.SRsWithTag,
	}

	if s.Migrating {
		si.Migrating = &Migrating{}
	}

	return si
}

func storageSelectionFromStorageInfo(si *StorageInfo, vdi *xoa.VDI) *storageSelection {
	sr := &storageSelection{
		SRsWithTag: si.SRsWithTag,
		Migrating:  si.Migrating != nil,
		CurrentVDI: vdi,
	}

	if si.SRsWithTag != nil {
		sr.StorageSelectionType = StorageRepositorySelectionTag
	} else {
		sr.StorageSelectionType = StorageRepositorySelectionUUID
	}

	return sr
}

func (s *storageSelection) getScope() SRFilterScope {
	if s.Migrating {
		return SRFilterScopeGlobal
	}

	if s.SharedSR {
		return SRFilterScopePool
	}

	return SRFilterScopeHost
}

// finds relevant SRs based on the inforomation we are given. If we are also given a VDI, we will get the current SR from it.
func (s *storageSelection) findSRs(ctx context.Context, xoaClient xoa.Client) error {
	switch s.StorageSelectionType {
	case StorageRepositorySelectionUUID:
		if s.SRUUID == nil {
			if s.CurrentVDI != nil {
				s.SRUUID = &s.CurrentVDI.SR
			} else {
				return fmt.Errorf("SRUUID is required")
			}
		}

		sr, err := xoaClient.GetSRByUUID(ctx, *s.SRUUID)
		if errors.Is(err, xoa.ErrObjectNotFound) {
			return fmt.Errorf("SR with UUID %s not found", *s.SRUUID)
		} else if err != nil {
			return err
		}
		s.SRs = []xoa.SR{*sr}
		s.SharedSR = sr.Shared

		if s.CurrentVDI != nil {
			s.CurrentSR = sr
		}
		return nil
	case StorageRepositorySelectionTag:
		srs, err := xoaClient.GetSRsWithTag(ctx, *s.SRsWithTag)
		if err != nil {
			return err
		}
		if len(srs) == 0 {
			// TODO: find out what the right way to deal with this is.
			return fmt.Errorf("no SRs found with tag %s", *s.SRsWithTag)
		}
		s.SRs = srs
		s.SharedSR = srs[0].Shared

		// Check if all srs have the same value of shared
		for _, sr := range srs {
			if sr.Shared != srs[0].Shared {
				return fmt.Errorf("%w: SRs with tag %s have different shared values", ErrInconsistentSRs, *s.SRsWithTag)
			}

			if s.CurrentVDI != nil && sr.UUID == s.CurrentVDI.SR {
				s.CurrentSR = &sr
			}
		}

		if s.CurrentVDI != nil && s.CurrentSR == nil {
			// If we know the current VDI, and we haven't found it through the tag, we will look it up here via UUID
			rs, err := xoaClient.GetSRByUUID(ctx, s.CurrentVDI.SR)
			if err != nil {
				return err
			}
			s.CurrentSR = rs
		}

		return nil
	default:
		return fmt.Errorf("invalid storage selection type: %s", s.StorageSelectionType)
	}
}

// pickSRForTopology picks a SR for the given topology requirement
// this is used during the creation of the volume, when we need to pick a SR for the volume
func (s *storageSelection) pickSRForTopology(capacity int64, req *csi.TopologyRequirement) (*xoa.SR, error) {
	switch s.StorageSelectionType {
	case StorageRepositorySelectionUUID:
		return &s.SRs[0], nil
	case StorageRepositorySelectionTag:
		if !s.SharedSR {
			// Local SRs are scoped by `host`
			return pickSR(s.SRs, capacity, req, SRFilterScopeHost)
		} else {
			// Shared SRs are scoped by `pool`
			return pickSR(s.SRs, capacity, req, SRFilterScopePool)
		}
	default:
		return nil, fmt.Errorf("invalid storage selection type: %s", s.StorageSelectionType)
	}
}

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

func (s *storageSelection) needsMigration(vm *xoa.VM) (bool, error) {
	if s.CurrentSR == nil {
		return true, fmt.Errorf("no current SR found")
	}

	if s.isCurrentSRValidForHost(vm) {
		return false, nil
	}

	if s.Migrating {
		return true, nil
	}

	return true, fmt.Errorf("%w: current SR is not valid for host %s", ErrSRNotValidForHost, vm.Host)
}

func (s *storageSelection) isCurrentSRValidForHost(vm *xoa.VM) bool {
	if s.CurrentSR == nil {
		return false
	}
	switch s.CurrentSR.Shared {
	case true:
		return s.CurrentSR.Pool == vm.Pool
	case false:
		return s.CurrentSR.Host == vm.Host
	}
	return false
}

func (s *storageSelection) pickSRForHost(host string, capacity int64) (*xoa.SR, error) {
	filteredSrs := filterSrsForHost(s.SRs, host)
	return pickSrFromPool(filteredSrs, capacity)
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

func filterSrsByTopology(srs []xoa.SR, topologies []*csi.Topology, scope SRFilterScope) []xoa.SR {
	var filteredSrs []xoa.SR
	for _, topology := range topologies {
		for _, sr := range srs {
			if scope == SRFilterScopeHost {
				if sr.Host == topology.Segments["host"] && sr.Pool == topology.Segments["pool"] {
					filteredSrs = append(filteredSrs, sr)
				}
			} else if scope == SRFilterScopePool {
				if sr.Pool == topology.Segments["pool"] {
					filteredSrs = append(filteredSrs, sr)
				}
			}
		}
	}
	return filteredSrs
}

func pickSR(srs []xoa.SR, capacity int64, accessibilityRequirements *csi.TopologyRequirement, scope SRFilterScope) (*xoa.SR, error) {
	// If we have preferred topo, let's first try to find a SR that matches it
	var filteredPreferredSrs []xoa.SR
	if accessibilityRequirements != nil {
		var bestError = ErrNoSRFound
		filteredPreferredSrs = filterSrsByTopology(srs, accessibilityRequirements.Preferred, scope)
		bestSR, err := pickSrFromPool(filteredPreferredSrs, capacity)
		if errors.Is(err, ErrNoSRFound) {
			// continue
		} else if errors.Is(err, ErrNoSpace) {
			bestError = err
		} else {
			return bestSR, nil
		}

		filteredRequisiteSrs := filterSrsByTopology(srs, accessibilityRequirements.Requisite, scope)
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

	// Now, let's pick the best SR from the pool based on left over space
	return pickSrFromPool(srs, capacity)
}

func (s *storageSelection) getTopologyForVDI(vdi *xoa.VDI) ([]*csi.Topology, error) {
	var foundSR *xoa.SR
	for i, sr := range s.SRs {
		if sr.UUID == vdi.SR {
			foundSR = &s.SRs[i]
			break
		}
	}

	if foundSR == nil {
		// TODO: Consequences?
		return nil, fmt.Errorf("SR with UUID %s not found", vdi.SR)
	}

	switch s.getScope() {
	case SRFilterScopeGlobal:
		return nil, nil
	case SRFilterScopeHost:
		return []*csi.Topology{
			{
				Segments: map[string]string{
					"host": foundSR.Host,
					"pool": foundSR.Pool,
				},
			},
		}, nil
	case SRFilterScopePool:
		return []*csi.Topology{
			{
				Segments: map[string]string{
					"pool": foundSR.Pool,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("invalid scope: %s", s.getScope())
	}
}
