# csi-xen-orchestra-driver
CSI Driver for xen-ng managed via Xen Orchestra

Under active Development. Not ready to be used in anger!

## Requirements
- Kubernetes (TODO: Figure out min version)
  - Minimum based on on csi components: 1.20
  - Minimum based on deployment resources
  - Tested with 1.30
- Xen Orchestra (TODO: Figure out min version)
- xe-guest-utils running on nodes


## Features
- Dynamic Provisioning (creating disk on demand through PVC)
- Static Provisioning (by referencing UUID of Disk)
- Offline Volume Expansion (i.e. when not in use)

### Mode: shared

In this mode, the SR will needs to be accessibly by all hypervisors that kubernetes nodes will be running on.
This should work with any `shared` storage types, such as `NFS` and `iSCSI` etc. However I have only been it able to test it with `NFS`.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xen-orchestra-storage
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  type: shared
  srUUID: 15607a67-58c5-ac9c-45ca-7c3659bdfa19
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```


### Mode: migrating

In this mode, it is expected that different SRs will be available on the hypvervisors that kubernetes nodes will be running on.
Therefore `srUUIDs` needs to be populated with a comma seperated list of SR UUIDs, and it should span all hypvervisors where you will need this to work for.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xoa-migrating
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  type: migrating
  srUUIDs: 5e653748-9223-c319-7cb4-f6e20384de61,c17dba30-e17a-f349-6316-56f20478992e
volumeBindingMode: Immediate
allowVolumeExpansion: true
```



## Development References
- https://kubernetes-csi.github.io/
- https://github.com/container-storage-interface/spec/blob/master/spec.md
- https://arslan.io/2018/06/21/how-to-write-a-container-storage-interface-csi-plugin/


## Installation & Configuration

### Deployment

tbA Proper Deployment. This is how we do it during development

```bash
make deploy
```

### Authentication with Xen Orchestra

The driver needs access to the Xen Orchestra.
Create the following secret in the kube-system namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: csi-xen-orchestra-credentials
  namespace: kube-system
type: Opaque
stringData:
  url: https://xoa.example.lan
  token: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## Usage

### Install Storage Class

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xen-orchestra-storage
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  mode: shared # or migrating
  srUUID: 5e653748-9223-c319-7cb4-f6e20384de61 # this is the UUID of a Storage Repository
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

Supported Parameters:
- `csi.storage.k8s.io/fstype`: `ext3`,`ext4`,`xfs` (default: `ext4`)
- `mode`: `shared` or `migrating`
- `srUUID`: UUID of a Storage Repository (required for mode=`shared`)
- `srUUIDs`: UUIDs of all Storage Repositories (required for mode=`migrating`)
