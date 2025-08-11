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
	"errors"
	"fmt"

	"k8s.io/utils/ptr"
)

type StorageType string

var (
	ErrInvalidStorageParameters = errors.New("invalid storage parameters")
)

type storageParameters struct {
	// Specific Storage Repository
	// Type will be able to be deduced from the SRUUID (shared vs local)
	SRUUID *string

	// Selected Storage Repository/ies
	SRsWithTag *string

	// Allow the migration betwen SRs (with the same Tag).
	// Does not make sense if SRUUID is set
	Migrating bool
}

func LoadStorageParameters(parameters map[string]string) (*storageParameters, error) {
	storageParams := &storageParameters{}
	storageParams.Migrating = parameters["migrating"] == "true"
	if parameters["srUUID"] != "" {
		storageParams.SRUUID = ptr.To(parameters["srUUID"])
	}

	if parameters["srsWithTag"] != "" {
		storageParams.SRsWithTag = ptr.To(parameters["srsWithTag"])
	}

	if parameters["srUUID"] != "" && parameters["srsWithTag"] != "" {
		return nil, fmt.Errorf("%w: srUUID and srsWithTag cannot be set at the same time", ErrInvalidStorageParameters)
	}

	if storageParams.SRUUID != nil && storageParams.Migrating {
		return nil, fmt.Errorf("%w: srUUID and migrating cannot be set at the same time", ErrInvalidStorageParameters)
	}
	return storageParams, nil
}
