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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMalformedVDIMetadata = errors.New("malformed VDI metadata")
	ErrInvalidVDIMetadata   = errors.New("invalid VDI metadata")
)

const (
	CSIStorageInfoPrefix              = "csi:info: "
	CSIStorageDeletionCandidatePrefix = "csi:deletion-candidate: "
)

// EmbeddedVDIMetadata represents the different types of metadata that can be embedded in VDI descriptions
type EmbeddedVDIMetadata interface {
	ToVDIDescription() string
}

type StorageInfo struct {
	Migrating  *Migrating `json:"migrating,omitempty"`  // If migration feature is enabled this is not nil
	SRsWithTag *string    `json:"srsWithTag,omitempty"` // If storage selection is done through tags this is not nil
}

type Migrating struct {
	Enabled            bool    `json:"enabled"`                      // always true
	InProgressToSRUUID *string `json:"inProgressToSRUUID,omitempty"` // if not nil, a migration is in progress
}

type DeletionCandidate struct {
	UnusedSince time.Time `json:"unusedSince"`
}

type NoMetadata struct{}

func (n *NoMetadata) ToVDIDescription() string {
	return ""
}

func NewStorageInfoWithMigrating(srsWithTag string) *StorageInfo {
	return &StorageInfo{
		Migrating: &Migrating{
			Enabled: true,
		},
	}
}

func NewDeletionCandidate(unusedSince time.Time) *DeletionCandidate {
	return &DeletionCandidate{
		UnusedSince: unusedSince,
	}
}

// FromVDIDescription parses a VDI description and returns the appropriate type
// Returns nil if no recognized type is found
func EmbeddedVDIMetadataFromDescription(description string) (EmbeddedVDIMetadata, error) {
	switch {
	case strings.HasPrefix(description, CSIStorageInfoPrefix):
		return parseStorageInfo(description)
	case strings.HasPrefix(description, CSIStorageDeletionCandidatePrefix):
		return parseDeletionCandidate(description)
	default:
		return &NoMetadata{}, nil
	}
}

func parseStorageInfo(description string) (*StorageInfo, error) {
	var storageInfo StorageInfo
	err := json.Unmarshal([]byte(description[len(CSIStorageInfoPrefix):]), &storageInfo)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedVDIMetadata, err)
	}

	if storageInfo.Migrating != nil {
		if storageInfo.SRsWithTag == nil {
			return nil, fmt.Errorf("%w: migrating storage info has no SRs with tag", ErrInvalidVDIMetadata)
		}
	}

	return &storageInfo, nil
}

func parseDeletionCandidate(description string) (*DeletionCandidate, error) {
	var deletionCandidate DeletionCandidate
	err := json.Unmarshal([]byte(description[len(CSIStorageDeletionCandidatePrefix):]), &deletionCandidate)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedVDIMetadata, err)
	}
	return &deletionCandidate, nil
}

// ToVDIDescription implementations
func (s *StorageInfo) ToVDIDescription() string {
	json, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return CSIStorageInfoPrefix + string(json)
}

func (d *DeletionCandidate) ToVDIDescription() string {
	json, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return CSIStorageDeletionCandidatePrefix + string(json)
}

func (d *DeletionCandidate) GetUnusedSince() time.Time {
	return d.UnusedSince
}

func (s *StorageInfo) HasOngoingMigration() (bool, string) {
	if s.Migrating == nil {
		return false, ""
	}

	if s.Migrating.InProgressToSRUUID == nil {
		return false, ""
	}

	return true, *s.Migrating.InProgressToSRUUID
}

func (s *StorageInfo) IsMigrating() (bool, *Migrating) {
	if s.Migrating == nil {
		return false, nil
	}

	return true, s.Migrating
}

func (s *Migrating) StartMigration(srUUID string) {
	s.InProgressToSRUUID = &srUUID
}

func (s *Migrating) EndMigration() {
	s.InProgressToSRUUID = nil
}
