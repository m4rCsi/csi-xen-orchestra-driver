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
	"fmt"
	"strings"
	"time"
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
	Migrating  *Migrating `json:"migrating,omitempty"`
	SRsWithTag *string    `json:"srsWithTag,omitempty"`
}

type Migrating struct {
	OngoingMigrationToSRUUID *string `json:"toSRUUID,omitempty"`
}

type DeletionCandidate struct {
	UnusedSince time.Time `json:"unusedSince"`
}

type NoMetadata struct{}

func (n *NoMetadata) ToVDIDescription() string {
	return ""
}

// Constructor functions
func NewStorageInfoWithMigrating(srsWithTag string) *StorageInfo {
	return &StorageInfo{
		Migrating: &Migrating{},
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
		return nil, fmt.Errorf("failed to parse storage info: %w", err)
	}

	if storageInfo.Migrating != nil {
		if storageInfo.SRsWithTag == nil {
			return nil, fmt.Errorf("migrating storage info has no SRs with tag")
		}
	}

	return &storageInfo, nil
}

func parseDeletionCandidate(description string) (*DeletionCandidate, error) {
	var deletionCandidate DeletionCandidate
	err := json.Unmarshal([]byte(description[len(CSIStorageDeletionCandidatePrefix):]), &deletionCandidate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse deletion candidate: %w", err)
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

	if s.Migrating.OngoingMigrationToSRUUID == nil {
		return false, ""
	}

	return true, *s.Migrating.OngoingMigrationToSRUUID
}

func (s *StorageInfo) IsMigrating() (bool, *Migrating) {
	if s.Migrating == nil {
		return false, nil
	}

	return true, s.Migrating
}

func (s *Migrating) StartMigration(srUUID string) {
	s.OngoingMigrationToSRUUID = &srUUID
}

func (s *Migrating) EndMigration() {
	s.OngoingMigrationToSRUUID = nil
}
