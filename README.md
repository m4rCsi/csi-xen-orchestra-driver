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

## Documentation

Start here:
- [Getting started](docs/getting-started.md)

How to use the driver:
- [Type: Shared](docs/type-shared.md)
- [Type: Local-Migrating](docs/type-localmigrating.md)
- [Static volumes (pre-existing VDI)](docs/static.md)

Configuration:
- [StorageClass parameters](docs/storage-class.md)
- [Driver configuration (flags, auth)](docs/driver-configuration.md)

Operational notes:
- [Disk creation leakage and mitigations](docs/disk-creation-leakage.md)

Development:
- [Development](DEVELOPMENT.md)


