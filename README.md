# csi-xen-orchestra-driver
CSI Driver for xen-ng managed via Xen Orchestra


## Requirements
- Kubernetes (TODO: Figure out min version)
- Xen Orchestra (TODO: Figure out min version) with Access


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
