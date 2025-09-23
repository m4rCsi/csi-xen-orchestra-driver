# Driver configuration

This page covers controller runtime flags and other requirements for the driver to work. For StorageClass options, see [storage-class.md](storage-class.md).

## Authentication with Xen Orchestra

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

Official docs on how to create a token: https://docs.xcp-ng.org/management/manage-at-scale/xo-api/#authentication

CLI example to create the secret directly:

```sh
kubectl create secret generic csi-xen-orchestra-credentials \
  -n kube-system \
  --from-literal=url="https://xoa.example.lan" \
  --from-literal=token="<paste-your-xo-token>"
```

### Configure secret name with Helm

You can configure which secret the driver uses through Helm values:

```yaml
# values.yaml
xenOrchestra:
  # Secret name for Xen Orchestra credentials
  secretName: my-custom-xo-credentials
```

This allows you to use a different secret name than the default `csi-xen-orchestra-credentials`.

## Controller flags

All Helm configurations are under `csiXenOrchestraDriver.config`:

| Flag | Helm Configuration | Default | Type | What it does | When to use |
| --- | --- | --- | --- | --- | --- |
| `--disk-name-prefix` | `.diskNamePrefix` | `csi-` | string | Prefix added to all driver-managed disk names. Helps keep names unique across clusters sharing the same XO. | Multiple clusters using the same Xen Orchestra; clearer naming/segregation. |
| `--host-topology` | `.hostTopology` | `false` | bool | Enables host-level topology instead of pool-level topology. | See [Local Storage](local-storage.md) for details before enabling. |
| `--temp-cleanup` | `.tempCleanup` | `false` | bool | Enables background cleanup of leaked temporary disks (created as `csi-temp-...`). | If disk creation can exceed the provisioner timeout. See [Disk creation leakage](disk-creation-leakage.md). |
| `--xoa-timeout` | `.xoaTimeout` | `300s` | duration | timeout for calls to Xen Orchestra | If disk creation can take longer than `300s` increase this together with the provisioner timeout |

Notes:
- Temporary disks use the prefix `<disk-name-prefix>temp-` (e.g., `csi-temp-`).
- Cleanup targets only temporary-prefix disks; safe to enable broadly.

### Apply flags with Helm

Example using Helm values to configure the driver flags:

```yaml
# values.yaml
csiXenOrchestraDriver:
  config:
    tempCleanup: true
    diskNamePrefix: "csistaging-"
```

