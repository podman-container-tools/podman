#!/usr/bin/env bats   -*- bats -*-
#
# Tests for 'podman cp'
#
# ASSUMPTION FOR ALL THESE TESTS: /tmp in the container starts off empty
#

load helpers

# All tests here can be run in parallel
# bats file_tags=ci:parallel

@test "podman cp file from host to container" {
    srcdir=$PODMAN_TMPDIR/cp-test-file-host-to-ctr
    mkdir -p $srcdir
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
        random-2-$(random_string 20)
    )

    echo "${randomcontent[0]}" > $srcdir/hostfile0
    echo "${randomcontent[1]}" > $srcdir/hostfile1
    echo "${randomcontent[2]}" > $srcdir/hostfile2
    mkdir -p $srcdir/subdir
    echo "${randomcontent[2]}" > $srcdir/subdir/dotfile.

    local destrunning="c-destrun-$(safename)"
    run_podman run -d --name $destrunning --workdir=/srv $IMAGE sh -c "mkdir /srv/subdir; echo READY; sleep infinity"
    wait_for_ready $destrunning

    # Commit the image for testing non-running containers
    run_podman commit -q $destrunning
    cpimage="$output"

    # format is: <id> | <destination arg to cp> | <full dest path> | <test name>
    # where:
    #    id        is 0-2, one of the random strings/files
    #    dest arg  is the right-hand argument to 'podman cp' (may be implicit)
    #    dest path is the full explicit path we expect to see
    #    test name is a short description of what we're testing here
    tests="
0 | /                    | /hostfile0            | copy to root
0 | /anotherbase.txt     | /anotherbase.txt      | copy to root, new name
0 | /tmp                 | /tmp/hostfile0        | copy to /tmp
1 | /tmp/                | /tmp/hostfile1        | copy to /tmp/
2 | /tmp/.               | /tmp/hostfile2        | copy to /tmp/.
0 | /tmp/anotherbase.txt | /tmp/anotherbase.txt  | copy to /tmp, new name
0 | .                    | /srv/hostfile0        | copy to workdir (rel path), new name
1 | ./                   | /srv/hostfile1        | copy to workdir (rel path), new name
0 | anotherbase.txt      | /srv/anotherbase.txt  | copy to workdir (rel path), new name
0 | subdir               | /srv/subdir/hostfile0 | copy to workdir/subdir
"

    defer-assertion-failures

    # RUNNING container
    while read id dest dest_fullname description; do
        run_podman cp $srcdir/hostfile$id $destrunning:$dest
        run_podman exec $destrunning cat $dest_fullname
        is "$output" "${randomcontent[$id]}" "$description (cp -> ctr:$dest)"
    done < <(parse_table "$tests")

    # Dots are special for dirs not files.
    run_podman cp $srcdir/subdir/dotfile. $destrunning:/tmp
    run_podman exec $destrunning cat /tmp/dotfile.
    is "$output" "${randomcontent[2]}" "$description (cp -> ctr:$dest)"

    # Host path does not exist.
    run_podman 125 cp $srcdir/IdoNotExist $destrunning:/tmp
    is "$output" 'Error: ".*/IdoNotExist" could not be found on the host' \
       "copy nonexistent host path"

    # Container (parent) path does not exist.
    run_podman 125 cp $srcdir/hostfile0 $destrunning:/IdoNotExist/
    is "$output" "Error: \"/IdoNotExist/\" could not be found on container $destrunning: no such file or directory" \
       "copy into nonexistent path in container"

    run_podman rm -t 0 -f $destrunning

    # CREATED container
    while read id dest dest_fullname description; do
        local destcreated="c-destcreated-$(safename)"
        run_podman create --name $destcreated --workdir=/srv $cpimage sleep infinity
        run_podman cp $srcdir/hostfile$id $destcreated:$dest
        run_podman start $destcreated
        run_podman exec $destcreated cat $dest_fullname
        is "$output" "${randomcontent[$id]}" "$description (cp -> ctr:$dest)"
        run_podman rm -t 0 -f $destcreated
    done < <(parse_table "$tests")

    run_podman rmi -f $cpimage
}


@test "podman cp file from host to container tmpfs mount" {
    srcdir=$PODMAN_TMPDIR/cp-test-file-host-to-ctr
    mkdir -p $srcdir
    content=tmpfile-content$(random_string 20)
    echo $content > $srcdir/file

    # RUNNING container
    local destrunning="c-destrun-$(safename)"
    run_podman run -d --mount type=tmpfs,dst=/tmp --name $destrunning $IMAGE sleep infinity
    run_podman cp $srcdir/file $destrunning:/tmp
    run_podman exec $destrunning cat /tmp/file
    is "$output" "${content}" "cp to running container's tmpfs"
    run_podman rm -t 0 -f $destrunning

    # CREATED container (with copy up)
    local destcreated=cdestcreated$(safename)
    run_podman create --mount type=tmpfs,dst=/tmp --name $destcreated $IMAGE sleep infinity
    run_podman cp $srcdir/file $destcreated:/tmp
    run_podman start $destcreated
    run_podman exec $destcreated cat /tmp/file
    is "$output" "${content}" "cp to created container's tmpfs"
    run_podman rm -t 0 -f $destcreated
}


@test "podman cp (-a=true) file from host to container and check ownership" {
    srcdir=$PODMAN_TMPDIR/cp-test-file-host-to-ctr
    mkdir -p $srcdir
    content=cp-user-test-$(random_string 10)
    echo "content" > $srcdir/hostfile
    userid=$(id -u)

    keepid="--userns=keep-id"
    is_rootless || keepid=""
    local destrunning="c-destrun-$(safename)"
    run_podman run --user=$userid ${keepid} -d --name $destrunning $IMAGE sleep infinity
    run_podman cp $srcdir/hostfile $destrunning:/tmp/hostfile
    run_podman exec $destrunning stat -c "%u" /tmp/hostfile
    is "$output" "$userid" "copied file is chowned to the container user"
    run_podman rm -t 0 -f $destrunning
}

@test "podman cp (-a=false) file from host to container and check ownership" {
    local tmpdir="${PODMAN_TMPDIR}/cp-test-file-host-to-ctr"
    mkdir -p "${tmpdir}"

    pushd "${tmpdir}"
    touch a.txt
    tar --owner=1042 --group=1043 -cf a.tar a.txt
    popd

    userid=$(id -u)

    keepid="--userns=keep-id"
    is_rootless || keepid=""
    local destrunning="c-destrun-$(safename)"
    run_podman run --user=$userid ${keepid} -d --name $destrunning $IMAGE sleep infinity
    run_podman cp -a=false - $destrunning:/tmp/ < "${tmpdir}/a.tar"
    run_podman exec $destrunning stat -c "%u:%g" /tmp/a.txt
    is "$output" "1042:1043" "copied file retains uid/gid from the tar"
    run_podman rm -t 0 -f $destrunning
}

@test "podman cp file from/to host while --pid=host" {
    if is_rootless && ! is_cgroupsv2; then
        skip "'podman cp --pid=host' (rootless) only works with cgroups v2"
    fi

    srcdir=$PODMAN_TMPDIR/cp-pid-equals-host
    mkdir -p $srcdir
    touch $srcdir/hostfile

    local cpcontainer="cpctr-$(safename)"
    run_podman run --pid=host -d --name $cpcontainer $IMAGE sleep infinity
    run_podman cp $srcdir/hostfile $cpcontainer:/tmp/hostfile
    run_podman cp $cpcontainer:/tmp/hostfile $srcdir/hostfile1
    run_podman rm -t 0 -f $cpcontainer
}

@test "podman cp file from container to host" {
    srcdir=$PODMAN_TMPDIR/cp-test-file-ctr-to-host
    mkdir -p $srcdir

    # Create 3 files with random content in the container.
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
        random-2-$(random_string 20)
    )
    local srcrunning="c-srcrun-$(safename)"
    run_podman run -d --name $srcrunning --workdir=/srv $IMAGE sh -c "mkdir /srv/subdir;
         echo ${randomcontent[0]} > /tmp/containerfile;
         echo ${randomcontent[0]} > /tmp/dotfile.;
         echo ${randomcontent[1]} > /srv/containerfile1;
         echo ${randomcontent[2]} > /srv/subdir/containerfile2;
         echo READY;
         sleep infinity"
    wait_for_ready $srcrunning

    # Commit the image for testing non-running containers
    run_podman commit -q $srcrunning
    cpimage="$output"

    # format is: <id> | <source arg to cp> | <destination arg (appended to $srcdir) to cp> | <full dest path (appended to $srcdir)> | <test name>
    tests="
0 | /tmp/containerfile    |          | /containerfile  | copy to srcdir/
0 | /tmp/dotfile.         |          | /dotfile.       | copy to srcdir/
0 | /tmp/containerfile    | /        | /containerfile  | copy to srcdir/
0 | /tmp/containerfile    | /.       | /containerfile  | copy to srcdir/.
0 | /tmp/containerfile    | /newfile | /newfile        | copy to srcdir/newfile
1 | containerfile1        | /        | /containerfile1 | copy from workdir (rel path) to srcdir
2 | subdir/containerfile2 | /        | /containerfile2 | copy from workdir/subdir (rel path) to srcdir
"

    defer-assertion-failures

    # RUNNING container
    while read id src dest dest_fullname description; do
        # dest may be "''" for empty table cells
        if [[ $dest == "''" ]];then
            unset dest
        fi
        run_podman cp $srcrunning:$src "$srcdir$dest"
        is "$(< $srcdir$dest_fullname)" "${randomcontent[$id]}" "$description (cp ctr:$src to \$srcdir$dest)"
        rm $srcdir$dest_fullname
    done < <(parse_table "$tests")
    run_podman rm -t 0 -f $srcrunning

    # Created container
    local srccreated="c-src-crt-$(safename)"
    run_podman create --name $srccreated --workdir=/srv $cpimage
    while read id src dest dest_fullname description; do
        # dest may be "''" for empty table cells
        if [[ $dest == "''" ]];then
            unset dest
        fi
        run_podman cp $srccreated:$src "$srcdir$dest"
        is "$(< $srcdir$dest_fullname)" "${randomcontent[$id]}" "$description (cp ctr:$src to \$srcdir$dest)"
        rm $srcdir$dest_fullname
    done < <(parse_table "$tests")
    run_podman rm -t 0 -f $srccreated

    run_podman rmi -f $cpimage
}

@test "podman cp file from container to container" {
    # Create 3 files with random content in the container.
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
        random-2-$(random_string 20)
    )

    local srcrunning="c-src-run-$(safename)"
    run_podman run -d --name $srcrunning --workdir=/srv $IMAGE sh -c "mkdir /srv/subdir;
         echo ${randomcontent[0]} > /tmp/containerfile;
         echo ${randomcontent[0]} > /tmp/dotfile.;
         echo ${randomcontent[1]} > /srv/containerfile1;
         echo ${randomcontent[2]} > /srv/subdir/containerfile2;
         echo READY;
         sleep infinity"
    wait_for_ready $srcrunning

    # Commit the image for testing non-running containers
    run_podman commit -q $srcrunning
    cpimage="$output"

    # format is: <id> | <source arg to cp> | <destination arg (appended to $srcdir) to cp> | <full dest path (appended to $srcdir)> | <test name>
    tests="
0 | /tmp/containerfile    | /        | /containerfile  | /
0 | /tmp/dotfile.         | /        | /dotfile.       | /
0 | /tmp/containerfile    | /        | /containerfile  | /
0 | /tmp/containerfile    | /.       | /containerfile  | /.
0 | /tmp/containerfile    | /newfile | /newfile        | /newfile
1 | containerfile1        | /        | /containerfile1 | copy from workdir (rel path) to /
2 | subdir/containerfile2 | /        | /containerfile2 | copy from workdir/subdir (rel path) to /
"

    defer-assertion-failures

    # From RUNNING container
    local -a destcontainers=()
    while read id src dest dest_fullname description; do
        # dest may be "''" for empty table cells
        if [[ $dest == "''" ]];then
            unset dest
        fi

        # To RUNNING container
        run_podman run -d $IMAGE sleep infinity
        destcontainer="$output"
        destcontainers+=($destcontainer)
        run_podman cp $srcrunning:$src $destcontainer:"$dest"
        run_podman exec $destcontainer cat "$dest_fullname"
        is "$output" "${randomcontent[$id]}" "$description (cp ctr:$src to $dest - RUNNING)"

        # To CREATED container
        run_podman create $IMAGE sleep infinity
        destcontainer="$output"
        destcontainers+=($destcontainer)
        run_podman cp $srcrunning:$src $destcontainer:"$dest"
        run_podman start $destcontainer
        run_podman exec $destcontainer cat "$dest_fullname"
        is "$output" "${randomcontent[$id]}" "$description (cp ctr:$src to $dest - CREATED)"
    done < <(parse_table "$tests")
    run_podman rm -t 0 -f $srcrunning ${destcontainers[@]}

    # From CREATED container
    destcontainers=()
    local srccreated="c-src-crt-$(safename)"
    run_podman create --name $srccreated --workdir=/srv $cpimage
    while read id src dest dest_fullname description; do
        # dest may be "''" for empty table cells
        if [[ $dest == "''" ]];then
            unset dest
        fi

        # To RUNNING container
        run_podman run -d $IMAGE sleep infinity
        destcontainer="$output"
        destcontainers+=($destcontainer)
        run_podman cp $srccreated:$src $destcontainer:"$dest"
        run_podman exec $destcontainer cat "$dest_fullname"
        is "$output" "${randomcontent[$id]}" "$description (cp ctr:$src to $dest)"
        # To CREATED container
        run_podman create $IMAGE sleep infinity
        destcontainer="$output"
        destcontainers+=($destcontainer)
        run_podman cp $srccreated:$src $destcontainer:"$dest"
        run_podman start $destcontainer
        run_podman exec $destcontainer cat "$dest_fullname"
        is "$output" "${randomcontent[$id]}" "$description (cp ctr:$src to $dest)"
    done < <(parse_table "$tests")
    run_podman rm -t 0 -f $srccreated ${destcontainers[@]}
    run_podman rmi -f $cpimage
}

@test "podman cp dir from host to container" {
    srcdir=$PODMAN_TMPDIR
    mkdir -p $srcdir/dir/sub
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
    )
    echo "${randomcontent[0]}" > $srcdir/dir/sub/hostfile0
    echo "${randomcontent[1]}" > $srcdir/dir/sub/hostfile1

    # "." and "dir/." will copy the contents, so make sure that a dir ending
    # with dot is treated correctly.
    mkdir -p $srcdir/dir.
    cp -r $srcdir/dir/* $srcdir/dir.

    local destrunning="c-destrun-$(safename)"
    run_podman run -d --name $destrunning --workdir=/srv $IMAGE sh -c "mkdir /srv/subdir; echo READY;sleep infinity"
    wait_for_ready $destrunning

    # Commit the image for testing non-running containers
    run_podman commit -q $destrunning
    cpimage="$output"

    # format is: <source arg to cp (appended to srcdir)> | <destination arg to cp> | <full dest path> | <test name>
    tests="
 dir       | /        | /dir/sub     | copy dir  to root
 dir.      | /        | /dir./sub    | copy dir. to root
 dir/      | /tmp     | /tmp/dir/sub | copy dir/ to tmp
 dir/.     | /usr/    | /usr/sub     | copy dir/. usr/
 dir/sub   | .        | /srv/sub     | copy dir/sub to workdir (rel path)
 dir/sub/. | subdir/. | /srv/subdir  | copy dir/sub/. to workdir subdir (rel path)
 dir       | /newdir1 | /newdir1/sub | copy dir to newdir1
 dir/      | /newdir2 | /newdir2/sub | copy dir/ to newdir2
 dir/.     | /newdir3 | /newdir3/sub | copy dir/. to newdir3
"

    defer-assertion-failures

    # RUNNING container
    while read src dest dest_fullname description; do
        run_podman cp $srcdir/$src $destrunning:$dest
        run_podman exec $destrunning cat $dest_fullname/hostfile0 $dest_fullname/hostfile1
        is "${lines[*]}" "${randomcontent[*]}" "$description (cp -> ctr:$dest - RUNNING)"
    done < <(parse_table "$tests")
    run_podman rm -t 0 -f $destrunning

    # CREATED container
    local destcreated="c-dest-created-$(safename)"
    run_podman create --name $destcreated --workdir=/srv $cpimage sleep infinity
    while read src dest dest_fullname description; do
        run_podman cp $srcdir/$src $destcreated:$dest
        # tests checks are done below
    done < <(parse_table "$tests")

    # Now do the test checks, it is a bit ugly that we do this over two loops but this
    # makes the test faster as we do not have to start/stop the container on every iteration.
    run_podman start $destcreated
    while read src dest dest_fullname description; do
        run_podman exec $destcreated cat $dest_fullname/hostfile0 $dest_fullname/hostfile1
        is "${lines[*]}" "${randomcontent[*]}" "$description (cp -> ctr:$dest - CREATED)"
    done < <(parse_table "$tests")
    run_podman rm -t 0 -f $destcreated

    local destnotdir="c-destnotdir-$(safename)"
    run_podman create --name $destnotdir --workdir=/srv $cpimage sleep infinity
    run_podman 125 cp $srcdir $destnotdir:/etc/os-release
    is "$output" "Error: destination must be a directory when copying a directory" "cannot copy directory to file"
    run_podman rm -t 0 -f $destnotdir

    run_podman rmi -f $cpimage
}

@test "podman cp dir from container to host" {
    destdir=$PODMAN_TMPDIR/cp-test-dir-ctr-to-host
    mkdir -p $destdir

    # Create 2 files with random content in the container.
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
    )

    local srcrunning="c-src-run-$(safename)"
    run_podman run -d --name $srcrunning --workdir=/srv $IMAGE sh -c "mkdir /srv/subdir;
         echo ${randomcontent[0]} > /srv/subdir/containerfile0; \
         echo ${randomcontent[1]} > /srv/subdir/containerfile1; \
         mkdir /tmp/subdir.; cp /srv/subdir/* /tmp/subdir./; \
         echo READY;
         sleep infinity"
    wait_for_ready $srcrunning

    # Commit the image for testing non-running containers
    run_podman commit -q $srcrunning
    cpimage="$output"

    # format is: <source arg to cp (appended to /srv)> | <dest> | <full dest path> | <test name>
    tests="
/srv          |         | /srv/subdir    | copy /srv
/srv          | /newdir | /newdir/subdir | copy /srv to /newdir
/srv/         |         | /srv/subdir    | copy /srv/
/srv/.        |         | /subdir        | copy /srv/.
/srv/.        | /newdir | /newdir/subdir | copy /srv/. to /newdir
/srv/subdir/. |         |                | copy /srv/subdir/.
/tmp/subdir.  |         | /subdir.       | copy /tmp/subdir.
"

    defer-assertion-failures

    # RUNNING container
    while read src dest dest_fullname description; do
        if [[ $dest == "''" ]];then
            unset dest
        fi
        if [[ $dest_fullname == "''" ]];then
            unset dest_fullname
        fi
        run_podman cp $srcrunning:$src $destdir$dest
        is "$(< $destdir$dest_fullname/containerfile0)" "${randomcontent[0]}" "$description RUNNING (containerfile0)"
        is "$(< $destdir$dest_fullname/containerfile1)" "${randomcontent[1]}" "$description RUNNING (containerfile1)"
        rm -rf ${destdir:?}/*
    done < <(parse_table "$tests")
    run_podman rm -t 0 -f $srcrunning

    # CREATED container
    local srccreated="c-src-crt-$(safename)"
    run_podman create --name $srccreated --workdir=/srv $cpimage
    while read src dest dest_fullname description; do
        if [[ $dest == "''" ]];then
            unset dest
        fi
        if [[ $dest_fullname == "''" ]];then
            unset dest_fullname
        fi
        run_podman cp $srccreated:$src $destdir$dest
        is "$(< $destdir$dest_fullname/containerfile0)" "${randomcontent[0]}" "$description CREATED (containerfile0)"
        is "$(< $destdir$dest_fullname/containerfile1)" "${randomcontent[1]}" "$description CREATED (containerfile1)"
        rm -rf ${destdir:?}/*
    done < <(parse_table "$tests")

    touch $destdir/testfile
    run_podman 125 cp $srccreated:/etc/ $destdir/testfile
    is "$output" "Error: destination must be a directory when copying a directory" "cannot copy directory to file"
    run_podman rm -t 0 -f $srccreated

    run_podman rmi -f $cpimage
}

@test "podman cp dir from container to container" {
    # Create 2 files with random content in the container.
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
    )

    local srcrunning="c-src-run-$(safename)"
    run_podman run -d --name $srcrunning --workdir=/srv $IMAGE sh -c "mkdir /srv/subdir;
         echo ${randomcontent[0]} > /srv/subdir/containerfile0; \
         echo ${randomcontent[1]} > /srv/subdir/containerfile1; \
         mkdir /tmp/subdir.; cp /srv/subdir/* /tmp/subdir./; \
         echo READY;
         sleep infinity"
    wait_for_ready $srcrunning

    # Commit the image for testing non-running containers
    run_podman commit -q $srcrunning
    cpimage="$output"

    # format is: <source arg to cp (appended to /srv)> | <dest> | <full dest path> | <test name>
    tests="
/srv          | /       | /srv/subdir    | copy /srv
/srv          | /newdir | /newdir/subdir | copy /srv to /newdir
/srv/         | /       | /srv/subdir    | copy /srv/
/srv/.        | /       | /subdir        | copy /srv/.
/srv/.        | /newdir | /newdir/subdir | copy /srv/. to /newdir
/srv/subdir/. | /       |                | copy /srv/subdir/.
/tmp/subdir.  | /       | /subdir.       | copy /tmp/subdir.
"

    defer-assertion-failures

    # From RUNNING container
    local -a destcontainers=()
    while read src dest dest_fullname description; do
        if [[ $dest_fullname == "''" ]];then
            unset dest_fullname
        fi

        # To RUNNING container
        run_podman run -d $IMAGE sleep infinity
        destcontainer="$output"
        destcontainers+=($destcontainer)
        run_podman cp $srcrunning:$src $destcontainer:"$dest"
        run_podman exec $destcontainer cat "$dest_fullname/containerfile0" "$dest_fullname/containerfile1"
        is "$(echo $output)" "${randomcontent[*]}" "$description - from RUNNING to RUNNING"

        # To CREATED container
        run_podman create $IMAGE sleep infinity
        destcontainer="$output"
        destcontainers+=($destcontainer)
        run_podman cp $srcrunning:$src $destcontainer:"$dest"
        run_podman start $destcontainer
        run_podman exec $destcontainer cat "$dest_fullname/containerfile0" "$dest_fullname/containerfile1"
        is "$(echo $output)" "${randomcontent[*]}" "$description - from RUNNING to CREATED/STARTED"
    done < <(parse_table "$tests")
    run_podman rm -t 0 -f $srcrunning ${destcontainers[@]}

    # From CREATED container
    destcontainers=()
    local srccreated="c-src-crt-$(safename)"
    run_podman create --name $srccreated --workdir=/srv $cpimage
    while read src dest dest_fullname description; do
        if [[ $dest_fullname == "''" ]];then
            unset dest_fullname
        fi

        # To RUNNING container
        run_podman run -d $IMAGE sleep infinity
        destcontainer="$output"
        destcontainers+=($destcontainer)
        run_podman cp $srccreated:$src $destcontainer:"$dest"
        run_podman exec $destcontainer cat "$dest_fullname/containerfile0" "$dest_fullname/containerfile1"
        is "$(echo $output)" "${randomcontent[*]}" "$description - from CREATED to RUNNING"

        # To CREATED container
        run_podman create $IMAGE sleep infinity
        destcontainer="$output"
        destcontainers+=($destcontainer)
        run_podman start $destcontainer
        run_podman cp $srccreated:$src $destcontainer:"$dest"
        run_podman exec $destcontainer cat "$dest_fullname/containerfile0" "$dest_fullname/containerfile1"
        is "$(echo $output)" "${randomcontent[*]}" "$description - from CREATED to CREATED"
    done < <(parse_table "$tests")

    run_podman rm -t 0 -f $srccreated ${destcontainers[@]}
    run_podman rmi -f $cpimage
}

@test "podman cp symlinked directory from container" {
    destdir=$PODMAN_TMPDIR/cp-weird-symlink
    mkdir -p $destdir

    # Create 3 files with random content in the container.
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
    )

    local srcrunning="c-src-run-$(safename)"
    run_podman run -d --name $srcrunning $IMAGE sh -c "echo ${randomcontent[0]} > /tmp/containerfile0; \
         echo ${randomcontent[1]} > /tmp/containerfile1; \
         mkdir /tmp/sub && cd /tmp/sub && ln -s .. weirdlink; \
         echo READY;
         sleep infinity"
    wait_for_ready $srcrunning

    # Commit the image for testing non-running containers
    run_podman commit -q $srcrunning
    cpimage="$output"

    # RUNNING container
    # NOTE: /dest does not exist yet but is expected to be created during copy
    run_podman cp $srcrunning:/tmp/sub/weirdlink $destdir/dest
    for i in 0 1; do
        assert "$(< $destdir/dest/containerfile$i)" = "${randomcontent[$i]}" \
               "eval symlink - running container - file $i/1"
    done

    run_podman rm -t 0 -f $srcrunning
    rm -rf $srcdir/dest

    # CREATED container
    local srccreated="c-src-crt-$(safename)"
    run_podman create --name $srccreated $cpimage
    run_podman cp $srccreated:/tmp/sub/weirdlink $destdir/dest
    for i in 0 1; do
        assert "$(< $destdir/dest/containerfile$i)" = "${randomcontent[$i]}" \
               "eval symlink - created container - file $i/1"
    done
    run_podman rm -t 0 -f $srccreated
    run_podman rmi $cpimage
}

@test "podman cp file from host to container volume" {
    srcdir=$PODMAN_TMPDIR/cp-test-volume
    mkdir -p $srcdir
    echo "This file should be in volume2" > $srcdir/hostfile
    volume1="vol1-$(safename)"
    volume2="vol2-$(safename)"

    run_podman volume create $volume1
    run_podman volume inspect $volume1 --format "{{.Mountpoint}}"
    volume1_mount="$output"
    run_podman volume create $volume2
    run_podman volume inspect $volume2 --format "{{.Mountpoint}}"
    volume2_mount="$output"

    # Create a container using the volume.  Note that copying on not-running
    # containers is allowed, so Podman has to analyze the container paths and
    # check if they are hitting a volume, and eventually resolve to the path on
    # the *host*.
    # This test is extra tricky, as volume2 is mounted into a sub-directory of
    # volume1.  Podman must copy the file into volume2 and not volume1.
    local cpcontainer="c-cp_$(safename)"
    run_podman create --name $cpcontainer -v $volume1:/tmp/volume -v $volume2:/tmp/volume/sub-volume $IMAGE

    run_podman cp $srcdir/hostfile $cpcontainer:/tmp/volume/sub-volume
    is "$(< $volume2_mount/hostfile)" "This file should be in volume2"

    # Volume 1 must be empty.
    run ls $volume1_mount
    is "$output" ""

    run_podman rm -t 0 -f $cpcontainer
    run_podman volume rm $volume1 $volume2
}


@test "podman cp file from host to container mount" {
    srcdir=$PODMAN_TMPDIR/cp-test-mount-src
    mountdir=$PODMAN_TMPDIR/cp-test-mount
    mkdir -p $srcdir $mountdir
    echo "This file should be in the mount" > $srcdir/hostfile

    volume="v-$(safename)"
    run_podman volume create $volume

    # Make it a bit more complex and put the mount on a volume.
    local cpcontainer="c-cp-$(safename)"
    run_podman create --name $cpcontainer -v $volume:/tmp/volume -v $mountdir:/tmp/volume/mount $IMAGE

    run_podman cp $srcdir/hostfile $cpcontainer:/tmp/volume/mount
    is "$(< $mountdir/hostfile)" "This file should be in the mount"

    run_podman rm -t 0 -f $cpcontainer
    run_podman volume rm $volume
}


# Create two random-name random-content files in /tmp in the container
# podman-cp them into the host using '/tmp/*', i.e. asking podman to
# perform wildcard expansion in the container. We should get both
# files copied into the host.
@test "podman cp * - wildcard copy multiple files from container to host" {
    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir

    rand_filename1=$(random_string 20)
    rand_content1=$(random_string 50)
    rand_filename2=$(random_string 20)
    rand_content2=$(random_string 50)

    local cpcontainer="c-cp-$(safename)"
    run_podman run --name $cpcontainer $IMAGE sh -c \
               "echo $rand_content1 >/tmp/$rand_filename1;
                echo $rand_content2 >/tmp/$rand_filename2"

    # cp no longer supports wildcarding
    run_podman 125 cp '$cpcontainer:/tmp/*' $dstdir

    run_podman rm -t 0 -f $cpcontainer
}


# Create a file on the host; make a symlink in the container pointing
# into host-only space. Try to podman-cp that symlink. It should fail.
@test "podman cp - will not recognize symlink pointing into host space" {
    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir
    echo "this file is on the host" >$srcdir/hostfile

    local cpcontainer="c-cp-$(safename)"
    run_podman run --name $cpcontainer $IMAGE \
               sh -c "ln -s $srcdir/hostfile /tmp/badlink"
    # This should fail because, from the container's perspective, the symlink
    # points to a nonexistent file
    run_podman 125 cp "$cpcontainer:/tmp/*" $dstdir/

    # FIXME: this might not be the exactly correct error message
    is "$output" 'Error: "/tmp/\*" could not be found on container.*'

    # make sure there are no files in dstdir
    is "$(/bin/ls -1 $dstdir)" "" "incorrectly copied symlink from host"

    run_podman rm -t 0 -f $cpcontainer
}


# Issue #3829 - like the above, but with a level of indirection in the
# wildcard expansion: create a file on the host; create a symlink in
# the container named 'file1' pointing to this file; then another symlink
# in the container pointing to 'file*' (file star). Try to podman-cp
# this invalid double symlink. It must fail.
@test "podman cp - will not expand globs in host space (#3829)" {
    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir
    echo "This file is on the host" > $srcdir/hostfile

    local cpcontainer="c-cp-$(safename)"
    run_podman run --name $cpcontainer $IMAGE \
               sh -c "ln -s $srcdir/hostfile file1;ln -s file\* copyme"
    run_podman 125 cp $cpcontainer:copyme $dstdir

    is "$output" 'Error: "copyme*" could not be found on container.*'

    # make sure there are no files in dstdir
    is "$(/bin/ls -1 $dstdir)" "" "incorrectly copied symlink from host"

    run_podman rm -t 0 -f $cpcontainer
}


# Another symlink into host space, this one named '*' (star). cp should fail.
@test "podman cp - will not expand wildcard" {
    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir
    echo "This file lives on the host" > $srcdir/hostfile

    local cpcontainer="c-cp-$(safename)"
    run_podman run --name $cpcontainer $IMAGE \
               sh -c "ln -s $srcdir/hostfile /tmp/\*"
    run_podman 125 cp "$cpcontainer:/tmp/*" $dstdir

    is "$output" 'Error: "/tmp/\*" could not be found on container.*'

    # dstdir must be empty
    is "$(/bin/ls -1 $dstdir)" "" "incorrectly copied symlink from host"

    run_podman rm -t 0 -f $cpcontainer
}


# THIS IS EXTREMELY WEIRD. Podman expands symlinks in weird ways.
@test "podman cp into container: weird symlink expansion" {
    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir

    rand_filename1=$(random_string 20)
    rand_content1=$(random_string 50)
    echo $rand_content1 > $srcdir/$rand_filename1

    rand_filename2=$(random_string 20)
    rand_content2=$(random_string 50)
    echo $rand_content2 > $srcdir/$rand_filename2

    rand_filename3=$(random_string 20)
    rand_content3=$(random_string 50)
    echo $rand_content3 > $srcdir/$rand_filename3

    # Create tmp subdirectories in container, most with an invalid 'x' symlink
    # Keep container running so we can exec into it.
    local cpcontainer="c-cp-$(safename)"
    run_podman run -d --name $cpcontainer $IMAGE \
               sh -c "mkdir /tmp/d1;ln -s /tmp/nonesuch1 /tmp/d1/x;
                      mkdir /tmp/d2;ln -s /tmp/nonesuch2 /tmp/d2/x;
                      mkdir /tmp/d3;
                      trap 'exit 0' 15;echo READY;while :;do sleep 0.5;done"
    wait_for_ready $cpcontainer

    # Copy file from host into container, into a file named 'x'
    # Note that the second has a trailing slash, implying a directory.
    # Since that destination directory doesn't exist, the cp will fail
    run_podman cp $srcdir/$rand_filename1 $cpcontainer:/tmp/d1/x
    is "$output" "" "output from podman cp 1"

    run_podman 125 cp $srcdir/$rand_filename2 $cpcontainer:/tmp/d2/x/
    is "$output" "Error: \"/tmp/d2/x/\" could not be found on container $cpcontainer: no such file or directory" "cp will not create nonexistent destination directory"

    run_podman cp $srcdir/$rand_filename3 $cpcontainer:/tmp/d3/x
    is "$output" "" "output from podman cp 3"

    # Read back.
    # In the first case, podman actually creates the file nonesuch1 (i.e.
    # podman expands 'x -> nonesuch1' and, instead of overwriting x,
    # creates an actual file).
    run_podman exec $cpcontainer cat /tmp/nonesuch1
    is "$output" "$rand_content1" "cp creates destination file"


    # cp into nonexistent directory should not mkdir nonesuch2 directory
    run_podman 1 exec $cpcontainer test -e /tmp/nonesuch2

    # In the third case, podman (correctly imo) creates a file named 'x'
    run_podman exec $cpcontainer cat /tmp/d3/x
    is "$output" "$rand_content3" "cp creates file named x"

    run_podman rm -t 0 -f $cpcontainer
}


# rhbz1741718 : file copied into container:/var/lib/foo appears as /foo
# (docker only, never seems to have affected podman. Make sure it never does).
@test "podman cp into a subdirectory matching GraphRoot" {
    # Create tempfile with random name and content
    srcdir=$PODMAN_TMPDIR/cp-test-in
    mkdir -p $srcdir
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)
    echo $rand_content > $srcdir/$rand_filename
    chmod 644 $srcdir/$rand_filename

    # Determine path to podman storage (eg /var/lib/c/s, or $HOME/.local/...)
    run_podman info --format '{{.Store.GraphRoot}}'
    graphroot=$output

    # Create that directory in the container, and keep container running
    local cpcontainer="c-cp-$(safename)"
    run_podman run -d --name $cpcontainer $IMAGE sh -c \
               "mkdir -p $graphroot; echo READY; top"
    wait_for_ready $cpcontainer

    # Copy from host into container.
    run_podman cp $srcdir/$rand_filename $cpcontainer:$graphroot/$rand_filename

    # ls, and confirm it's there.
    run_podman exec $cpcontainer ls -l $graphroot/$rand_filename
    is "$output" "-rw-r--r-- .* 1 .* root .* 51 .* $graphroot/$rand_filename" \
       "File is copied into container in the correct (full) path"

    # Confirm it has the expected content (this is unlikely to ever fail)
    run_podman exec $cpcontainer cat $graphroot/$rand_filename
    is "$output" "$rand_content" "Contents of file copied into container"

    run_podman rm -t 0 -f $cpcontainer
}

@test "podman cp from stdin to container" {
    # Create tempfile with random name and content
    srcdir=$PODMAN_TMPDIR/cp-test-stdin
    mkdir -p $srcdir
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)
    echo $rand_content > $srcdir/$rand_filename
    chmod 644 $srcdir/$rand_filename

    # Now tar it up!
    tar_file=$PODMAN_TMPDIR/archive.tar.gz
    tar -zvcf $tar_file $srcdir

    local cpcontainer="c-cp-$(safename)"
    run_podman run -d --name $cpcontainer $IMAGE sleep infinity

    # NOTE: podman is supposed to auto-detect the gzip compression and
    # decompress automatically.
    #
    # "-" will evaluate to "/dev/stdin" when used a source.
    run_podman cp - $cpcontainer:/tmp < $tar_file
    run_podman exec $cpcontainer cat /tmp/$srcdir/$rand_filename
    is "$output" "$rand_content"
    run_podman exec $cpcontainer rm -rf /tmp/$srcdir

    # Now for "/dev/stdin".
    # Note: while this works, the content ends up in Nirvana.
    #       Same for Docker.
    run_podman cp /dev/stdin $cpcontainer:/tmp < $tar_file

    # Error checks below ...

    # Input stream must be a (compressed) tar archive.
    run_podman 125 cp - $cpcontainer:/tmp < $srcdir/$rand_filename
    is "$output" "Error: source must be a (compressed) tar archive when copying from stdin"

    # Destination must be a directory (on an existing file).
    run_podman exec $cpcontainer touch /tmp/file.txt
    run_podman 125 cp - $cpcontainer:/tmp/file.txt < $tar_file
    is "$output" 'Error: destination must be a directory when copying from stdin' "cp from stdin to existing file"

    # Destination must be a directory (on an absent path).
    run_podman 125 cp - $cpcontainer:/tmp/IdoNotExist < $tar_file
    is "$output" 'Error: destination must be a directory when copying from stdin' "cp from stdin to absent path"

    run_podman rm -t 0 -f $cpcontainer
}


@test "podman cp from container to stdout" {
    srcdir=$PODMAN_TMPDIR/cp-test-stdout
    mkdir -p $srcdir
    rand_content=$(random_string 50)

    local cpcontainer="c-cp-$(safename)"
    run_podman run -d --name $cpcontainer $IMAGE sleep infinity

    run_podman exec $cpcontainer sh -c "echo '$rand_content' > /tmp/file.txt"
    run_podman exec $cpcontainer touch /tmp/empty.txt

    # Make sure that only "-" gets special treatment. "/dev/stdout"
    run_podman 125 cp $cpcontainer:/tmp/file.txt /dev/stdout
    is "$output" 'Error: invalid destination: "/dev/stdout" must be a directory or a regular file'

    # Copying from stdout will always compress.  So let's copy the previously
    # created file from the container via stdout, untar the archive and make
    # sure the file exists with the expected content.
    #
    # NOTE that we can't use run_podman because that uses the BATS 'run'
    # function which redirects stdout and stderr. Here we need to guarantee
    # that podman's stdout is redirected cleanly with no artifacts.

    # Copy file.
    "${PODMAN_CMD[@]}" cp $cpcontainer:/tmp/file.txt - > $srcdir/stdout.tar

    tar xvf $srcdir/stdout.tar -C $srcdir
    is "$(< $srcdir/file.txt)" "$rand_content" "File contents: file.txt"
    if [[ -e "$srcdir/empty.txt" ]]; then
        die "File should not exist, but does: empty.txt"
    fi
    rm -f $srcdir/*

    # Copy directory.
    "${PODMAN_CMD[@]}" cp $cpcontainer:/tmp - > $srcdir/stdout.tar

    tar xvf $srcdir/stdout.tar -C $srcdir
    is "$(< $srcdir/tmp/file.txt)" "$rand_content"  "file.txt contents"
    is "$(< $srcdir/tmp/empty.txt)" ""              "empty.txt contents"

    run_podman rm -t 0 -f $cpcontainer
}

@test "podman cp --overwrite file - ctr/ctr" {
    rand_content_file=$(random_string 50)
    rand_content_dir=$(random_string 50)

    local ctrfile="c-f-$(safename)"
    local ctrdir="c-d-$(safename)"
    run_podman run -d --name $ctrfile $IMAGE sh -c "echo '$rand_content_file' > /tmp/foo; echo READY; sleep infinity"
    run_podman run -d --name $ctrdir  $IMAGE sh -c "mkdir /tmp/foo; echo '$rand_content_dir' > /tmp/foo/file.txt; echo READY; sleep infinity"
    wait_for_ready $ctrfile
    wait_for_ready $ctrdir

    # overwrite a directory with a file
    run_podman 125 cp $ctrfile:/tmp/foo $ctrdir:/tmp
    if ! is_remote; then # remote just returns a 500
        is "$output" ".* error creating \"/tmp/foo\": .*: file exists.*"
    fi
    run_podman cp --overwrite $ctrfile:/tmp/foo $ctrdir:/tmp
    run_podman exec $ctrdir cat /tmp/foo
    is "$output" "$rand_content_file" "$ctrdir:/tmp/foo, after cp --overwrite, is a file"

    # reset the $ctrdir container
    run_podman exec $ctrdir sh -c "rm -rf /tmp/foo; mkdir /tmp/foo; echo '$rand_content_dir' > /tmp/foo/file.txt"

    # overwrite a file with a directory
    run_podman 125 cp $ctrdir:/tmp/foo $ctrfile:/tmp
    if ! is_remote; then # remote just returns a 500
        is "$output" ".* error creating \"/tmp/foo\": .*: file exists.*"
    fi
    run_podman cp --overwrite $ctrdir:/tmp/foo $ctrfile:/tmp
    run_podman exec $ctrfile cat /tmp/foo/file.txt
    is "$output" "$rand_content_dir"

    run_podman rm -t 0 -f $ctrfile $ctrdir
}

@test "podman cp --overwrite file - ctr/host" {
    hostdir=$PODMAN_TMPDIR/cp-test
    mkdir -p $hostdir

    rand_content_file=$(random_string 50)
    rand_content_dir=$(random_string 50)

    local ctrfile="c-f-$(safename)"
    local ctrdir="c-d-$(safename)"
    run_podman run -d --name $ctrfile $IMAGE sh -c "echo '$rand_content_file' > /tmp/foo; echo READY; sleep infinity"
    run_podman run -d --name $ctrdir  $IMAGE sh -c "mkdir /tmp/foo; echo '$rand_content_dir' > /tmp/foo/file.txt; echo READY; sleep infinity"
    wait_for_ready $ctrfile
    wait_for_ready $ctrdir

    # overwrite a directory with a file
    mkdir $hostdir/foo
    run_podman 125 cp $ctrfile:/tmp/foo $hostdir
    if ! is_remote; then # remote just returns a 500
        is "$output" ".* error creating \"/foo\": .*: file exists.*"
    fi
    run_podman cp --overwrite $ctrfile:/tmp/foo $hostdir
    is "$(< $hostdir/foo)" "$rand_content_file"

    # overwrite a file with a directory
    rm -rf $hostdir/foo
    touch $hostdir/foo
    run_podman 125 cp $ctrdir:/tmp/foo $hostdir
    if ! is_remote; then # remote just returns a 500
        is "$output" ".* error creating \"/foo\": .*: file exists.*"
    fi
    run_podman cp --overwrite $ctrdir:/tmp/foo $hostdir
    is "$(< $hostdir/foo/file.txt)" "$rand_content_dir"

    run_podman rm -t 0 -f $ctrfile $ctrdir
}

@test "podman cp --overwrite file - host/ctr" {
    hostdir=$PODMAN_TMPDIR/cp-test
    mkdir -p $hostdir

    rand_content_file=$(random_string 50)
    rand_content_dir=$(random_string 50)

    local ctrfile="c-f-$(safename)"
    local ctrdir="c-d-$(safename)"
    run_podman run -d --name $ctrdir  $IMAGE sh -c "mkdir /tmp/foo; echo READY; sleep infinity"
    run_podman run -d --name $ctrfile $IMAGE sh -c "touch /tmp/foo; echo READY; sleep infinity"
    wait_for_ready $ctrdir
    wait_for_ready $ctrfile

    # overwrite a directory with a file
    echo "$rand_content_file" > $hostdir/foo
    run_podman 125 cp  $hostdir/foo $ctrdir:/tmp
    if ! is_remote; then # remote just returns a 500
        is "$output" ".* error creating \"/tmp/foo\": .*: file exists.*"
    fi
    run_podman cp --overwrite $hostdir/foo $ctrdir:/tmp
    run_podman exec $ctrdir cat /tmp/foo
    is "$output" "$rand_content_file"

    # overwrite a file with a directory
    rm -f $hostdir/foo
    mkdir $hostdir/foo
    echo "$rand_content_dir" > $hostdir/foo/file.txt
    run_podman 125 cp $hostdir/foo $ctrfile:/tmp
    if ! is_remote; then # remote just returns a 500
        is "$output" ".* error creating \"/tmp/foo\": .*: file exists.*"
    fi
    run_podman cp --overwrite $hostdir/foo $ctrfile:/tmp
    run_podman exec $ctrfile cat /tmp/foo/file.txt
    is "$output" "$rand_content_dir" "$ctrfile:/tmp/foo/file.txt, after cp --overwrite"

    run_podman rm -t 0 -f $ctrfile $ctrdir
}

# https://github.com/containers/podman/pull/16498
@test "podman cp - dot notation - host to container" {
    srcdir=$PODMAN_TMPDIR/src
    mkdir -p $srcdir
    mkdir -p $srcdir/test1. $srcdir/test2
    touch $srcdir/test1./file1 $srcdir/test2/file2

    local cname="c-$(safename)"
    run_podman run -d --name=$cname --rm $IMAGE sleep infinity
    run_podman cp $srcdir/test1. $cname:/tmp/foo
    run_podman exec $cname /bin/ls -1 /tmp/foo/
    assert "$output" = "file1" "ls ctr:/tmp/foo: only file1 exists, nothing more"

    run_podman rm -f -t0 $cname
}

@test "podman cp - dot notation - container to host" {
    dstdir=$PODMAN_TMPDIR/dst
    mkdir -p $dstdir

    local cname="c-$(safename)"
    run_podman run -d --name=$cname --rm $IMAGE sh -c "mkdir -p /foo/test1. /foo/test2; touch /foo/test1./file1 /foo/test2/file2; echo READY;sleep infinity"
    wait_for_ready $cname

    run_podman cp $cname:/foo/test1. $dstdir/foo
    run /bin/ls -1 $dstdir/foo
    assert "$output" = "file1" "ls [local]/foo: only file1 was copied, nothing more"

    run_podman rm -f -t0 $cname
}

@test "podman cp - dot notation - container to container" {
    local ctrsrc="c-src-$(safename)"
    run_podman run -d --name=$ctrsrc --rm $IMAGE sh -c "mkdir -p /foo/test1. /foo/test2; touch /foo/test1./file1 /foo/test2/file2; echo READY;sleep infinity"
    wait_for_ready $ctrsrc

    local ctrdest="c-test-$(safename)"
    run_podman run -d --name=$ctrdest --rm $IMAGE sleep infinity
    run_podman cp $ctrsrc:/foo/test1. $ctrdest:/foo

    run_podman exec $ctrdest /bin/ls -1 /foo
    assert "$output" = "file1" "ls dest-ctr:/foo: only file1 exists, nothing more"

    run_podman rm -f -t0 $ctrsrc $ctrdest
}

# vim: filetype=sh
