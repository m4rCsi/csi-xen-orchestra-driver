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
  type: shared
  srUUID: <sr-uuid>
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

## Parameters

| Name | Required | Values | What it does |
| --- | --- | --- | --- |
| `type` | yes | `shared`, `localmigrating` | Selects the storage behavior/mode |
| `srUUID` | for `type=shared` | SR UUID string | The shared SR where volumes are created |
| `srsWithTag` | for `type=localmigrating` | tag string | Tag that selects eligible local SRs on each host |
| `csi.storage.k8s.io/fstype` | no (default `ext4`) | `ext4`, `xfs`, `ext3`  | Filesystem for the node mount |

For type details see:
- [Shared](type-shared.md)
- [Local-Migrating](type-localmigrating.md)


