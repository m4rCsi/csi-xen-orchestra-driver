# StorageClass configuration

Provisioner: `csi.xen-orchestra.marcsi.ch`

## Example:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xoa-shared
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  srUUID: <sr-uuid>
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

## Parameters

| Name | Required | Values | What it does |
| --- | --- | --- | --- |
| `srUUID` | no (1) | SR UUID string | selected Storage Repository |
| `srsWithTag` | no (1) | tag string | selected Storage Repositories |
| `migrating` | no (default: `false`) | `true`/`false` | If migrations are enabled (see [Disk Migrations](disk-migrations.md)) |
| `csi.storage.k8s.io/fstype` | no (default `ext4`) | `ext4`, `xfs`, `ext3`  | Filesystem for the node mount |


- (1) either `srUUID` or `srsWithTag` needs to be set. (see [Storage Repositories](storage-repositories.md)).



