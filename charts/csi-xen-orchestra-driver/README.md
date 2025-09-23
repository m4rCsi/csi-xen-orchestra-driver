# CSI Xen Orchestra Driver Helm Chart

This Helm chart deploys the CSI Xen Orchestra Driver, a Container Storage Interface driver for Xen Orchestra that enables dynamic provisioning and management of storage volumes in Kubernetes.

- GitHub Issues: [csi-xen-orchestra-driver](https://github.com/m4rcsi/csi-xen-orchestra-driver)
- Documentation: [docs/](https://github.com/m4rcsi/csi-xen-orchestra-driver/tree/main/docs)

## Install

You have two options:

**Option A: Let the chart create the secret (simplest)**
```bash
helm install csi-xen-orchestra oci://ghcr.io/m4rcsi/charts/csi-xen-orchestra-driver \
  --namespace kube-system \
  --set xenOrchestra.createSecret=true \
  --set xenOrchestra.url="https://your-xoa.example.com" \
  --set xenOrchestra.token="your-api-token"
```

**Option B: Create the secret manually first, then install (recommended for production)**
```bash
# Create the secret first
kubectl create secret generic csi-xen-orchestra-credentials \
  --namespace kube-system \
  --from-literal=url="https://your-xoa.example.com" \
  --from-literal=token="your-api-token"

# Then install the chart
helm install csi-xen-orchestra oci://ghcr.io/m4rcsi/charts/csi-xen-orchestra-driver \
  --namespace kube-system 
```

## Configuration

### Xen Orchestra Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `xenOrchestra.createSecret` | Whether to create the Xen Orchestra credentials secret | `false` |
| `xenOrchestra.secretName` | Name of the secret containing Xen Orchestra credentials | `csi-xen-orchestra-credentials` |
| `xenOrchestra.url` | Xen Orchestra URL (only used if createSecret is true) | `""` |
| `xenOrchestra.token` | Xen Orchestra API token (only used if createSecret is true) | `""` |

### Controller Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.enabled` | Whether to enable the controller deployment | `true` |
| `controller.replicas` | Number of controller replicas | `1` |
| `controller.annotations` | Annotations to add to the controller deployment | `{}` |
| `controller.labels` | Labels to add to the controller deployment | `{}` |
| `controller.nodeSelector` | Node selector for controller pods | `{}` |
| `controller.tolerations` | Tolerations for controller pods | `[]` |
| `controller.affinity` | Affinity for controller pods | `{}` |
| `controller.podSecurityContext` | Pod security context for controller pods | `{}` |
| `controller.securityContext` | Container security context for controller containers | `{}` |
| `controller.resources` | Resource requests/limits for controller containers | `{}` |

#### CSI Sidecar Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.csiProvisioner.image.repository` | CSI Provisioner image repository | `registry.k8s.io/sig-storage/csi-provisioner` |
| `controller.csiProvisioner.image.tag` | CSI Provisioner image tag | `v5.3.0` |
| `controller.csiProvisioner.image.pullPolicy` | CSI Provisioner image pull policy | `IfNotPresent` |
| `controller.csiProvisioner.verbosity` | CSI Provisioner verbosity level (0-5) | `0` |
| `controller.csiProvisioner.timeout` | CSI Provisioner operation timeout | `300s` |
| `controller.csiProvisioner.resources` | CSI Provisioner resource limits | `{}` |
| `controller.csiAttacher.image.repository` | CSI Attacher image repository | `registry.k8s.io/sig-storage/csi-attacher` |
| `controller.csiAttacher.image.tag` | CSI Attacher image tag | `v4.9.0` |
| `controller.csiAttacher.image.pullPolicy` | CSI Attacher image pull policy | `IfNotPresent` |
| `controller.csiAttacher.verbosity` | CSI Attacher verbosity level (0-5) | `0` |
| `controller.csiAttacher.timeout` | CSI Attacher operation timeout | `300s` |
| `controller.csiAttacher.resources` | CSI Attacher resource limits | `{}` |
| `controller.csiResizer.image.repository` | CSI Resizer image repository | `registry.k8s.io/sig-storage/csi-resizer` |
| `controller.csiResizer.image.tag` | CSI Resizer image tag | `v1.13.2` |
| `controller.csiResizer.image.pullPolicy` | CSI Resizer image pull policy | `IfNotPresent` |
| `controller.csiResizer.verbosity` | CSI Resizer verbosity level (0-5) | `0` |
| `controller.csiResizer.timeout` | CSI Resizer operation timeout | `120s` |
| `controller.csiResizer.resources` | CSI Resizer resource limits | `{}` |

**Note**: CSI sidecar standard arguments are hardcoded for consistency. Images, verbosity, timeouts, and resources are configurable.

#### Driver Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `csiXenOrchestraDriver.image.repository` | Driver image repository | `ghcr.io/m4rcsi/csi-xen-orchestra-driver` |
| `csiXenOrchestraDriver.image.tag` | Driver image tag (leave empty to use Chart.AppVersion) | `""` |
| `csiXenOrchestraDriver.image.digest` | Driver image digest (takes precedence over tag and appVersion) | `""` |
| `csiXenOrchestraDriver.image.pullPolicy` | Driver image pull policy | `IfNotPresent` |
| `csiXenOrchestraDriver.config.diskNamePrefix` | Prefix for all driver-managed disks | `csi-` |
| `csiXenOrchestraDriver.config.hostTopology` | Enable host-level topology | `false` |
| `csiXenOrchestraDriver.config.tempCleanup` | Enable temporary disk cleanup | `false` |
| `csiXenOrchestraDriver.config.xoaTimeout` | Xen Orchestra API timeout | `300s` |
| `controller.csiXenOrchestraDriver.verbosity` | Controller driver verbosity level (0-5) | `0` |
| `node.csiXenOrchestraDriver.verbosity` | Node driver verbosity level (0-5) | `0` |

### Node Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `node.enabled` | Whether to enable the node daemon set | `true` |
| `node.annotations` | Annotations to add to the node daemon set | `{}` |
| `node.labels` | Labels to add to the node daemon set | `{}` |
| `node.nodeSelector` | Node selector for node pods | `{}` |
| `node.tolerations` | Tolerations for node pods | `[]` |
| `node.affinity` | Affinity for node pods | `{}` |
| `node.podSecurityContext` | Pod security context for node pods | `{}` |
| `node.resources` | Resource requests/limits for node containers | `{}` |

#### CSI Node Driver Registrar

| Parameter | Description | Default |
|-----------|-------------|---------|
| `node.csiDriverRegistrar.image.repository` | Node Driver Registrar image repository | `registry.k8s.io/sig-storage/csi-node-driver-registrar` |
| `node.csiDriverRegistrar.image.tag` | Node Driver Registrar image tag | `v2.14.0` |
| `node.csiDriverRegistrar.image.pullPolicy` | Node Driver Registrar image pull policy | `IfNotPresent` |
| `node.csiDriverRegistrar.verbosity` | Node Driver Registrar verbosity level (0-5) | `0` |
| `node.csiDriverRegistrar.resources` | Node Driver Registrar resource limits | `{}` |

**Note**: CSI Node Driver Registrar standard arguments are hardcoded for consistency. Images, verbosity, and resources are configurable.

### RBAC Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Whether to create RBAC resources | `true` |
| `rbac.annotations` | Annotations to add to RBAC resources | `{}` |

### CSI Driver Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `csiDriver.create` | Whether to create the CSIDriver resource | `true` |
| `csiDriver.annotations` | Annotations to add to the CSIDriver | `{}` |

### Service Account Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Whether to create a service account | `true` |
| `serviceAccount.name` | Name of the service account to use | `""` |
| `serviceAccount.annotations` | Annotations to add to the service account | `{}` |

### Global Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.imagePullSecrets` | Global image pull secrets | `[]` |
| `global.nameOverride` | Override the chart name | `""` |
| `global.fullnameOverride` | Override the full name | `""` |

## Advanced Configuration

### Resource Limits

The chart includes an example resource configuration file to help you get started:

- **`values-resources.yaml`**: Recommended starting resources
  - Resource limits and requests for all containers

Set resource limits for containers:

```yaml
controller:
  csiXenOrchestraDriver:
    resources:
      limits:
        cpu: 500m
        memory: 512Mi
      requests:
        cpu: 100m
        memory: 128Mi
```

### Node Selectors and Tolerations

```yaml
controller:
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
    - key: node-role.kubernetes.io/control-plane
      operator: Exists
      effect: NoSchedule
```

## Storage Classes

After installing the driver, you can create StorageClass resources to use it. See the examples in the main repository for StorageClass configurations.
