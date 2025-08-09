# Type: Shared

Shared storage is for environments where a single Storage Repository (SR) is accessible from all hypervisors that your Kubernetes worker nodes may run on. Typical SR types: NFS, iSCSI, or any SR configured as shared in Xen Orchestra.

## Prerequisites

- One SR that is marked/shared across all relevant hosts in your pool.
- The SR must have sufficient capacity for your workloads.
- You know the SR UUID (visible in XO).

## StorageClass

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xoa-shared
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  csi.storage.k8s.io/fstype: ext4
  type: shared
  srUUID: <your-shared-sr-uuid>
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

Notes:
- Required parameter: `srUUID` (the shared SR to use for all volumes of this class)

