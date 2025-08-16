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

import "sync"

// CreationLock is a lock that is used to protect the creation of VDIs and marking of deletion candidates.
// There is a possible race condition where a VDI is created, not yet renamed.
// The temp cleanup service will mark it as a deletion candidate, but the creation service will not see it.
// This lock is used to protect against this race condition.

// Many Creations can happen in parallel.
// But only one mark for deletion can happen at a time.
// This is why we use a RWMutex for the creation lock.
// This is not the most efficient way to do this, but it is the simplest (or at least one way to implement this).
type CreationLock struct {
	lock sync.RWMutex
}

func NewCreationLock() *CreationLock {
	return &CreationLock{}
}

func (c *CreationLock) CreationLock() {
	c.lock.RLock()
}

func (c *CreationLock) CreationUnlock() {
	c.lock.RUnlock()
}

func (c *CreationLock) DeletionLock() {
	c.lock.Lock()
}

func (c *CreationLock) DeletionUnlock() {
	c.lock.Unlock()
}
