# Local Storage

Local Storage Repositories (SRs) in XCP-ng are storage pools that are accessible only from a specific hypervisor (host). This creates unique challenges and considerations when using them with Kubernetes and this CSI driver.

> [!NOTE]
> See [XCP-ng Documentation: Storage in XCP-ng](https://docs.xcp-ng.org/storage/) for details about how Storage works in XCP-ng.

## Topology Scoping Challenges

### Default Behavior: Pool-Level Topology

By default, this CSI driver uses **pool-level topology** rather than host-level topology. This means:

- Volumes are scheduled based on the XCP-ng pool, not individual hosts
- Local storage becomes problematic because it's host-scoped but the driver doesn't track host topology
- **Volume attachment failures**: The driver can't guarantee the volume will be accessible from the scheduled node

This design choice is intentional because it's expected that Virtual Machines (Kubernetes nodes) will be migrated between hosts when the need arises. When they are migrated, the topology will change. However, the CSI framework does not allow topology changes. Therefore, if we want to allow VMs to migrate while keeping the CSI driver working as expected, we need to use only pool-level topology.

### How to Enable Local Storage

There are two ways to work with local storage:
- **(A) Only use local SRs together with `migrating: "true"`**: This means that the local storage disk will be migrated to the host where the VM will be present on demand. **This is the recommended approach for local storage.** (See: [Disk migrations](disk-migrations.md) for details)

- **(B) Configure the driver with host-level topology enabled**: This will mean that when Virtual Machines are migrated, the CSI driver will fail to behave properly, since the topology cannot be changed. A deletion of the node resource will be necessary. (See: [Driver configuration](driver-configuration.md) for topology settings)


> [!NOTE]
> **Recommendation**: Use approach (A) with `migrating: "true"` instead of host-level topology when possible. Host-level topology should only be used when neither VM nor disk migrations are expected.

## Enabling Host-Level Topology


Configure the driver to use host-level topology by setting the appropriate topology mode:

```yaml
# In your Helm values.yaml
csiXenOrchestraDriver:
  config:
    hostTopology: true  # Use host-level topology instead of pool-level
```

This enables:
- Proper scheduling based on host availability
- Local storage repository support
- Volume placement that respects host boundaries

However, this has consequences. If you expect Virtual Machines to be migrated, this will not work properly with this CSI driver.

## Related Documentation

- [Storage Repositories](storage-repositories.md) - SR selection and configuration including local storage considerations
- [StorageClass options](storage-class.md) - Available parameters and configuration
- [Driver configuration](driver-configuration.md) - Driver flags and topology settings
- [Disk migrations](disk-migrations.md) - Understanding volume migration capabilities

