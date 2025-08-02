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
	MigrationPrefix         = "csi:migration: "
	DeletionCandidatePrefix = "csi:deletion-candidate: "
)

// EmbeddedVDIMetadata represents the different types of metadata that can be embedded in VDI descriptions
type EmbeddedVDIMetadata interface {
	ToVDIDescription() string
}

type Migration struct {
	ToSRUUID string `json:"toSRUUID"`
}

type DeletionCandidate struct {
	UnusedSince time.Time `json:"unusedSince"`
}

type NoMetadata struct{}

func (n *NoMetadata) ToVDIDescription() string {
	return ""
}

// Constructor functions
func NewMigration(toSRUUID string) *Migration {
	return &Migration{
		ToSRUUID: toSRUUID,
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
	case strings.HasPrefix(description, MigrationPrefix):
		return parseMigration(description)
	case strings.HasPrefix(description, DeletionCandidatePrefix):
		return parseDeletionCandidate(description)
	default:
		return &NoMetadata{}, nil
	}
}

func parseMigration(description string) (*Migration, error) {
	var migration Migration
	err := json.Unmarshal([]byte(description[len(MigrationPrefix):]), &migration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse migration: %w", err)
	}
	return &migration, nil
}

func parseDeletionCandidate(description string) (*DeletionCandidate, error) {
	var deletionCandidate DeletionCandidate
	err := json.Unmarshal([]byte(description[len(DeletionCandidatePrefix):]), &deletionCandidate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse deletion candidate: %w", err)
	}
	return &deletionCandidate, nil
}

// ToVDIDescription implementations
func (m *Migration) ToVDIDescription() string {
	json, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return MigrationPrefix + string(json)
}

func (d *DeletionCandidate) ToVDIDescription() string {
	json, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return DeletionCandidatePrefix + string(json)
}

// Helper methods
func (m *Migration) TargetSRUUID() string {
	return m.ToSRUUID
}

func (d *DeletionCandidate) GetUnusedSince() time.Time {
	return d.UnusedSince
}
