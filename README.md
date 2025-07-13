# csi-xen-orchestra-driver
CSI Driver for xen-ng managed via Xen Orchestra


## Requirements
- Kubernetes (TODO: Figure out min version)
- Xen Orchestra (TODO: Figure out min version) with Access


## Features
- Dynamic Provisioning (creating disk on demand through PVC)
- Static Provisioning (by referencing UUID of Disk)


## TODO
- Investigate if Resize Disk is possible
- Add E2E Tests


## Development References
- https://kubernetes-csi.github.io/


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
```

Supported Optional Parameters:
- `csi.storage.k8s.io/fstype`: `ext4` (default) or `xfs`
