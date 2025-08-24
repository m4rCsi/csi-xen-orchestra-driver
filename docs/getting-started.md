# Getting Started

## Create a Xen Orchestra Token

See official docs: https://docs.xcp-ng.org/management/manage-at-scale/xo-api/#authentication

Create the secret in `kube-system` namespace:

```sh
kubectl create secret generic csi-xen-orchestra-credentials \
  -n kube-system \
  --from-literal=url="https://xoa.example.lan" \
  --from-literal=token="<paste-your-xo-token>"
```


## Install Driver

```sh
# From Last Release
kubectl apply -f https://github.com/m4rCsi/csi-xen-orchestra-driver/releases/download/v0.2.0/driver.yaml
```

See [Driver Configuration](./driver-configuration.md) for configuration options.


## Example

Tag Storage Repositories with `k8s-shared`  (or `k8s-local`).

```bash
kubectl apply -f https://raw.githubusercontent.com/m4rCsi/csi-xen-orchestra-driver/refs/heads/main/examples/sc-shared.yaml  

# or 
kubectl apply -f https://raw.githubusercontent.com/m4rCsi/csi-xen-orchestra-driver/refs/heads/main/examples/sc-localmigrating.yaml
```

