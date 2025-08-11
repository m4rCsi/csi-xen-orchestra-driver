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

## Controller flags

| Flag | Default | Type | What it does | When to use |
| --- | --- | --- | --- | --- |
| `--disk-name-prefix` | `csi-` | string | Prefix added to all driver-managed disk names. Helps keep names unique across clusters sharing the same XO. | Multiple clusters using the same Xen Orchestra; clearer naming/segregation. |
| `--temp-cleanup` | `false` | bool | Enables background cleanup of leaked temporary disks (created as `csi-temp-...`). | If disk creation can exceed the provisioner timeout. See [Disk creation leakage](disk-creation-leakage.md). |
| `--xoa-timeout`  | `300s`  | duration | timeout for calls to Xen Orchestra | If disk creation can take longer than `300s` increase this together with the provisioner timeout |

Notes:
- Temporary disks use the prefix `<disk-name-prefix>temp-` (e.g., `csi-temp-`).
- Cleanup targets only temporary-prefix disks; safe to enable broadly.

### Apply flags with Kustomize

Example using JSON6902 patches in a `kustomization.yaml` file:

```yaml
patches:
- target:
    kind: Deployment
    name: csi-xen-orchestra-controller
  patch: |-
    - op: add
      path: /spec/template/spec/containers/3/args/-
      value: "--temp-cleanup=true"
- target:
    kind: Deployment
    name: csi-xen-orchestra-controller
  patch: |-
    - op: add
      path: /spec/template/spec/containers/3/args/-
      value: "--disk-name-prefix=csistaging-"
```
