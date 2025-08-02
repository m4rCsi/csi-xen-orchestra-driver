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

### Type: shared

In this mode, the SR will needs to be accessible by all hypervisors that kubernetes nodes will be running on.
This should work with any `shared` storage types, such as `NFS` and `iSCSI` etc. However I have only been it able to test it with `NFS`.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xoa-shared
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  type: shared
  srUUID: 15607a67-58c5-ac9c-45ca-7c3659bdfa19
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```


### Type: localmigrating

In this mode, it is expected that different SRs will be available on the hypvervisors that kubernetes nodes will be running on.
The Storage Repositories that should be used need to be tagged in Xen Orchestra, and then can be selected with the `srsWithTag` attribute.
It is your responsability to make sure that on every hypvervisor there is at least one local storage repository tagged with the right tag.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xoa-localmigrating
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  type: localmigrating
  srsWithTag: k8s-localmigrating
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

### Static

It's possible to directly use an existing disk. It can be referenced through the `volumeHandle` in the form of the UUID of the disk.
Since topology is not supported, this will not always work. 
If the disk is on a shared Storage Repository (e.g. nfs), then it will check if the k8s-node and the SR is on the same pool.
If, however, the Storage Repistory is local to a hypervisor, the driver will check if the SR and the VM are on the same hypvervisor.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: test-static-pv
  namespace: csi-test
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 5Gi
  csi:
    driver: csi.xen-orchestra.marcsi.ch
    volumeHandle: fe852dc6-c0bf-45f4-b80f-938c01990ac3 # UUID of the Disk
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
  type: shared # or migrating
  srUUID: 5e653748-9223-c319-7cb4-f6e20384de61 # this is the UUID of a Storage Repository
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

Supported Parameters:
- `csi.storage.k8s.io/fstype`: `ext3`,`ext4`,`xfs` (default: `ext4`)
- `type`: `shared` or `localmigrating`
- `srUUID`: UUID of a Storage Repository (required for mode=`shared`)
- `srsWithTag`: which local Storage Repositories should be used (required for mode=`localmigrating`)


## Temp Cleanup

If the timeout of the creation of disks is set too low, it is possible for leakage to occur during the disk creation process.
For this reason the driver has a 2-step process when creating volumes and creates them under a temporary name.
This temporary name is prefixed with `csi-temp-`. 

There is a cleanup mechanism built into the controller, that will delete those leaked temporary disks. However, this is disabled by default and needs to be enabled with `--temp-cleanup=true`.