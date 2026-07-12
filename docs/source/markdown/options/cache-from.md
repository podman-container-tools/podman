####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--cache-from**=*image*

Repository to utilize as a potential cache source. When specified, Buildah tries to look for
cache images in the specified repository and attempts to pull cache images instead of actually
executing the build steps locally. Buildah only attempts to pull previously cached images if they
are considered as valid cache hits.

Use the `--cache-to` option to populate a remote repository with cache content.

Example

```bash
# populate a cache and also consult it
buildah build -t test --layers --cache-to registry/myrepo/cache --cache-from registry/myrepo/cache .
```

Note: `--cache-from` option is ignored unless `--layers` is specified.
