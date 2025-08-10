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
	"context"
	"time"

	xoa "github.com/m4rCsi/csi-xen-orchestra-driver/pkg/xoa"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

const (
	TempCleanupInterval  = 5 * time.Minute
	TempCleanupThreshold = 30 * time.Minute
)

type TempCleanup struct {
	xoaClient xoa.Client
	timer     *time.Ticker
	ctx       context.Context
	cancel    context.CancelFunc

	diskNameGenerator *DiskNameGenerator
	creationLock      *CreationLock
}

func NewTempCleanup(xoaClient xoa.Client, diskNameGenerator *DiskNameGenerator, creationLock *CreationLock) *TempCleanup {
	ctx, cancel := context.WithCancel(context.Background())
	return &TempCleanup{
		xoaClient:         xoaClient,
		ctx:               ctx,
		cancel:            cancel,
		diskNameGenerator: diskNameGenerator,
		creationLock:      creationLock,
	}
}

func (t *TempCleanup) Run() error {
	// Run cleanup every 5 minutes
	t.timer = time.NewTicker(TempCleanupInterval)
	defer t.timer.Stop()

	for {
		select {
		case <-t.timer.C:
			ctx, cancel := context.WithTimeout(t.ctx, 1*time.Minute)
			defer cancel()
			t.cleanup(ctx)
		case <-t.ctx.Done():
			// Context was cancelled, exit gracefully
			return nil
		}
	}
}

func (t *TempCleanup) Stop() {
	if t.timer != nil {
		t.timer.Stop()
	}
	t.cancel()

}

// cleanup process.
// Deletion happens when:
// - they are (still) prefixed with VDIDiskPrefixTemporary
// - they have the deletion candidate description
// Therefore, as long as the creation of a disk is faster than the TempCleanupThreshold, we can safely delete the disk.
// The main risk here is that the VDIDiskPrefixTemporary is selecting disks that are not managed by this driver.
func (t *TempCleanup) cleanup(ctx context.Context) {
	klog.InfoS("Starting temp cleanup")
	vdis, err := t.xoaClient.GetVDIs(ctx, nil)
	if err != nil {
		klog.Errorf("Failed to get VDIs: %v", err)
		return
	}

	for i, vdi := range vdis {
		if !t.diskNameGenerator.IsTemporaryDisk(vdi.NameLabel) {
			continue
		}
		klog.InfoS("Processing temporary VDI", "vdi", vdi.NameLabel)

		metadata, err := EmbeddedVDIMetadataFromDescription(vdi.NameDescription)
		if err != nil {
			klog.Errorf("Failed to get VDI metadata: %v", err)
			continue
		}

		switch metadata := metadata.(type) {
		case *NoMetadata:
			// No metadata, so we should consider to mark this VDI as a deletion candidate.
			t.setDeletionCandidate(ctx, &vdis[i])
		case *DeletionCandidate:
			if metadata.GetUnusedSince().Before(time.Now().Add(-1 * TempCleanupThreshold)) {
				klog.InfoS("Deletion Candiate is old enough, deleting VDI", "vdi", vdi.NameLabel)
				err = t.xoaClient.DeleteVDI(ctx, vdi.UUID)
				if err != nil {
					klog.Errorf("Failed to delete VDI: %v", err)
				}
			} else {
				klog.InfoS("Deletion Candiate is not old enough, skipping", "vdi", vdi.NameLabel)
			}
		case *StorageInfo:
			// This should not be on a temporary disk. We simply ignore it.
		}
	}
}

func (t *TempCleanup) setDeletionCandidate(ctx context.Context, vdi *xoa.VDI) {
	t.creationLock.DeletionLock()
	defer t.creationLock.DeletionUnlock()

	refetchedVdi, err := t.xoaClient.GetVDIByUUID(ctx, vdi.UUID)
	if err != nil {
		klog.Errorf("Failed to get VDI: %v", err)
		return
	}

	if refetchedVdi.NameDescription != vdi.NameDescription {
		klog.InfoS("VDI Description has been modified, skipping", "vdi", vdi.NameLabel)
		return
	}

	if refetchedVdi.NameLabel != vdi.NameLabel {
		klog.InfoS("VDI has been renamed, skipping", "vdi", vdi.NameLabel)
		return
	}

	klog.InfoS("No metadata found, setting deletion candidate", "vdi", vdi.NameLabel)
	deletionCandidate := NewDeletionCandidate(time.Now())
	err = t.xoaClient.EditVDI(ctx, vdi.UUID, nil, ptr.To(deletionCandidate.ToVDIDescription()))
	if err != nil {
		klog.Errorf("Failed to set VDI description: %v", err)
		return
	}
}
