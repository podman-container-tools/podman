#!/bin/sh
for i in $@; do
    b=$(basename $i)
    filename=$(echo $i | sed 's/podman/docker/g')
    echo .so man1/$b > $filename
done
echo .so man5/containerfile.5 > $(dirname $1)/dockerfile.5
echo .so man5/containerignore.5 > $(dirname $1)/.dockerignore.5
echo .so man5/containerignore.5 > $(dirname $1)/dockerignore.5
