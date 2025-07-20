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


## TODO
- Add E2E Tests


## Development References
- https://kubernetes-csi.github.io/
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

```bash
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: xen-orchestra-storage
provisioner: csi.xen-orchestra.marcsi.ch
parameters:
  srUUID: 5e653748-9223-c319-7cb4-f6e20384de61 # this is the UUID of a Storage Repository
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

Supported Optional Parameters:
- `csi.storage.k8s.io/fstype`: `ext3`,`ext4`,`xfs` (default: `ext4`)
