# csi-xen-orchestra-driver

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

CSI Driver for Xen-orchestra-managed Xen-ng environments

> ⚠️ Under active development. Not production-ready yet.

![Attached Disks](docs/assets/xoa-attached-disks.png)

## Requirements
- Xen Orchestra (XO)
- Kubernetes Cluster
- Kubernetes Nodes: 
  - `xe-guest-utils` installed

## Compatibility

Expecting much broader compatibility, but for now only tested on 1.30 (with Talos).

| Driver | Kubernetes |
| --- | --- |
| dev (main) | 1.30 (tested) |

## Features
- Dynamic provisioning (create disks on demand via PVCs)
- Migration of local disks between hypvervisors (see: [Type: Local-Migrating](docs/type-localmigrating.md))
- Static provisioning (use an existing VDI by UUID)
- Offline volume expansion

## Documentation

Start here
- [Getting started](docs/getting-started.md)

How to use the driver
- [Type: Shared](docs/type-shared.md)
- [Type: Local-Migrating](docs/type-localmigrating.md)
- [Static volumes (pre-existing VDI)](docs/static.md)

Configuration
- [StorageClass parameters](docs/storage-class.md)
- [Driver configuration (flags, authentication)](docs/driver-configuration.md)

Operational notes
- [Disk creation leakage and mitigations](docs/disk-creation-leakage.md)

Development
- [Development](DEVELOPMENT.md)


