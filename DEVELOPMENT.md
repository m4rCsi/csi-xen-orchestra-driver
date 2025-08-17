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


## Release
- Update `IMAGE_TAG` to release version in Makefile (e.g. `v0.1.0`)
- Run `make build push`
- Update getting-started.md and README.md if necessary

```bash
cd deploy/kustomize/overlays/release
vi kustomization.yaml # and update the newTag
kubectl kustomize .  -o driver.yaml # this will be a release artifact
```

- Git Commit and push the kustomization.yaml update
- Create a release with the release artifact


