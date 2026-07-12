% podman-manifest-remove 1

## NAME
podman\-manifest\-remove - Remove an item from a manifest list or image index

## SYNOPSIS
**podman manifest remove** *listnameorindexname* *digest*

## DESCRIPTION
Removes the item with the specified digest from the specified manifest list or image index.

## RETURN VALUE
The list image's ID and the digest of the removed image's manifest.

## EXAMPLE

Remove specified digest from the manifest list:
```
podman manifest remove mylist:v1.11 sha256:cb8a924afdf0229ef7515d9e5b3024e23b3eb03ddbba287f4a19c6ac90b8d221
podman manifest remove e604eabaaee4858232761b4fef84e2316ed8f93e15eceafce845966ee3400036 sha256:cb8a924afdf0229ef7515d9e5b3024e23b3eb03ddbba287f4a19c6ac90b8d221
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-manifest(1)](podman-manifest.1.md)**
