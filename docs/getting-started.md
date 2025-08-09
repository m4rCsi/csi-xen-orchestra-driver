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
kubectl apply -k ./deploy/kustomize/overlays/dev/
```

See [Driver Configuration](./driver-configuration.md) for configuration options.


## Example

tbA