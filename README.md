# csi-xen-orchestra-driver

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![CI](https://github.com/m4rcsi/csi-xen-orchestra-driver/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/m4rcsi/csi-xen-orchestra-driver/actions/workflows/ci.yml)

CSI Driver for Xen-orchestra-managed Xen-ng environments

> ⚠️ Under active development. Not production-ready yet.

![Attached Disks](docs/assets/xoa-attached-disks.png)

## Requirements
- Xen Orchestra (XO)
- Kubernetes Cluster
    - With [xenorchestra-cloud-controller-manager](https://github.com/vatesfr/xenorchestra-cloud-controller-manager/tree/main)
- Kubernetes Nodes: 
  - `xe-guest-utils` installed

## Compatibility

Expecting much broader compatibility, but for now only tested on 1.30 (with Talos).

| Driver | Kubernetes |
| --- | --- |
| dev (main) | 1.30 (tested) |

## Features
- Dynamic provisioning (create disks on demand via PVCs)
- Migration of disks between storage repositories (see: [Disk Migrations](docs/disk-migrations.md))
- Static provisioning (use an existing VDI by UUID)
- Offline volume expansion
- Topology aware

## Documentation

Start here
- [Getting started](docs/getting-started.md)

How to use the driver
- selecting [Storage Repositories](docs/storage-repositories.md)
- feature [Disk Migrations](docs/disk-migrations.md)
- pre existing [Static volumes](docs/static.md)

Configuration
- [StorageClass parameters](docs/storage-class.md)
- [Driver configuration (flags, authentication)](docs/driver-configuration.md)

Operational notes
- [Disk creation leakage and mitigations](docs/disk-creation-leakage.md)

Development
- [Development](DEVELOPMENT.md)


