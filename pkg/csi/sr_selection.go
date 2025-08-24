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

type VolumeScope string

const (
	VolumeScopeHost   VolumeScope = "host"
	VolumeScopePool   VolumeScope = "pool"
	VolumeScopeGlobal VolumeScope = "global"
)

// storageSelection is a construct that holds all the information we need to select a SR for a volume.
// it is initialized in two steps:
// 1. storageSelectionFromParameters  or  storageSelectionFromStorageInfo (depending on the context)
// 2. findSRs to populate the SRs and CurrentSR fields
type storageSelection struct {
	// Configuration
	hostTopology bool

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

// storageSelectionFromParameters creates a storageSelection from the parameters passed in the StorageClass
func storageSelectionFromParameters(parameters *storageParameters, hostTopology bool) *storageSelection {
	sr := &storageSelection{
		hostTopology: hostTopology,

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

// VolumeIDType returns the type of volume ID to use for the volume
// we can only use the UUID if we are not migrating and the SR is shared
// otherwise the UUID will change when we migrate.
func (s *storageSelection) VolumeIDType() VolumeIDType {
	if s.Migrating {
		return NameAsVolumeID
	}

	if s.SharedSR {
		return UUIDAsVolumeID
	}

	return NameAsVolumeID
}

// toStorageInfo creates a StorageInfo from the storageSelection
// this is used to store the storage selection in the volume context
func (s *storageSelection) toStorageInfo() *StorageInfo {
	si := &StorageInfo{
		SRsWithTag: s.SRsWithTag,
	}

	if s.Migrating {
		si.Migrating = &Migrating{}
	}

	return si
}

// storageSelectionFromStorageInfo creates a storageSelection from the StorageInfo
// this is used to restore the storage selection from the volume context
func storageSelectionFromStorageInfo(si *StorageInfo, vdi *xoa.VDI, hostTopology bool) *storageSelection {
	sr := &storageSelection{
		hostTopology: hostTopology,

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

func (s *storageSelection) getVolumeScope() VolumeScope {
	if s.Migrating {
		return VolumeScopeGlobal
	}

	if s.SharedSR {
		return VolumeScopePool
	}

	if s.hostTopology {
		return VolumeScopeHost
	}

	return VolumeScopePool
}

// findSRs populates the SRs and CurrentSR fields based on the information we have.
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
			return fmt.Errorf("%w: SR with UUID %s not found", ErrNoSRFound, *s.SRUUID)
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
			return fmt.Errorf("%w: no SRs found with tag %s", ErrNoSRFound, *s.SRsWithTag)
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
			return pickSR(s.SRs, capacity, req, VolumeScopeHost)
		} else {
			// Shared SRs are scoped by `pool`
			return pickSR(s.SRs, capacity, req, VolumeScopePool)
		}
	default:
		return nil, fmt.Errorf("invalid storage selection type: %s", s.StorageSelectionType)
	}
}

// needsMigration checks if we need to migrate the volume to a new SR
// this will be true if the VM we want to attach to, is on a different host than the current SR.
// this will only return true if the storage selection has migrating enabled.
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

	// Yes we do need to migrate, but migration is disabled
	return true, fmt.Errorf("%w: current SR is not valid for host %s", ErrSRNotValidForHost, vm.Host)
}

// isCurrentSRValidForHost checks if the current SR is valid for the given host
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
	return pickSRWithMostSpace(filteredSrs, capacity)
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

func pickSRWithMostSpace(srs []xoa.SR, capacity int64) (*xoa.SR, error) {
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

func filterSrsByTopology(srs []xoa.SR, topologies []*csi.Topology, scope VolumeScope) []xoa.SR {
	var filteredSrs []xoa.SR
	for _, topology := range topologies {
		for _, sr := range srs {
			switch scope {
			case VolumeScopeHost:
				if topology.Segments["host"] != "" && sr.Host != topology.Segments["host"] {
					continue
				}
				fallthrough
			case VolumeScopePool:
				if topology.Segments["pool"] != "" && sr.Pool != topology.Segments["pool"] {
					continue
				}
				fallthrough
			case VolumeScopeGlobal:
				// Global scope is not filtered by topology
				filteredSrs = append(filteredSrs, sr)
			}
		}
	}
	return filteredSrs
}

func pickSR(srs []xoa.SR, capacity int64, accessibilityRequirements *csi.TopologyRequirement, scope VolumeScope) (*xoa.SR, error) {
	// If topology requirement is defined, either preferred or requisite or both are defined.
	if accessibilityRequirements == nil {
		return pickSRWithMostSpace(srs, capacity)
	}

	if len(accessibilityRequirements.Preferred) > 0 {
		// If we have preferred topo, let's first try to find a SR that matches it
		filteredPreferredSrs := filterSrsByTopology(srs, accessibilityRequirements.Preferred, scope)
		bestSR, err := pickSRWithMostSpace(filteredPreferredSrs, capacity)
		if errors.Is(err, ErrNoSRFound) {
			// no SR found that matches the preferred topology
		} else if errors.Is(err, ErrNoSpace) {
			// no SR found that matches the preferred topology and has enough space
		} else {
			// we found a SR that matches the preferred topology
			return bestSR, nil
		}
	}

	// If requisite is not defined, we can pick a SR from all the SRs
	if len(accessibilityRequirements.Requisite) == 0 {
		return pickSRWithMostSpace(srs, capacity)
	}

	filteredRequisiteSrs := filterSrsByTopology(srs, accessibilityRequirements.Requisite, scope)
	bestSR, err := pickSRWithMostSpace(filteredRequisiteSrs, capacity)
	return bestSR, err

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
		return nil, fmt.Errorf("%w: SR with UUID %s not found", ErrNoSRFound, vdi.SR)
	}

	switch s.getVolumeScope() {
	case VolumeScopeGlobal:
		return nil, nil
	case VolumeScopeHost:
		return []*csi.Topology{
			{
				Segments: map[string]string{
					"host": foundSR.Host,
					"pool": foundSR.Pool,
				},
			},
		}, nil
	case VolumeScopePool:
		return []*csi.Topology{
			{
				Segments: map[string]string{
					"pool": foundSR.Pool,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("invalid scope: %s", s.getVolumeScope())
	}
}
