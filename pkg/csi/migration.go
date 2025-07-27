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
	"strings"
)

const MigrationPrefix = "csi:migration: "

type Migration struct {
	ToSRUUID string `json:"toSRUUID"`
}

func NewMigration(toSRUUID string) *Migration {
	return &Migration{
		ToSRUUID: toSRUUID,
	}
}

func FromVDIDescription(description string) (*Migration, error) {
	if !strings.HasPrefix(description, MigrationPrefix) {
		return nil, nil
	}
	var migration Migration
	err := json.Unmarshal([]byte(description[len(MigrationPrefix):]), &migration)
	if err != nil {
		return nil, err
	}
	return &migration, nil
}

func (m *Migration) ToVDIDescription() string {
	json, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return MigrationPrefix + string(json)
}

func (m *Migration) TargetSRUUID() string {
	return m.ToSRUUID
}
