# -*- bats -*-

# This lets us do "run -0", which does an implicit exit-status check
bats_require_minimum_version 1.8.0

load helpers

# Create a var-lib-containers dir for this podman. We need to bind-mount
# this into the container, and use --root and --runroot and --tmpdir
# options both in the container podman and out here: that's the only
# way to share image and container storage.
if [ -z "${PODMAN_UPGRADE_WORKDIR}" ]; then
    # Much as I'd love a descriptive name like "podman-upgrade-tests.XXXXX",
    # keep it short ("pu") because of the 100-character path length limit
    # for UNIX sockets (needed by conmon)
    export PODMAN_UPGRADE_WORKDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-${TMPDIR:-/tmp}} pu.XXXXXX)

    touch $PODMAN_UPGRADE_WORKDIR/status
fi

# Generate a set of random strings used for content verification
if [ -z "${RANDOM_STRING_1}" ]; then
    export RANDOM_STRING_1=$(random_string 15)
    export LABEL_CREATED=$(random_string 16)
    export LABEL_FAILED=$(random_string 17)
    export LABEL_RUNNING=$(random_string 18)
    export HOST_PORT=$(random_free_port)
    export MYTESTNETWORK=mytestnetwork$(random_string 8)
fi

# Version string of the podman we're actually testing, e.g. '3.0.0-dev-d1a26013'
PODMAN_VERSION=$($PODMAN version  |awk '/^Version:/ { V=$2 } /^Git Commit:/ { G=$3 } END { print V "-" substr(G,0,8) }')

setup() {
    skip_if_rootless

    # The podman-in-podman image (old podman)
    if [[ -z "$PODMAN_UPGRADE_FROM" ]]; then
        echo "# \$PODMAN_UPGRADE_FROM is undefined (should be e.g. v4.1.0)" >&3
        false
    fi

    if [ "$(< $PODMAN_UPGRADE_WORKDIR/status)" = "failed" ]; then
        skip "*** setup failed - no point in running tests"
    fi

    # cgroup-manager=systemd does not work inside a container
    # skip_mount_home=true is required so we can share the storage mounts between host and container,
    # the default c/storage behavior is to make the mount propagation private.
    export _PODMAN_TEST_OPTS="--storage-opt=skip_mount_home=true --cgroup-manager=cgroupfs --root=$PODMAN_UPGRADE_WORKDIR/root --runroot=$PODMAN_UPGRADE_WORKDIR/runroot --tmpdir=$PODMAN_UPGRADE_WORKDIR/tmp"


    # Starting with v6.0.0 we only test upgrade from versions that support nftables.
    printf "[network]\nfirewall_driver=\"nftables\"\n" > $PODMAN_UPGRADE_WORKDIR/containers.conf
    export CONTAINERS_CONF_OVERRIDE=$PODMAN_UPGRADE_WORKDIR/containers.conf
}

###############################################################################
# BEGIN setup

@test "initial setup: start $PODMAN_UPGRADE_FROM containers" {
    echo failed >| $PODMAN_UPGRADE_WORKDIR/status

    OLD_PODMAN=quay.io/podman/stable:$PODMAN_UPGRADE_FROM
    $PODMAN pull $OLD_PODMAN

    # Shortcut name, because we're referencing it a lot
    pmroot=$PODMAN_UPGRADE_WORKDIR

    # WWW content to share
    mkdir -p $pmroot/var/www
    echo $RANDOM_STRING_1 >$pmroot/var/www/index.txt

    # podman tmpdir
    mkdir -p $pmroot/tmp

    #
    # Script to run >>OLD<< podman commands.
    #
    # These commands will be run inside a podman container. The "podman"
    # command in this script will be the desired old-podman version.
    #
    pmscript=$pmroot/setup
    cat >| $pmscript <<EOF
#!/bin/bash

#
# Argh! podman >= 3.4 something something namespace something, fails with
#   Error: invalid config provided: cannot set hostname when running in the host UTS namespace: invalid configuration
#
# https://github.com/containers/podman/issues/11969#issuecomment-943386484
#
if grep -q utsns /etc/containers/containers.conf; then
    sed -i -e '/^\utsns=/d' /etc/containers/containers.conf
fi

# events-backend=journald does not work inside a container
opts="--events-backend=file $_PODMAN_TEST_OPTS"

set -ex

# Try try again, because network flakiness makes this a point of failure
podman \$opts pull $IMAGE \
  || (sleep 10; podman \$opts pull $IMAGE) \
  || (sleep 30; podman \$opts pull $IMAGE)


podman \$opts create --name mycreatedcontainer --label mylabel=$LABEL_CREATED \
                                               $IMAGE false

podman \$opts run    --name mydonecontainer    $IMAGE echo ++$RANDOM_STRING_1++

podman \$opts run    --name myfailedcontainer  --label mylabel=$LABEL_FAILED \
                                               $IMAGE sh -c 'exit 17' || true

podman \$opts run -d --name myrunningcontainer --label mylabel=$LABEL_RUNNING \
                                               --network bridge \
                                               -p $HOST_PORT:80 \
                                               -p 127.0.0.1:9090-9092:8080-8082 \
                                               -v $pmroot/var/www:/var/www \
                                               -w /var/www \
                                               --mac-address aa:bb:cc:dd:ee:ff \
                                               $IMAGE /bin/busybox-extras httpd -f -p 80

podman \$opts pod create --name mypod

podman \$opts network create --disable-dns $MYTESTNETWORK

echo READY
while :;do
    if [ -e /stop ]; then
        echo STOPPING
        podman \$opts stop -t 0 myrunningcontainer || true
        podman \$opts rm -f     myrunningcontainer || true
        podman \$opts network rm -f $MYTESTNETWORK
        exit 0
    fi
    sleep 0.5
done
EOF
    chmod 555 $pmscript

    # Clean up vestiges of previous run
    $PODMAN rm -f podman_parent

    # Not entirely a NOP! This is just so we get the /run/... mount points created on a CI VM
    $PODMAN run --rm $OLD_PODMAN true

    #
    # Use new-podman to run the above script under old-podman.
    #
    # DO NOT USE run_podman HERE! That would use $_PODMAN_TEST_OPTS
    # and would write into our shared test dir, which would then
    # pollute it for use by old-podman. We must keep that pristine
    # so old-podman is the first to write to it.
    #
    # mount /dev/shm because the container locks are stored there
    # mount /run/containers for the dnsname plugin
    #
    $PODMAN run -d --name podman_parent \
            --privileged \
            --net=host \
            --cgroupns=host \
            --pid=host \
            --env CONTAINERS_CONF_OVERRIDE \
            $v_sconf \
            -v /dev/fuse:/dev/fuse \
            -v /run/crun:/run/crun \
            -v /run/netns:/run/netns:rshared \
            -v /run/containers:/run/containers \
            -v /dev/shm:/dev/shm \
            -v /etc/containers/networks:/etc/containers/networks \
            -v $pmroot:$pmroot:rshared \
            $OLD_PODMAN $pmroot/setup

    _PODMAN_TEST_OPTS= wait_for_ready podman_parent

    echo OK >| $PODMAN_UPGRADE_WORKDIR/status
}

# END   setup
###############################################################################
# BEGIN actual tests

# This is a NOP; used only so the version string will show up in logs
@test "upgrade: $PODMAN_UPGRADE_FROM -> $PODMAN_VERSION" {
    :
}

@test "info - network" {
    run_podman info --format '{{.Host.NetworkBackend}}'
    assert "$output" = "netavark" "Netavark is the only supported network backend"
}

# Whichever DB was picked by old_podman, make sure we honor it
@test "info - database" {
    run_podman info --format '{{.Host.DatabaseBackend}}'
    assert "$output" = "sqlite" "DatabaseBackend for podman >= 4.8"
}

@test "images" {
    run_podman images -a --format '{{.Names}}'
    # Filter out the podman-pause image which isn't present for
    # versions >= 5.5.0
    run -0 grep -v "localhost/podman-pause" <<< "$output"
    assert "${lines[0]}" = "[$IMAGE]" "podman images, line 0"
}

@test "ps : one container running" {
    run_podman ps --format '{{.Image}}--{{.Names}}'
    is "$output" "$IMAGE--myrunningcontainer" "ps: one container running"
}

@test "ps -a : shows all containers" {
    run_podman ps -a \
               --format '{{.Names}}--{{.Status}}--{{.Ports}}--{{.Label "mylabel"}}' \
               --sort=created
    assert "${lines[0]}" == "mycreatedcontainer--Created----$LABEL_CREATED" "line 0, created"
    assert "${lines[1]}" =~ "mydonecontainer--Exited \(0\).*----"   "line 1, done"
    assert "${lines[2]}" =~ "myfailedcontainer--Exited \(17\) .*----$LABEL_FAILED" "line 2, fail"

    # Port order is not guaranteed
    assert "${lines[3]}" =~ "myrunningcontainer--Up .*--$LABEL_RUNNING" "line 3, running"
    assert "${lines[3]}" =~ ".*--.*0\.0\.0\.0:$HOST_PORT->80\/tcp.*--.*"  "line 3, first port forward"
    assert "${lines[3]}" =~ ".*--.*127\.0\.0\.1\:9090-9092->8080-8082\/tcp.*--.*" "line 3, second port forward"

    assert "${lines[4]}" =~ ".*-infra--Created----" "line 4, infra container"

    # For debugging: dump containers and IDs
    if [[ -n "$PODMAN_UPGRADE_TEST_DEBUG" ]]; then
        run_podman ps -a
        for l in "${lines[@]}"; do
            echo "# $l" >&3
        done
    fi
}


@test "inspect - all container status" {
    tests="
running   | running    |  0
created   | created    |  0
done      | exited     |  0
failed    | exited     | 17
"
    while read cname state exitstatus; do
        run_podman inspect --format '{{.State.Status}}--{{.State.ExitCode}}' my${cname}container
        is "$output" "$state--$exitstatus" "status of my${cname}container"
    done < <(parse_table "$tests")
}

@test "inspect - HealthCheck Defaults" {
    run_podman inspect --format '{{.Config.HealthMaxLogSize}}--{{.Config.HealthMaxLogCount}}--{{.Config.HealthLogDestination}}' myrunningcontainer
    assert "$output" == "500--5--local" "HealthCheck Default values of Log size, count and destination"
}

@test "network - curl" {
    run -0 curl --max-time 3 -s 127.0.0.1:$HOST_PORT/index.txt
    is "$output" "$RANDOM_STRING_1" "curl on running container"
}

# IMPORTANT: connect should happen before restart, we want to check
# if we can connect on an existing running container
@test "network - connect" {
    run_podman network connect $MYTESTNETWORK myrunningcontainer
    run_podman network disconnect podman myrunningcontainer
    run -0 curl --max-time 3 -s 127.0.0.1:$HOST_PORT/index.txt
    is "$output" "$RANDOM_STRING_1" "curl on container with second network connected"
}

@test "network - restart" {
    # restart the container and check if we can still use the port
    run_podman stop -t0 myrunningcontainer
    run_podman start myrunningcontainer

    run -0 curl --max-time 3 -s 127.0.0.1:$HOST_PORT/index.txt
    is "$output" "$RANDOM_STRING_1" "curl on restarted container"
}


@test "logs" {
    run_podman logs mydonecontainer
    is "$output" "++$RANDOM_STRING_1++" "podman logs on stopped container"
}

@test "exec" {
    run_podman exec myrunningcontainer cat /var/www/index.txt
    is "$output" "$RANDOM_STRING_1" "exec into myrunningcontainer"
}

@test "load" {
    # FIXME, is this really necessary?
    skip "TBI. Not sure if there's any point to this."
}

@test "mount" {
    skip "TBI"
}

@test "pods" {
    run_podman pod inspect mypod
    is "$output" ".*mypod.*"

    run_podman pod start mypod
    is "$output" "mypod" "podman pod start"

    # run a container in an existing pod
    # FIXME: 2024-02-07 fails: pod X cgroup is not set: internal libpod error
    #run_podman run --pod=mypod --ipc=host --rm $IMAGE echo it works
    #is "$output" ".*it works.*" "podman run --pod"

    run_podman pod ps
    is "$output" ".*mypod.*" "podman pod ps shows name"
    is "$output" ".*Running.*" "podman pod ps shows running state"

    run_podman pod stop mypod
    is "$output" "mypod" "podman pod stop"

    run_podman pod rm mypod
    is "$output" "[0-9a-f]\\{64\\}" "podman pod rm"
}

# FIXME: commit? kill? network? pause? restart? top? volumes? What else?


@test "start" {
    run_podman start -a mydonecontainer
    is "$output" "++$RANDOM_STRING_1++" "start on already-run container"
}

@test "rm a stopped container" {
    run_podman rm myfailedcontainer
    is "$output" "myfailedcontainer" "podman rm myfailedcontainer"

    run_podman rm mydonecontainer
    is "$output" "mydonecontainer" "podman rm mydonecontainer"
}


@test "stop and rm" {
    run_podman stop -t0 myrunningcontainer
    run_podman rm       myrunningcontainer
}

@test "clean up parent" {
    if [[ -n "$PODMAN_UPGRADE_TEST_DEBUG" ]]; then
        skip "workdir is $PODMAN_UPGRADE_WORKDIR"
    fi

    # We're done with shared environment. By clearing this, we can now
    # use run_podman for actions on the podman_parent container
    unset _PODMAN_TEST_OPTS

    # (Useful for debugging the 'rm -f' step below, which, when it fails, only
    # gives a container ID. This 'ps' confirms that the CID is podman_parent)
    run_podman ps -a

    # Stop the container gracefully
    run_podman exec podman_parent touch /stop
    run_podman wait podman_parent

    run_podman 0+we logs podman_parent
    run_podman 0+we rm -f podman_parent

    # Maybe some day I'll understand why podman leaves stray overlay mounts
    while read overlaydir; do
        umount $overlaydir || true
    done < <(mount | grep $PODMAN_UPGRADE_WORKDIR | awk '{print $3}' | sort -r)

    rm -rf $PODMAN_UPGRADE_WORKDIR
}

# FIXME: now clean up
