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
	"time"

	"github.com/stretchr/testify/assert"
	ptr "k8s.io/utils/ptr"
)

func TestEmbeddedVDIMetadataFromDescription(t *testing.T) {
	metadata, err := EmbeddedVDIMetadataFromDescription(`csi:info: {}`)
	assert.NoError(t, err)
	assert.IsType(t, &StorageInfo{}, metadata)

	metadata, err = EmbeddedVDIMetadataFromDescription(`csi:deletion-candidate: {}`)
	assert.NoError(t, err)
	assert.IsType(t, &DeletionCandidate{}, metadata)

	metadata, err = EmbeddedVDIMetadataFromDescription(``)
	assert.NoError(t, err)
	assert.IsType(t, &NoMetadata{}, metadata)

	metadata, err = EmbeddedVDIMetadataFromDescription(`sdfsdfsf`)
	assert.NoError(t, err)
	assert.IsType(t, &NoMetadata{}, metadata)

	metadata, err = EmbeddedVDIMetadataFromDescription(`csi:info: sdfdsfsd`)
	assert.ErrorIs(t, err, ErrMalformedVDIMetadata)
	assert.IsType(t, &StorageInfo{}, metadata)
}

func TestDeletionCandidateMetadata(t *testing.T) {
	description := `csi:deletion-candidate: {"unusedSince":"2025-01-01T00:00:00Z"}`
	metadata, err := EmbeddedVDIMetadataFromDescription(description)
	assert.NoError(t, err)
	assert.IsType(t, &DeletionCandidate{}, metadata)
	assert.Equal(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), metadata.(*DeletionCandidate).GetUnusedSince())

	descriptionRoundtrip := metadata.ToVDIDescription()
	assert.Equal(t, description, descriptionRoundtrip)

	metadata, err = EmbeddedVDIMetadataFromDescription(`csi:deletion-candidate: {}`)
	assert.NoError(t, err)
	assert.IsType(t, &DeletionCandidate{}, metadata)

	metadata, err = EmbeddedVDIMetadataFromDescription(`csi:deletion-candidate: {"unusedSince": "sdfsdfsd"}`)
	assert.ErrorIs(t, err, ErrMalformedVDIMetadata)
	assert.IsType(t, &DeletionCandidate{}, metadata)
}

func TestStorageInfoMetadata(t *testing.T) {
	// Parse description
	description := `csi:info: {"migrating":{"enabled":true},"srsWithTag":"some-tag"}`
	metadata, err := EmbeddedVDIMetadataFromDescription(description)
	assert.NoError(t, err)
	assert.IsType(t, &StorageInfo{}, metadata)
	assert.Equal(t, "some-tag", *metadata.(*StorageInfo).SRsWithTag)
	ongoingMigration, toSRUUID := metadata.(*StorageInfo).HasOngoingMigration()
	assert.False(t, ongoingMigration)
	assert.Equal(t, "", toSRUUID)

	isMigrating, migrating := metadata.(*StorageInfo).IsMigratingType()
	assert.True(t, isMigrating)
	assert.Nil(t, migrating.InProgressToSRUUID)

	// Roundtrip should end up with the same description
	descriptionRoundtrip := metadata.ToVDIDescription()
	assert.Equal(t, description, descriptionRoundtrip)

	// Start migration
	migrating.StartMigration("sr-123")
	ongoingMigration, toSRUUID = metadata.(*StorageInfo).HasOngoingMigration()
	assert.True(t, ongoingMigration)
	assert.Equal(t, "sr-123", toSRUUID)

	descriptionWithMigration := metadata.ToVDIDescription()
	assert.Equal(t, `csi:info: {"migrating":{"enabled":true,"inProgressToSRUUID":"sr-123"},"srsWithTag":"some-tag"}`, descriptionWithMigration)

	// End migration
	migrating.EndMigration()
	ongoingMigration, toSRUUID = metadata.(*StorageInfo).HasOngoingMigration()
	assert.False(t, ongoingMigration)
	assert.Equal(t, "", toSRUUID)

	descriptionWithoutMigration := metadata.ToVDIDescription()
	assert.Equal(t, `csi:info: {"migrating":{"enabled":true},"srsWithTag":"some-tag"}`, descriptionWithoutMigration)
}

func TestStorageInfoMetadataCreation(t *testing.T) {
	metadata := NewStorageInfoWithMigrating(ptr.To("some-tag"))
	assert.True(t, *metadata.Migrating.Enabled)
	assert.Nil(t, metadata.Migrating.InProgressToSRUUID)

	description := metadata.ToVDIDescription()
	assert.Equal(t, `csi:info: {"migrating":{"enabled":true},"srsWithTag":"some-tag"}`, description)
}
