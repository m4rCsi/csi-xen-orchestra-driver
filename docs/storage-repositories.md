# Storage Repositories

> [!TIP]
> See [XCP-ng Documentation: Storage in XCP-ng](https://docs.xcp-ng.org/storage/) for details about how Storage works in XCP-ng.

This driver can create and attach volumes in specific Xen Orchestra Storage Repositories (SRs). You select SRs through your StorageClass parameters in one of two ways:

- Exact SR selection with `srUUID`
- Set-based SR selection with `srsWithTag`

Both methods work with shared and with local SRs.

## Option 1: Select a single SR by UUID (`srUUID`)

Use this when you want every volume of a class to reside in one specific SR.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xoa-specified-sr
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  csi.storage.k8s.io/fstype: ext4
  srUUID: <sr-uuid>
volumeBindingMode: Immediate   # or WaitForFirstConsumer, see notes below
allowVolumeExpansion: true
```

Notes:
- Works for both shared and local SRs.
- Do not set `srsWithTag` or `migrating` when `srUUID` is set.
- For local SRs, consider `WaitForFirstConsumer` to align scheduling with where the SR is available.

## Option 2: Select from a set of SRs by tag (`srsWithTag`)

Tag all SRs in XO that should be eligible for this class. The driver will pick an SR among those tagged, based on topology and capacity. This also enables optional attach-time migration.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xoa-tagged-srs
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  csi.storage.k8s.io/fstype: ext4
  srsWithTag: k8s-general
  # migrating: "true"    # optional; allows attach-time migration between tagged SRs
volumeBindingMode: WaitForFirstConsumer   # good default for local; Immediate is fine for shared
allowVolumeExpansion: true
```

Notes:
- Works for both shared and local SRs.
- Do not set `srUUID` when `srsWithTag` is used.
- If you enable `migrating: "true"`, the driver may transparently move the disk between tagged SRs at attach time. See Disk migrations for details.

## Compatibility and constraints

- `srUUID` and `srsWithTag` are mutually exclusive.
- `migrating` cannot be combined with `srUUID`; use `srsWithTag` when `migrating` is enabled.
- All SRs under the same tag should share the same characteristic (all local or all shared).

## Related

- [StorageClass options](storage-class.md)
- [Disk migrations](disk-migrations.md)
