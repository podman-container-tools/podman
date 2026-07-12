#!/usr/bin/env bats   -*- bats -*-
#
# podman volume-related tests
#

load helpers

function setup() {
    basic_setup

    run_podman '?' volume rm -a
}

function teardown() {
    run_podman '?' rm -a --volumes
    run_podman '?' volume rm -t 0 -a -f

    basic_teardown
}


# Simple volume tests: share files between host and container
@test "podman run --volumes : basic" {
    run_podman volume list --noheading
    is "$output" "" "baseline: empty results from list --noheading"
    run_podman volume list -n
    is "$output" "" "baseline: empty results from list -n"

    # Create three temporary directories
    vol1=${PODMAN_TMPDIR}/v1_$(random_string)
    vol2=${PODMAN_TMPDIR}/v2_$(random_string)
    vol3=${PODMAN_TMPDIR}/v3_$(random_string)
    mkdir $vol1 $vol2 $vol3

    # In each directory, write a random string to a file
    echo $(random_string) >$vol1/file1_in
    echo $(random_string) >$vol2/file2_in
    echo $(random_string) >$vol3/file3_in

    # Run 'cat' on each file, and compare against local files. Mix -v / --volume
    # flags, and specify them out of order just for grins. The shell wildcard
    # expansion must sort vol1/2/3 lexically regardless.
    v_opts="-v $vol1:/vol1:z --volume $vol3:/vol3:z -v $vol2:/vol2:z"
    run_podman run --rm $v_opts $IMAGE sh -c "cat /vol?/file?_in"

    for i in 1 2 3; do
        eval voldir=\$vol${i}
        is "${lines[$(($i - 1))]}" "$(< $voldir/file${i}_in)" \
           "contents of /vol${i}/file${i}_in"
    done

    # Confirm that container sees vol1 as a mount point
    run_podman run --rm $v_opts $IMAGE mount
    is "$output" ".* on /vol1 type .*" "'mount' in container lists vol1"

    # Have the container do write operations, confirm them on host
    out1=$(random_string)
    run_podman run --rm $v_opts $IMAGE sh -c "echo $out1 >/vol1/file1_out;
                                              cp /vol2/file2_in /vol3/file3_out"
    is "$(<$vol1/file1_out)" "$out1"              "contents of /vol1/file1_out"
    is "$(<$vol3/file3_out)" "$(<$vol2/file2_in)" "contents of /vol3/file3_out"

    # Writing to read-only volumes: not allowed
    run_podman 1 run --rm -v $vol1:/vol1ro:z,ro $IMAGE sh -c "touch /vol1ro/abc"
    is "$output" ".*Read-only file system"  "touch on read-only volume"
}


@test "podman volume duplicates" {
    vol1=${PODMAN_TMPDIR}/v1_$(random_string)
    vol2=${PODMAN_TMPDIR}/v2_$(random_string)
    mkdir $vol1 $vol2

    # if volumes source and dest match then pass
    run_podman run --rm -v $vol1:/vol1 -v $vol1:/vol1 $IMAGE /bin/true
    run_podman 125 run --rm -v $vol1:$vol1 -v $vol2:$vol1 $IMAGE /bin/true
    is "$output" "Error: $vol1: duplicate mount destination"  "diff volumes mounted on same dest should fail"

    # if named volumes source and dest match then pass
    run_podman run --rm -v vol1:/vol1 -v vol1:/vol1 $IMAGE /bin/true
    run_podman 125 run --rm -v vol1:/vol1 -v vol2:/vol1 $IMAGE /bin/true
    is "$output" "Error: /vol1: duplicate mount destination"  "diff named volumes mounted on same dest should fail"

    # if tmpfs volumes source and dest match then pass
    run_podman run --rm --tmpfs /vol1 --tmpfs /vol1 $IMAGE /bin/true
    run_podman 125 run --rm --tmpfs $vol1 --tmpfs $vol1:ro $IMAGE /bin/true
    is "$output" "Error: $vol1: duplicate mount destination"  "diff named volumes and tmpfs mounted on same dest should fail"

    run_podman 125 run --rm -v vol2:/vol2 --tmpfs /vol2 $IMAGE /bin/true
    is "$output" "Error: /vol2: duplicate mount destination"  "diff named volumes and tmpfs mounted on same dest should fail"

    run_podman 125 run --rm -v $vol1:/vol1 --tmpfs /vol1 $IMAGE /bin/true
    is "$output" "Error: /vol1: duplicate mount destination"  "diff named volumes and tmpfs mounted on same dest should fail"
}

# Filter volumes by name
@test "podman volume filter --name" {
    suffix=$(random_string)
    prefix="volume"

    for i in 1 2; do
        myvolume=${prefix}_${i}_${suffix}
        run_podman volume create $myvolume
        is "$output" "$myvolume" "output from volume create $i"
    done

    run_podman volume ls --filter name=${prefix}_1.+ --format "{{.Name}}"
    is "$output" "${prefix}_1_${suffix}" "--filter name=${prefix}_1.+ shows only one volume"

    # The _1* is intentional as asterisk has different meaning in glob and regexp. Make sure this is regexp
    run_podman volume ls --filter name=${prefix}_1* --format "{{.Name}}"
    is "$output" "${prefix}_1_${suffix}.*${prefix}_2_${suffix}.*" "--filter name=${prefix}_1* shows ${prefix}_1_${suffix} and ${prefix}_2_${suffix}"

    for i in 1 2; do
        run_podman volume rm ${prefix}_${i}_${suffix}
    done
}

# Named volumes
@test "podman volume create / run" {
    myvolume=myvol$(random_string)
    mylabel=$(random_string)

    # Create a named volume
    run_podman volume create -d local --label l=$mylabel  $myvolume
    is "$output" "$myvolume" "output from volume create"

    # Confirm that it shows up in 'volume ls', and confirm values
    run_podman volume ls --format json
    tests="
Name           | $myvolume
Driver         | local
Labels.l       | $mylabel
"
    parse_table "$tests" | while read field expect; do
        actual=$(jq -r ".[0].$field" <<<"$output")
        is "$actual" "$expect" "volume ls .$field"
    done

    # Run a container that writes to a file in that volume
    mountpoint=$(jq -r '.[0].Mountpoint' <<<"$output")
    rand=$(random_string)
    run_podman run --rm --volume $myvolume:/vol $IMAGE sh -c "echo $rand >/vol/myfile"

    # Confirm that the file is visible, with content, outside the container
    is "$(<$mountpoint/myfile)" "$rand" "we see content created in container"

    # Clean up
    run_podman volume rm $myvolume
}

# Removing volumes with --force
@test "podman volume rm --force" {
    run_podman run -d --volume myvol:/myvol $IMAGE top
    cid=$output
    run_podman 2 volume rm myvol
    is "$output" "Error: volume myvol is being used by the following container(s): $cid: volume is being used" "should error since container is running"
    run_podman volume rm myvol --force -t0
}

# Running scripts (executables) from a volume
@test "podman volume: exec/noexec" {
    myvolume=myvol$(random_string)

    run_podman volume create $myvolume
    is "$output" "$myvolume" "output from volume create"

    run_podman volume inspect --format '{{.Mountpoint}}' $myvolume
    mountpoint="$output"

    # Create a script, make it runnable
    rand=$(random_string)
    cat >$mountpoint/myscript <<EOF
#!/bin/sh
echo "got here -$rand-"
EOF
    chmod 755 $mountpoint/myscript

    # By default, volumes are mounted exec, but we have manually added the
    # noexec option. This should fail.
    run_podman 126 run --rm --volume $myvolume:/vol:noexec,z $IMAGE /vol/myscript

    # crun and runc emit different messages, and even runc is inconsistent
    # with itself (output changed some time in 2022?). Deal with all.
    assert "$output" =~ 'exec.* permission denied' "run on volume, noexec"

    # With the default, it should pass
    run_podman run --rm -v $myvolume:/vol:z $IMAGE /vol/myscript
    is "$output" "got here -$rand-" "script in volume is runnable with default (exec)"

    # Clean up
    run_podman volume rm $myvolume
}


# Anonymous temporary volumes, and persistent autocreated named ones
@test "podman volume, implicit creation with run" {
    # No hostdir arg: create anonymous container with random name
    rand=$(random_string)
    run_podman run -v /myvol $IMAGE sh -c "echo $rand >/myvol/myfile"

    run_podman volume ls -q
    tempvolume="$output"

    # We should see the file created in the container
    run_podman volume inspect --format '{{.Mountpoint}}' $tempvolume
    mountpoint="$output"
    test -e "$mountpoint/myfile"
    is "$(< $mountpoint/myfile)" "$rand" "file contents, anonymous volume"

    # Remove the container, using rm --volumes. Volume should now be gone.
    run_podman rm -a --volumes
    run_podman volume ls -q
    is "$output" "" "anonymous volume is removed after container is rm'ed"

    # Create a *named* container. This one should persist after container ends
    myvol=myvol$(random_string)
    rand=$(random_string)

    # Duplicate "-v" confirms #8307, fix for double-lock on same volume
    run_podman run --rm -v $myvol:/myvol:z -v $myvol:/myvol2:z $IMAGE \
               sh -c "echo $rand >/myvol/myfile"
    run_podman volume ls -q
    is "$output" "$myvol" "autocreated named container persists"

    # ...and should be usable, read/write, by a second container
    run_podman run --rm -v $myvol:/myvol:z $IMAGE \
               sh -c "cp /myvol/myfile /myvol/myfile2"

    run_podman volume rm $myvol

    if is_rootless; then
       # Autocreated volumes should also work with keep-id
       # All we do here is check status; podman 1.9.1 would fail with EPERM
       myvol=myvol$(random_string)
       run_podman run --rm -v $myvol:/myvol:z --userns=keep-id $IMAGE \
               touch /myvol/myfile
       run_podman volume rm $myvol
    fi
}


# Podman volume import test
@test "podman volume import test" {
    run_podman volume create --driver local my_vol
    run_podman run --rm -v my_vol:/data $IMAGE sh -c "echo hello >> /data/test"
    run_podman volume create my_vol2

    tarfile=${PODMAN_TMPDIR}/hello$(random_string | tr A-Z a-z).tar
    run_podman volume export my_vol --output=$tarfile
    # we want to use `run_podman volume export my_vol` but run_podman is wrapping EOF
    run_podman volume import my_vol2 - < $tarfile
    rm -f $tarfile
    run_podman run --rm -v my_vol2:/data $IMAGE sh -c "cat /data/test"
    is "$output" "hello" "output from second container"
    run_podman volume rm my_vol
    run_podman volume rm my_vol2
}

# stdout with NULs is easier to test here than in ginkgo
@test "podman volume export to stdout" {
    local volname="myvol_$(random_string 10)"
    local mountpoint="/data$(random_string 8)"

    run_podman volume create $volname
    assert "$output" == "$volname" "volume create emits the name it was given"

    local content="mycontent-$(random_string 20)-the-end"
    run_podman run --rm --volume "$volname:$mountpoint" $IMAGE \
               sh -c "echo $content >$mountpoint/testfile"
    assert "$output" = ""

    # We can't use run_podman because bash can't handle NUL characters.
    # Can't even store them in variables, so we need immediate redirection
    # The "-v" is only for debugging: tar will emit the filename to stderr.
    # If this test ever fails, that may give a clue.
    echo "$_LOG_PROMPT $PODMAN volume export $volname | tar -x ..."
    tar_output="$("${PODMAN_CMD[@]}" volume export $volname | tar -x -v --to-stdout)"
    echo "$tar_output"
    assert "$tar_output" == "$content" "extracted content"

    # Clean up
    run_podman volume rm $volname
}

# Podman volume user test
@test "podman volume user test" {
    is_rootless || skip "only meaningful when run rootless"
    skip_if_remote "not applicable on podman-remote"

    user="1000:2000"
    newuser="100:200"
    tmpdir=${PODMAN_TMPDIR}/volume_$(random_string)
    mkdir $tmpdir
    touch $tmpdir/test1

    run_podman run --name user --user $user -v $tmpdir:/data:U $IMAGE stat -c "%u:%g" /data
    is "$output" "$user" "user should be changed"

    # Now chown the source directory and make sure recursive chown happens
    run_podman unshare chown -R $newuser $tmpdir
    run_podman start --attach user
    is "$output" "$user" "user should be the same"

    # Now chown the file in source directory and make sure recursive chown
    # doesn't happen
    run_podman unshare chown -R $newuser $tmpdir/test1
    run_podman start --attach user
    is "$output" "$user" "user should be the same"
    # test1 should still be chowned to $newuser
    run_podman unshare stat -c "%u:%g" $tmpdir/test1
    is "$output" "$newuser" "user should not be changed"

    run_podman unshare rm $tmpdir/test1
    run_podman rm user
}


# Confirm that container sees the correct id
@test "podman volume with --userns=keep-id" {
    is_rootless || skip "only meaningful when run rootless"

    myvoldir=${PODMAN_TMPDIR}/volume_$(random_string)
    mkdir $myvoldir
    touch $myvoldir/myfile

    containersconf=${PODMAN_TMPDIR}/containers.conf
    cat >$containersconf <<EOF
[containers]
userns="keep-id"
EOF

    # With keep-id
    run_podman run --rm -v $myvoldir:/vol:z --userns=keep-id $IMAGE \
               stat -c "%u:%g:%s" /vol/myfile
    is "$output" "$(id -u):$(id -g):0" "with keep-id: stat(file in container) == my uid"

    # Without
    run_podman run --rm -v $myvoldir:/vol:z $IMAGE \
               stat -c "%u:%g:%s" /vol/myfile
    is "$output" "0:0:0" "w/o keep-id: stat(file in container) == root"

    # With keep-id from containers.conf
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm -v $myvoldir:/vol:z $IMAGE \
               stat -c "%u:%g:%s" /vol/myfile
    is "$output" "$(id -u):$(id -g):0" "with keep-id from containers.conf: stat(file in container) == my uid"

    # With keep-id from containers.conf overridden with --userns=nomap
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm -v $myvoldir:/vol:z --userns=nomap $IMAGE \
               stat -c "%u:%g:%s" /vol/myfile
    is "$output" "65534:65534:0" "w/o overridden containers.conf keep-id->nomap: stat(file in container) == root"
}


# 'volume prune' identifies and cleans up unused volumes
@test "podman volume prune" {
    # Create four named volumes
    local -a v=()
    for i in 1 2 3 4;do
        vol=myvol${i}$(random_string)
        v[$i]=$vol
        run_podman volume create $vol
    done

    # Create two additional labeled volumes
    for i in 5 6; do
        vol=myvol${i}$(random_string)
        v[$i]=$vol
        run_podman volume create $vol --label "mylabel"
    done

    # (Assert that output is formatted, not a one-line blob: #8011)
    run_podman volume inspect ${v[1]}
    assert "${#lines[*]}" -ge 10 "Output from 'volume inspect'; see #8011"

    # Run two containers: one mounting v1, one mounting v2 & v3
    run_podman run --name c1 --volume ${v[1]}:/vol1 $IMAGE date
    run_podman run --name c2 --volume ${v[2]}:/vol2 -v ${v[3]}:/vol3 \
               $IMAGE date

    # List available volumes for pruning after using 1,2,3 (without -a only anonymous are listed; we have none)
    run_podman volume prune <<< N
    is "${lines[1]}" "No dangling anonymous volumes found" "volume prune without -a lists no volumes when only named volumes exist"

    # List available volumes for pruning after using 1,2,3 and filtering; see #8913 (without -a only anonymous; we have none)
    run_podman volume prune --filter label=mylabel <<< N
    is "${lines[1]}" "No dangling anonymous volumes found" "volume prune without -a and filter: no anonymous volumes to list"

    # prune with -a should remove named unused volumes v4, v5, v6
    run_podman volume prune --force -a
    is "$(echo $(sort <<<$output))" "${v[4]} ${v[5]} ${v[6]}" \
       "volume prune, with 1, 2, 3 in use, deletes only 4, 5, 6"

    # Remove the container using v2 and v3. Prune should now remove those.
    # The 'echo sort' is to get the output sorted and in one line.
    run_podman rm c2
    run_podman volume prune --force -a
    is "$(echo $(sort <<<$output))" "${v[2]} ${v[3]}" \
       "volume prune, after rm c2, deletes volumes 2 and 3"

    # Remove the final container. Prune should now remove v1.
    run_podman rm c1
    run_podman volume prune --force -a
    is "$output"  "${v[1]}" "volume prune, after rm c2 & c1, deletes volume 1"

    # Further prunes are NOPs
    run_podman volume prune --force -a
    is "$output"  "" "no more volumes to prune"
}

@test "podman volume prune without -a keeps named volumes" {
    local -a v=()
    for i in 1 2 3;do
        vol=myvol${i}$(random_string)
        v[$i]=$vol
        run_podman volume create $vol
    done

    run_podman run --name c1 --volume ${v[1]}:/vol1 $IMAGE date

    run_podman volume prune <<< N
    is "${lines[1]}" "No dangling anonymous volumes found" \
       "volume prune without -a does not list named volumes"

    run_podman volume prune --force
    is "$output" "" "volume prune without -a does not delete named volumes"

    run_podman volume exists ${v[2]}
    run_podman volume exists ${v[3]}
}

@test "podman volume prune removes only anonymous unused volumes" {
    # Start clean: create two named volumes and one anonymous (no name)
    local suffix=$(random_string)
    local named1=named1_${suffix}
    local named2=named2_${suffix}

    run_podman volume create $named1
    is "$output" "$named1" "volume create named1"

    run_podman volume create $named2
    is "$output" "$named2" "volume create named2"

    run_podman volume create
    anon_vol=$(echo "$output" | tr -d '\n\r')
    assert "$anon_vol" != "" "anonymous volume create returns a name"

    run_podman volume ls --format '{{.Name}}'
    assert "$output" =~ "$named1" "volume ls includes named1"
    assert "$output" =~ "$named2" "volume ls includes named2"
    assert "$output" =~ "$anon_vol" "volume ls includes anonymous volume"

    run_podman volume prune --force
    assert "$output" =~ "$anon_vol" "volume prune removes the anonymous volume"

    run_podman volume ls --format '{{.Name}}'
    assert "$output" =~ "$named1" "after prune: named1 still present"
    assert "$output" =~ "$named2" "after prune: named2 still present"
    assert "$output" !~ "$anon_vol" "after prune: anonymous volume removed"
}

@test "podman volume type=bind" {
    myvoldir=${PODMAN_TMPDIR}/volume_$(random_string)
    mkdir $myvoldir
    touch $myvoldir/myfile

    myvolume=myvol$(random_string)
    run_podman 125 volume create -o type=bind -o device=/bogus $myvolume
    is "$output" "Error: invalid volume option device for driver 'local': faccessat /bogus: no such file or directory" "should fail with bogus directory not existing"

    run_podman volume create -o type=bind -o device=/$myvoldir $myvolume
    is "$output" "$myvolume" "should successfully create myvolume"

    run_podman run --rm -v $myvolume:/vol:z $IMAGE \
               stat -c "%u:%s" /vol/myfile
    is "$output" "0:0" "w/o keep-id: stat(file in container) == root"
}

@test "podman volume type=tmpfs" {
    myvolume=myvol$(random_string)
    run_podman volume create -o type=tmpfs -o o=size=2M -o device=tmpfs $myvolume
    is "$output" "$myvolume" "should successfully create myvolume"

    run_podman run --rm -v $myvolume:/vol $IMAGE stat -f -c "%T" /vol
    is "$output" "tmpfs" "volume should be tmpfs"

    run_podman run --rm -v $myvolume:/vol $IMAGE sh -c "mount| grep /vol"
    is "$output" "tmpfs on /vol type tmpfs.*size=2048k.*" "size should be set to 2048k"
}

# Named volumes copyup
@test "podman volume create copyup" {
    myvolume=myvol$(random_string)
    mylabel=$(random_string)

    # Create a named volume
    run_podman volume create $myvolume
    is "$output" "$myvolume" "output from volume create"

    # Confirm that it shows up in 'volume ls', and confirm values
    run_podman volume ls --format json
    tests="
Name           | $myvolume
Driver         | local
NeedsCopyUp    | true
NeedsChown    | true
"
    parse_table "$tests" | while read field expect; do
        actual=$(jq -r ".[0].$field" <<<"$output")
        is "$actual" "$expect" "volume ls .$field"
    done

    run_podman run --rm --volume $myvolume:/vol $IMAGE true
    run_podman volume inspect --format '{{ .NeedsCopyUp }}' $myvolume
    is "${output}" "true" "If content in dest '/vol' empty NeedsCopyUP should still be true"
    run_podman volume inspect --format '{{ .NeedsChown }}' $myvolume
    is "${output}" "true" "No copy up occurred so the NeedsChown will still be true"

    run_podman run --rm --volume $myvolume:/etc $IMAGE ls /etc/passwd
    run_podman volume inspect --format '{{ .NeedsCopyUp }}' $myvolume
    is "${output}" "false" "If content in dest '/etc' non-empty NeedsCopyUP should still have happened and be false"
    run_podman volume inspect --format '{{ .NeedsChown }}' $myvolume
    is "${output}" "false" "Content has been copied up into volume, needschown will be false"

    run_podman volume inspect --format '{{.Mountpoint}}' $myvolume
    mountpoint="$output"
    test -e "$mountpoint/passwd"

    # Clean up
    run_podman volume rm -t -1 --force $myvolume
}

@test "podman volume mount" {
    skip_if_remote "podman --remote volume mount not supported"
    myvolume=myvol$(random_string)
    myfile=myfile$(random_string)
    mytext=$(random_string)

    # Create a named volume
    run_podman volume create $myvolume
    is "$output" "$myvolume" "output from volume create"

    if ! is_rootless ; then
        # image mount is hard to test as a rootless user
        # and does not work remotely
        run_podman volume mount ${myvolume}
        mnt=${output}
        echo $mytext >$mnt/$myfile
        run_podman run -v ${myvolume}:/vol:z $IMAGE cat /vol/$myfile
        is "$output" "$mytext" "$myfile should exist within the containers volume and contain $mytext"
        run_podman volume unmount ${myvolume}
    else
        run_podman 125 volume mount ${myvolume}
        is "$output" "Error: cannot run command \"podman volume mount\" in rootless mode, must execute.*podman unshare.*first" "Should fail and complain about unshare"
    fi
}

@test "podman --image-volume" {
    tmpdir=$PODMAN_TMPDIR/volume-test
    mkdir -p $tmpdir
    containerfile=$tmpdir/Containerfile
    cat >$containerfile <<EOF
FROM $IMAGE
VOLUME /data
EOF
    run_podman build -t volume_image $tmpdir

    containersconf=$tmpdir/containers.conf
    cat >$containersconf <<EOF
[engine]
image_volume_mode="tmpfs"
EOF

    run_podman run --image-volume tmpfs --rm volume_image stat -f -c %T /data
    is "$output" "tmpfs" "Should be tmpfs"

    run_podman 1 run --image-volume ignore --rm volume_image stat -f -c %T /data
    is "$output" "stat: can't read file system information for '/data': No such file or directory" "Should fail with /data does not exist"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm volume_image stat -f -c %T /data
    is "$output" "tmpfs" "Should be tmpfs"

    # Get the hostfs first so we can match it below. The important check is
    # the HEX filesystem type (%t); readable one (%T) is for ease of debugging.
    # We can't compare %T because our alpine-based testimage doesn't grok btrfs.
    run_podman info --format {{.Store.GraphRoot}}
    hostfs=$(stat -f -c '%t %T' $output)
    echo "# for debug: stat( $output ) = '$hostfs'"

    # "${foo%% *}" strips everything after the first space: "9123683e btrfs" -> "9123683e"
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --image-volume anonymous --rm volume_image stat -f -c '%t %T' /data
    assert "${output%% *}" == "${hostfs%% *}" "/data fs type should match hosts graphroot"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --image-volume tmpfs --rm volume_image stat -f -c %T /data
    is "$output" "tmpfs" "Should be tmpfs"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman 1 run --image-volume ignore --rm volume_image stat -f -c %T /data
    is "$output" "stat: can't read file system information for '/data': No such file or directory" "Should fail with /data does not exist"

    run_podman rm --all --force -t 0
    run_podman image rm --force localhost/volume_image
}

@test "podman volume rm --force bogus" {
    run_podman 1 volume rm bogus
    is "$output" "Error: no volume with name \"bogus\" found: no such volume" "Should print error"
    run_podman volume rm --force bogus
    is "$output" "" "Should print no output"

    run_podman volume create testvol
    run_podman volume rm --force bogus testvol
    assert "$output" = "testvol" "removed volume"

    run_podman volume ls -q
    assert "$output" = "" "no volumes"
}

@test "podman ps -f" {
    vol1="/v1_$(random_string)"
    run_podman run -d --rm --volume ${PODMAN_TMPDIR}:$vol1 $IMAGE top
    cid=$output

    run_podman ps --noheading --no-trunc -q -f volume=$vol1
    is "$output" "$cid" "Should find container by volume"

    run_podman ps --noheading --no-trunc -q --filter volume=/NoSuchVolume
    is "$output" "" "ps --filter volume=/NoSuchVolume"

    # Clean up
    run_podman rm -f -t 0 -a
}

@test "podman run with building volume and selinux file label" {
    skip_if_no_selinux
    run_podman create --security-opt label=filetype:usr_t --volume myvol:/myvol $IMAGE top
    run_podman volume inspect myvol --format '{{ .Mountpoint }}'
    path=${output}
    run ls -Zd $path
    is "$output" "system_u:object_r:usr_t:s0 $path" "volume should be labeled with usr_t type"
    run_podman volume rm myvol --force
}

@test "podman volume create --ignore - do not chown" {
    local user_id=2000
    local group_id=2000
    local volume_name=$(random_string)

    # Create a volume and get its mount point
    run_podman volume create --ignore ${volume_name}
    run_podman volume inspect --format '{{.Mountpoint}}' $volume_name
    mountpoint="$output"

    # Run a container with the volume mounted
    run_podman run --rm --user ${group_id}:${user_id} -v ${volume_name}:/vol $IMAGE

    # Podman chowns the mount point according to the user used in the previous command
    local original_owner=$(stat --format %g:%u ${mountpoint})

    # Creating an existing volume with ignore should be a noop
    run_podman volume create --ignore ${volume_name}

    # Verify that the mountpoint was not chowned
    owner=$(stat --format %g:%u ${mountpoint})
    is "$owner" "${original_owner}" "The volume was chowned by podman volume create"

    run_podman volume rm $volume_name --force
}

@test "podman volume ls --format Labels is a string" {
    rand_value=$(random_string 10)
    volume_name="v-$(safename)"

    run_podman volume create --label mylabel=$rand_value $volume_name

    # {{json .Labels}} should produce a JSON string, not a JSON object
    run_podman volume ls --format '{{json .Labels}}'
    assert "$output" =~ "\".*mylabel=${rand_value}.*\"" "json .Labels should be a quoted string, not a JSON object"
    assert "$output" !~ "{" "json .Labels should not contain braces (not a JSON object)"

    # Plain {{.Labels}} should produce key=value format
    run_podman volume ls --format '{{.Labels}}'
    assert "$output" =~ "mylabel=${rand_value}" ".Labels should contain key=value pair"
    assert "$output" !~ "map\[" ".Labels should not use Go map format"

    run_podman volume rm $volume_name
}

# vim: filetype=sh
