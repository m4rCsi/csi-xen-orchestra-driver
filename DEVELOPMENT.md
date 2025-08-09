# Development

## Useful References
- https://kubernetes-csi.github.io/
- https://github.com/container-storage-interface/spec/blob/master/spec.md
- https://arslan.io/2018/06/21/how-to-write-a-container-storage-interface-csi-plugin/


## Deploy

Adjust the makefile settings:
- OVERLAY_DIR (i.e. create your own with your own patches)
- IMAGE_NAME (i.e. if you want to host dev images on a different registry)

the makefile will update the image in the kustomize file and then deploy it

```bash
make deploy
```