# Development

## Useful References
- https://kubernetes-csi.github.io/
- https://github.com/container-storage-interface/spec/blob/master/spec.md
- https://arslan.io/2018/06/21/how-to-write-a-container-storage-interface-csi-plugin/


## Deploy

```bash
# When developing, it's expected to bring your own registry
export DEVELOPMENT_IMAGE_NAME=some-registry.example.com/csi-xen-orchestra-driver

make deploy
```


## Release
- Update `VERSION` to release version in Makefile (e.g. `0.1.0`)
- Publish Image: Run `RELEASE=true make build push`
- Publish Chart: Run `RELEASE=true make helm-publish`
- Update README.md if necessary
- Git commit and push
- Create a github release