#!/usr/bin/env bats

load helpers


# bats test_tags=ci:parallel
@test "podman mount - basic test" {
    # Only works with root (FIXME: does it work with rootless + vfs?)
    skip_if_rootless "mount does not work rootless"
    skip_if_remote "mounting remote is meaningless"

    f_path=/tmp/tmpfile_$(random_string 8)
    f_content=$(random_string 30)

    c_name="c-mount-$(safename)"
    run_podman run --name $c_name $IMAGE \
               sh -c "echo $f_content > $f_path"

    run_podman mount $c_name
    mount_path=$output

    test -d $mount_path
    test -e "$mount_path/$f_path"
    is $(< "$mount_path/$f_path") "$f_content" "contents of file, as read via fs"

    # Make sure that 'podman mount' (no args) returns the expected path and CID
    run_podman inspect --format '{{.ID}}' $c_name
    cid="$output"

    run_podman mount --notruncate
    assert "$output" != "" "'podman mount' should list one or more mounts"
    reported_cid=$(awk -v WANT="$mount_path" '$2 == WANT {print $1}' <<<"$output")
    assert "$reported_cid" == "$cid" "CID of mount point matches container ID"

    # umount, and make sure files are gone
    run_podman umount $c_name
    if [[ -e "$mount_path/$f_path" ]]; then
        # With vfs, umount is a NOP: the path always exists as long as the
        # container exists. But with overlay, umount should truly remove.
        if [[ "$(podman_storage_driver)" != "vfs" ]]; then
            die "Mounted file exists even after umount: $mount_path/$f_path"
        fi
    fi

    # Remove the container. Now even with vfs the file must be gone.
    run_podman rm $c_name
    if [[ -e "$mount_path/$f_path" ]]; then
        die "Mounted file exists even after container rm: $mount_path/$f_path"
    fi
}

# DO NOT PARALLELIZE: mount/umount -a
@test "podman image mount" {
    skip_if_remote "mounting remote is meaningless"
    skip_if_rootless "too hard to test rootless"

    # Start with clean slate
    run_podman image umount -a

    # Get full image ID, to verify umount
    run_podman image inspect --format '{{.ID}}' $IMAGE
    iid="$output"

    # Mount, and make sure the mount point exists
    run_podman image mount $IMAGE
    mount_path="$output"

    test -d $mount_path

    # Image is custom-built and has a file containing the YMD tag. Check it.
    testimage_file="/home/podman/testimage-id"
    test -e "$mount_path$testimage_file"
    is $(< "$mount_path$testimage_file") "$PODMAN_TEST_IMAGE_TAG"  \
       "Contents of $testimage_file in image"

    # 'image mount', no args, tells us what's mounted
    run_podman image mount
    is "$output" "$IMAGE *$mount_path" "podman image mount with no args"

    # Clean up, and make sure nothing is mounted any more
    run_podman image umount -f $IMAGE
    is "$output" "$iid" "podman image umount: image ID of what was umounted"

    run_podman image umount $IMAGE
    is "$output" "" "podman image umount: does not re-umount"

    run_podman 125 image umount no-such-image
    is "$output" "Error: no-such-image: image not known" \
       "error message from image umount no-such-image"

    # Tests for mount -a. This may mount more than one image! (E.g. systemd)
    run_podman image mount -a
    is "$output" "$IMAGE .*$mount_path"

    run_podman image umount -a
    assert "$output" =~ "$iid" "Test image is unmounted"

    run_podman image mount
    is "$output" "" "podman image mount, no args, after umount"
}

# bats test_tags=ci:parallel
@test "podman run --mount ro=false " {
    local volpath=/path/in/container
    local stdopts="type=volume,destination=$volpath"

    # Variations on a theme (not by Paganini). All of these should fail.
    for varopt in readonly readonly=true ro=true ro rw=false;do
        run_podman 1 run --rm -q --mount $stdopts,$varopt $IMAGE touch $volpath/a
        is "$output" "touch: $volpath/a: Read-only file system" "with $varopt"
    done

    # All of these should pass
    for varopt in rw rw=true ro=false readonly=false;do
        run_podman run --rm -q --mount $stdopts,$varopt $IMAGE touch $volpath/a
        assert "$output" = "" "touch, with varopt=$varopt"
    done
}

# bats test_tags=ci:parallel
@test "podman run --mount image" {
    skip_if_rootless "too hard to test rootless"

    # For parallel safety: create a temporary image to use for mounts
    local tmpctr="c-$(safename)"
    local iname="i-$(safename)"
    run_podman run --name $tmpctr $IMAGE true
    run_podman commit -q $tmpctr $iname
    run_podman rm $tmpctr

    run_podman image inspect --format '{{.ID}}' $iname
    local iid="$output"

    mountopts="type=image,src=$iname,dst=/image-mount"
    # Run a container with an image mount
    run_podman run --rm --mount $mountopts $iname \
               diff /etc/os-release /image-mount/etc/os-release
    assert "$output" == "" "no output from diff command"

    # Make sure the mount is read-only
    run_podman 1 run --rm --mount $mountopts $iname \
               touch /image-mount/read-only
    is "$output" "touch: /image-mount/read-only: Read-only file system"

    # Make sure that rw,readwrite work
    run_podman run --rm --mount "${mountopts},rw=true" $iname \
               touch /image-mount/readwrite
    run_podman run --rm --mount "${mountopts},readwrite=true" $iname \
               touch /image-mount/readwrite

    tmpctr="c-$(safename)"
    subpathopts="type=image,src=$iname,dst=/image-mount,subpath=/etc"
    run_podman run --name $tmpctr --mount "${subpathopts}" $iname \
               ls /image-mount/shadow
    run_podman inspect $tmpctr --format '{{ (index .Mounts 0).SubPath }}'
    assert "$output" == "/etc" "SubPath contains /etc"
    run_podman rm $tmpctr

    # The rest of the tests below are meaningless under remote
    if is_remote; then
        run_podman rmi $iname
        return
    fi

    # All the above commands were 'run --rm'. Confirm no stray mounts left.
    run_podman mount --notruncate
    assert "$output" !~ "$iid" "stray mount found!"

    # Now make sure that the image mount is not cleaned up during container removal when another entity mounted the image
    run_podman image mount $iname
    local mountpoint="$output"

    # Confirm that image is mounted
    run_podman image mount
    assert "$output" =~ ".*localhost/$iname:latest  *$mountpoint.*" \
           "Image is mounted"

    run_podman run --rm --mount $mountopts $iname \
               diff /etc/os-release /image-mount/etc/os-release
    assert "$output" == "" "no output from diff command"

    # Image must still be mounted
    run_podman image mount
    assert "$output" =~ ".*localhost/$iname:latest  *$mountpoint.*" \
           "Image is still mounted after container run --rm"

    run_podman image umount $iname
    is "$output" "$iid" "podman image umount, first time, confirms IID"

    run_podman image umount $iname
    is "$output" "" "podman image umount, second time, is a NOP"

    # Run a container in the background (source is the ID instead of name)
    run_podman run -d --mount type=image,src=$iid,dst=/image-mount,readwrite=true $IMAGE sleep infinity
    cid="$output"

    # Unmount the image
    run_podman image umount $iname
    is "$output" "$iid" "podman image umount of CONTAINER mount: confirms IID"
    run_podman image umount $iname
    is "$output" "" "podman image umount of CONTAINER, second time, is a NOP"

    # Make sure that the mount in the container is unaffected
    run_podman exec $cid diff /etc/os-release /image-mount/etc/os-release
    assert "$output" = "" "no output from exec diff"
    run_podman exec $cid find /image-mount/home/podman
    assert "$output" =~ ".*/image-mount/home/podman/testimage-id.*" \
           "find /image-mount/home/podman"

    # Clean up
    run_podman rm -t 0 -f $cid
    run_podman rmi $iname
}

# bats test_tags=ci:parallel
@test "podman run --mount image inspection" {
    skip_if_rootless "too hard to test rootless"

    # Run a container in the background
    run_podman run -d --mount type=image,src=$IMAGE,dst=/image-mount,rw=true $IMAGE sleep infinity
    cid="$output"

    run_podman inspect --format "{{(index .Mounts 0).Type}}" $cid
    is "$output" "image" "inspect data includes image mount type"

    run_podman inspect --format "{{(index .Mounts 0).Source}}" $cid
    is "$output" "$IMAGE" "inspect data includes image mount source"

    run_podman inspect --format "{{(index .Mounts 0).Destination}}" $cid
    is "$output" "/image-mount" "inspect data includes image mount source"

    run_podman inspect --format "{{(index .Mounts 0).RW}}" $cid
    is "$output" "true" "inspect data includes image mount source"

    run_podman rm -t 0 -f $cid
}

# bats test_tags=ci:parallel
@test "podman mount containers.conf" {
    skip_if_remote "remote does not support CONTAINERS_CONF*"

    dest=/$(random_string 30)
    tmpfile1=$PODMAN_TMPDIR/volume-test1
    random1=$(random_string 30)
    echo $random1 > $tmpfile1

    tmpfile2=$PODMAN_TMPDIR/volume-test2
    random2=$(random_string 30)
    echo $random2 > $tmpfile2
    bogus=$(random_string 10)

    mountStr1=type=bind,src=$tmpfile1,destination=$dest,ro,Z
    mountStr2=type=bind,src=$tmpfile2,destination=$dest,ro,Z
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[containers]
mounts=[ "$mountStr1", ]
EOF
    badcontainersconf=$PODMAN_TMPDIR/badcontainers.conf
    cat >$badcontainersconf <<EOF
[containers]
mounts=[ "type=$bogus,src=$tmpfile2,destination=$dest,ro", ]
EOF

    run_podman 1 run --rm $IMAGE cat $dest
    is "$output" "cat: can't open '$dest': No such file or directory" "$dest does not exist"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm $IMAGE cat $dest
    is "$output" "$random1" "file should contain $random1"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm --mount $mountStr2 $IMAGE cat $dest
    is "$output" "$random2" "overridden file should contain $random2"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman 125 run --mount $mountStr1 --mount $mountStr2 $IMAGE cat $dest
    is "$output" "Error: $dest: duplicate mount destination" "Should through duplicate destination error for $dest"

    CONTAINERS_CONF_OVERRIDE="$badcontainersconf" run_podman 125 run $IMAGE cat $dest
    is "$output" "Error: parsing containers.conf mounts: invalid filesystem type \"$bogus\"" "containers.conf should fail with bad mounts entry"
}

# bats test_tags=ci:parallel
@test "podman mount external container - basic test" {
    # Only works with root (FIXME: does it work with rootless + vfs?)
    skip_if_rootless "mount does not work rootless"
    skip_if_remote "mounting remote is meaningless"

    # Create a container that podman does not know about
    external_cname=$(buildah from --name b-$(safename) $IMAGE)

    run_podman mount $external_cname
    mount_path=$output

    # convert buildah CID to podman CID. We can't use podman inspect
    # because that can't access external containers by name.
    run_podman ps --external -a --notruncate --format '{{.ID}} {{.Names}}'
    external_cid=$(awk -v WANT="$external_cname" '$2 == WANT {print $1}' <<<"$output")
    assert "$external_cid" != "" "SHA for $external_cname"

    # Test image will always have this file, and will always have the tag
    test -d $mount_path
    is $(< "$mount_path/home/podman/testimage-id") "$PODMAN_TEST_IMAGE_TAG"  \
       "Contents of well-known file in image"

    # Make sure that 'podman mount' (no args) returns the expected path
    run_podman mount --notruncate

    cid_found=$(awk -v WANT="$mount_path" '$2 == WANT { print $1 }' <<<"$output")
    assert "$cid_found" = "$external_cid" "'podman mount' lists CID + mountpoint"

    # umount, and make sure mountpoint no longer exists
    run_podman umount $external_cname
    ensure_no_mountpoint "$mount_path"
    buildah rm $external_cname
}

# bats test_tags=ci:parallel
@test "podman volume globs" {
    v1a="v1a-$(safename)"
    v1b="v1b-$(safename)"
    v2="v2-$(safename)"
    vol1a=${PODMAN_TMPDIR}/$v1a
    vol1b=${PODMAN_TMPDIR}/$v1b
    vol2=${PODMAN_TMPDIR}/$v2
    touch $vol1a $vol1b $vol2

    # if volumes source and dest match then pass
    run_podman run --rm --mount type=glob,src=${PODMAN_TMPDIR}/v1\*,ro $IMAGE ls $vol1a $vol1b
    run_podman 1 run --rm --mount source=${PODMAN_TMPDIR}/v1\*,type=glob,ro $IMAGE ls $vol2
    is "$output" ".*No such file or directory" "$vol2 should not be mounted in the container"

    run_podman 125 run --rm --mount source=${PODMAN_TMPDIR}/v3\*,type=glob,ro $IMAGE ls $vol2
    is "$output" "Error: no file paths matching glob \"${PODMAN_TMPDIR}/v3\*\"" "Glob does not match so should throw error"

    run_podman 1 run --rm --mount source=${PODMAN_TMPDIR}/v2\*,type=glob,ro,Z $IMAGE touch $vol2
    is "$output" "touch: $vol2: Read-only file system" "Mount should be read-only"

    run_podman run --rm --mount source=${PODMAN_TMPDIR}/v2\*,type=glob,ro=false,Z $IMAGE touch $vol2

    run_podman run --rm --mount type=glob,src=${PODMAN_TMPDIR}/v1\*,destination=/non/existing/directory,ro $IMAGE ls /non/existing/directory
    is "$output" ".*$v1a" "podman images --inspect should include v1a"
    is "$output" ".*$v1b" "podman images --inspect should include v1b"

    run_podman create --mount type=glob,src=${PODMAN_TMPDIR}/v1\*,ro $IMAGE ls $vol1a $vol1b
    cid=$output
    run_podman container inspect $cid
    is "$output" ".*$v1a" "podman images --inspect should include v1a"
    is "$output" ".*$v1b" "podman images --inspect should include v1b"
    run_podman rm $cid

    run_podman 125 run --rm --mount type=bind,source=${PODMAN_TMPDIR}/v2\*,ro=false $IMAGE touch $vol2
    is "$output" "Error: must set volume destination" "Bind mounts require destination"

    run_podman 125 run --rm --mount type=bind,source=${PODMAN_TMPDIR}/v2\*,destination=/tmp/foobar,ro=false $IMAGE touch $vol2
    is "$output" "Error: statfs ${PODMAN_TMPDIR}/v2*: no such file or directory" "Bind mount should not interpret glob and must use as is"

    mkdir $PODMAN_TMPDIR/foo1 $PODMAN_TMPDIR/foo2 $PODMAN_TMPDIR/foo3
    touch $PODMAN_TMPDIR/foo1/bar $PODMAN_TMPDIR/foo2/bar $PODMAN_TMPDIR/foo3/bar
    touch $PODMAN_TMPDIR/foo1/bar1 $PODMAN_TMPDIR/foo2/bar2 $PODMAN_TMPDIR/foo3/bar3
    run_podman 125 run --rm --mount type=glob,source=${PODMAN_TMPDIR}/foo?/bar,destination=/tmp $IMAGE ls -l /tmp
    is "$output" "Error: /tmp/bar: duplicate mount destination" "Should report conflict on destination directory"
    run_podman run --rm --mount type=glob,source=${PODMAN_TMPDIR}/foo?/bar?,destination=/tmp,ro $IMAGE ls /tmp
    is "$output" "bar1.*bar2.*bar3" "Should match multiple source files on single destination directory"
}

# bats test_tags=ci:parallel
@test "podman mount noswap memory mounts" {
    # tmpfs+noswap new in kernel 6.x, mid-2023; likely not in RHEL for a while
    if ! is_rootless; then
        testmount=$PODMAN_TMPDIR/testmount
        mkdir $testmount
        run mount -t tmpfs -o noswap none $testmount
        if [[ $status -ne 0 ]]; then
            if [[ $output =~ "bad option" ]] || [[ "$output" =~ "Unknown parameter" ]]; then
                skip "requires kernel with tmpfs + noswap support"
            fi
            die "Could not test for tmpfs + noswap support: $output"
        else
            umount $testmount
        fi
    fi

    # if volumes source and dest match then pass
    run_podman run --rm --mount type=ramfs,destination=${PODMAN_TMPDIR} $IMAGE stat -f -c "%T" ${PODMAN_TMPDIR}
    is "$output" "ramfs" "ramfs mounted"

    if is_rootless; then
        run_podman 125 run --rm --mount type=tmpfs,destination=${PODMAN_TMPDIR},noswap  $IMAGE stat -f -c "%T" ${PODMAN_TMPDIR}
        is "$output" "Error: the 'noswap' option is only allowed with rootful tmpfs mounts: must provide an argument for option" "noswap not supported in rootless mode"
    else
        run_podman run --rm --mount type=tmpfs,destination=${PODMAN_TMPDIR},noswap  $IMAGE sh -c "mount| grep ${PODMAN_TMPDIR}"
        is "$output" ".*noswap" "tmpfs noswap mounted"
    fi
}

# bats test_tags=ci:parallel
@test "podman mount no-dereference" {
    # Test how bind and glob-mounts behave with respect to relative (rel) and
    # absolute (abs) symlinks.

    if [ $(podman_runtime) != "crun" ]; then
        # Requires crun >= 1.11.0
        skip "only crun supports the no-dereference (copy-symlink) mount option"
    fi

    # Contents of the file 'data' inside the container image.
    declare -A datacontent=(
        [img]="data file inside the IMAGE - $(random_string 15)"
    )

    # Purpose of the image is so "link -> data" can point to an existing
    # file whether or not "data" is mounted.
    dockerfile=$PODMAN_TMPDIR/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN mkdir /mountroot && echo ${datacontent[img]} > /mountroot/data
EOF

    # --layers=false needed to work around buildah#5674 parallel flake
    img="localhost/img-$(safename)"
    run_podman build -t $img --layers=false -f $dockerfile

    # Each test is set up in exactly the same way:
    #
    #    <tmpdir>/
    #    ├── mountdir/     <----- this is always the source dir
    #    │   ├── data
    #    │   └── link -> ?????
    #    └── otherdir/
    #        └── data
    #
    # The test is run in a container that has its own /mountroot/data file,
    # so in some situations 'link -> data' will get the container's
    # data file, in others it'll be the host's, and in others, ENOENT.
    #
    # There are four options for 'link': -> data in mountdir (same dir)
    # or otherdir, and, relative or absolute. Then, for each of those
    # permutations, run with and without no-dereference. (With no-dereference,
    # only the first of these options is valid, link->data. The other three
    # appear in the container as link->path-not-in-container)
    #
    # Finally, the table below defines a number of variations of mount
    # type (bind, glob); mount source (just the link, a glob, or entire
    # directory); and mount destination. These are the variations that
    # introduce complexity, hence the special cases in the innermost loop.
    #
    # Table format:
    #
    #    mount type | mount source | mount destination | what_is_data | enoents
    #
    # The what_is_data column indicates whether the file "data" in the
    # container will be the image's copy ("img") or the one from the host
    # ("in", referring to the source directory). "-" means N/A, no data file.
    #
    # The enoent column is a space-separated list of patterns to search for
    # in the test description. When these match, "link" will point to a
    # path that does not exist in the directory, and we should expect cat
    # to result in ENOENT.
    #
    tests="
bind | /link | /mountroot/link       | img
bind | /link | /i/do/not/exist/link  | -    | relative.*no-dereference
bind | /     | /mountroot/           | in   | absolute out
glob | /lin* | /mountroot/           | img
glob | /*    | /mountroot/           | in
"

    defer-assertion-failures

    while read mount_type mount_source mount_dest what_is_data enoents; do
        # link pointing inside the same directory, or outside
        for in_out in "in" "out"; do
            # relative symlink or absolute
            for rel_abs in "relative" "absolute"; do
                # Generate fresh new content for each data file (the in & out ones)
                datacontent[in]="data file in the SAME DIRECTORY - $(random_string 15)"
                datacontent[out]="data file OUTSIDE the tree - $(random_string 15)"

                # Populate data files in and out our tree
                local condition="${rel_abs:0:3}-${in_out}"
                local sourcedir="$PODMAN_TMPDIR/$condition"
                rm -rf $sourcedir $PODMAN_TMPDIR/outside-the-tree
                mkdir  $sourcedir $PODMAN_TMPDIR/outside-the-tree
                echo "${datacontent[in]}"  > "$sourcedir/data"
                echo "${datacontent[out]}" > "$PODMAN_TMPDIR/outside-the-tree/data"

                # Create the symlink itself (in the in-dir of course)
                local target
                case "$condition" in
                    rel-in)  target="data" ;;
                    rel-out) target="../outside-the-tree/data" ;;
                    abs-in)  target="$sourcedir/data" ;;
                    abs-out) target="$PODMAN_TMPDIR/outside-the-tree/data" ;;
                    *)       die "Internal error, invalid condition '$condition'" ;;
                esac
                ln -s $target "$sourcedir/link"

                # Absolute path to 'link' inside the container. What we stat & cat.
                local containerpath="$mount_dest"
                if [[ ! $containerpath =~ /link$ ]]; then
                    containerpath="${containerpath}link"
                fi

                # Now test with no args (mounts link CONTENT) and --no-dereference
                # (mounts symlink AS A SYMLINK)
                for mount_opts in "" ",no-dereference"; do
                    local description="$mount_type mount $mount_source -> $mount_dest ($in_out), $rel_abs $mount_opts"

                    # Expected exit status. Almost always success.
                    local exit_code=0

                    # Without --no-dereference, we always expect exactly the same,
                    # because podman mounts "link" as a data file...
                    local expect_stat="$containerpath"
                    local expect_cat="${datacontent[$in_out]}"
                    # ...except when bind-mounting link's parent directory: "link"
                    # is mounted as a link, and host's "data" file overrides the image
                    if [[ $mount_source = '/' ]]; then
                        expect_stat="'$containerpath' -> '$target'"
                    fi

                    # With --no-dereference...
                    if [[ -n "$mount_opts" ]]; then
                        # stat() is always the same (symlink and its target)  ....
                        expect_stat="'$containerpath' -> '$target'"

                        # ...and the only valid case for cat is same-dir relative:
                        if [[ "$condition" = "rel-in" ]]; then
                            expect_cat="${datacontent[$what_is_data]}"
                        else
                            # All others are ENOENT, because link -> nonexistent-path
                            exit_code=1
                        fi
                    fi

                    for ex in $enoents; do
                        if grep -q -w -E "$ex" <<<"$description"; then
                            exit_code=1
                        fi
                    done
                    if [[ $exit_code -eq 1 ]]; then
                        expect_cat="cat: can't open '$containerpath': No such file or directory"
                    fi

                    run_podman $exit_code run \
                               --mount type=$mount_type,src="$sourcedir$mount_source",dst="$mount_dest$mount_opts" \
                               --rm --privileged $img sh -c "stat -c '%N' $containerpath; cat $containerpath"
                    assert "${lines[0]}" = "$expect_stat" "$description -- stat $containerpath"
                    assert "${lines[1]}" = "$expect_cat"  "$description -- cat $containerpath"
                done
            done
        done
    done < <(parse_table "$tests")

    immediate-assertion-failures

    # Make sure that links are preserved across starts and stops
    local workdir=$PODMAN_TMPDIR/test-restart
    mkdir $workdir
    local datafile="data-$(random_string 5)"
    local datafile_contents="What we expect to see, $(random_string 20)"
    echo "$datafile_contents" > $workdir/$datafile
    ln -s $datafile $workdir/link

    run_podman create --mount type=glob,src=$workdir/*,dst=/mountroot/,no-dereference --privileged $img sh -c "stat -c '%N' /mountroot/link; cat /mountroot/link; ls -l /mountroot"
    cid="$output"
    run_podman start -a $cid
    assert "${lines[0]}" = "'/mountroot/link' -> '$datafile'" "symlink is preserved, on start"
    assert "${lines[1]}" = "$datafile_contents"         "glob matches symlink and host 'data' file, on start"
    run_podman start -a $cid
    assert "${lines[0]}" = "'/mountroot/link' -> '$datafile'" "symlink is preserved, on restart"
    assert "${lines[1]}" = "$datafile_contents"         "glob matches symlink and host 'data' file, on restart"
    run_podman rm -f -t=0 $cid

    run_podman rmi -f $img
}
