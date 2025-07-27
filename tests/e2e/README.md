# E2E Tests

This will be disruptive, so make sure not to run against a used cluster.

## Coverage and Notes

I am running this against a Talos cluster, and some tests don't seem to be working, because there are some requirements necessary that need to installed on each node.

## Requirements

* Existing Testing Cluster that is the default context in kubeconfig
* Driver deployed (no storageclass necessary)

## Run

```bash
make test
```