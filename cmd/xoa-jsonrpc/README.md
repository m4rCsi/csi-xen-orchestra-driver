# Xen Orchestra - JSON-RPC over websockets
[Xen Orchestra - JSON-RPC API](https://docs.xcp-ng.org/management/manage-at-scale/xo-api/#-json-rpc-over-websockets)

We use the json-rpc API to manage and attach volumes to our kubernetes nodes. 

## `./bin/xoa-jsonrpc`
This is a cli tool to help validate and test the `./pkg/xoa` library and its json-rpc connection to a xen-orchestra. It can also be used to make sure the token and authentication with your Xen-Orchestra works as expected.

```bash
$ # Building
$ make build
 
$ # Setting up Auth and Endpoint
$ export XOA_URL=https://xoa.example.com
$ export XOA_TOKEN=XXXXXXXXXXXXXX-YYYYYYYYYYYYYYYYYYYYYYYYYYYY

$ # List Disks
$ ./bin/xoa-jsonrpc -command list-disks
VDI: {UUID:b37df2c0-b549-4f50-bf29-ca849b09bde3 NameLabel:some_name Size:10737418240 Type:VDI SR: ReadOnly:false Sharable:false}
VDI: {UUID:359e652a-9cd5-4bb7-ae90-bde106c293e4 NameLabel:disk1 Size:34359738368 Type:VDI SR: ReadOnly:false Sharable:false}
VDI: {UUID:46611e4d-a845-4b21-bd82-164ccc972b4a NameLabel:/dev/xvda Size:53687091200 Type:VDI SR: ReadOnly:false Sharable:false}
VDI: {UUID:53a4d9bd-c72e-4c3c-b2c8-3379e963e37c NameLabel:/dev/xvdb Size:53687091200 Type:VDI SR: ReadOnly:false Sharable:false}

$ # Create Disk called example1
$ ./bin/xoa-jsonrpc -command create-disk -disk-name example1 -sr-uuid 5e653748-9223-c319-7cb4-f6e20384de61
Creating disk 'example1' with 1GB size in SR 5e653748-9223-c319-7cb4-f6e20384de61...
Successfully created disk 'example1' with UUID: a941bed2-8334-445a-8535-870368cb6050

$ # Resize Disk 
$ ./bin/xoa-jsonrpc -command resize-disk -disk-name example1
Resizing disk 'example1' to 5368709120 bytes...
Successfully resized disk 'example1' to 5368709120 bytes

$ # Attaching Disk and waiting until attached
$ ./bin/xoa-jsonrpc -command attach-disk -disk-name example1 -vm-uuid 5d538d74-a34e-2fc3-10f6-a440fca4345b
Attaching and connecting disk 'example1' to VM 5d538d74-a34e-2fc3-10f6-a440fca4345b...
Successfully attached disk 'example1' (VDI UUID: a941bed2-8334-445a-8535-870368cb6050) to VM 5d538d74-a34e-2fc3-10f6-a440fca4345b
VBD &{UUID:0ef97818-4de9-def2-13df-a29575d05de7 NameLabel: VM:5d538d74-a34e-2fc3-10f6-a440fca4345b VDI:a941bed2-8334-445a-8535-870368cb6050 Device:xvdc Mode: Bootable:false Attached:true}

$ # Detaching Disk
$ ./bin/xoa-jsonrpc -command detach-disk -disk-name example1 -vm-uuid 5d538d74-a34e-2fc3-10f6-a440fca4345b
Detaching disk 'example1' from VM 5d538d74-a34e-2fc3-10f6-a440fca4345b...
Deleting VBD: {UUID:0ef97818-4de9-def2-13df-a29575d05de7 NameLabel: VM:5d538d74-a34e-2fc3-10f6-a440fca4345b VDI:a941bed2-8334-445a-8535-870368cb6050 Device:xvdc Mode: Bootable:false Attached:true}
Successfully detached disk 'example1' from VM 5d538d74-a34e-2fc3-10f6-a440fca4345b

$ # Deleting Disk
$ ./bin/xoa-jsonrpc -command delete-disk -disk-name example1 -vm-uuid 5d538d74-a34e-2fc3-10f6-a440fca4345b
Deleting disk 'example1'...
Successfully deleted disk 'example1'
```
