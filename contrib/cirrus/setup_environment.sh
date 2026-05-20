#!/usr/bin/env bash

# This script is intended to be executed early by automation before
# performing other substantial operations.  It relies heavily on
# desired setup information being passed in environment variables
# from Cirrus-CI and/or other orchestration tooling.  To that end,
# VM's must always be considered single-purpose, single-use,
# disposable entities. i.e. One setup, one test, then always discarded.

set -e

# shellcheck source=./contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

showrun echo "starting"

die_unknown() {
    local var_name="$1"
    req_env_vars var_name
    local var_value="${!var_name}"
    die "Unknown/unsupported \$$var_name '$var_value'"
}

msg "************************************************************"
msg "Setting up runtime environment"
msg "************************************************************"
show_env_vars

req_env_vars USER HOME GOSRC SCRIPT_BASE TEST_FLAVOR TEST_ENVIRON \
             PODBIN_NAME PRIV_NAME DISTRO_NV DEST_BRANCH

# Verify basic dependencies
for depbin in go rsync unzip sha256sum curl make python3 git
do
    if ! type -P "$depbin" &> /dev/null
    then
        warn "$depbin binary not found in $PATH"
    fi
done

# Bypass git safety/security checks when operating in a throwaway environment
showrun git config --global --add safe.directory $GOSRC

# Special case: "composefs" is not a valid setting but it's useful for
# readability in .cirrus.yml. Here we translate that to overlayfs (the
# actual filesystem) along with extra magic envariables.
# Be sure to do this before writing /etc/ci_environment.
export CI_DESIRED_COMPOSEFS=
# shellcheck disable=SC2154
if [[ "$CI_DESIRED_STORAGE" = "composefs" ]]; then
    CI_DESIRED_STORAGE="overlay"

    # composefs is root only
    if [[ "$PRIV_NAME" == "root" ]]; then
        CI_DESIRED_COMPOSEFS="+composefs"

        # KLUDGE ALERT! Magic options needed for testing composefs.
        # This option was intended for passing one arg to --storage-opt
        # but we're hijacking it to pass an extra option+arg. And it
        # actually works.
        export STORAGE_OPTIONS_OVERLAY='overlay.use_composefs=true --pull-option=enable_partial_images=true --pull-option=convert_images=true'
    fi
fi

# Ensure that all lower-level contexts and child-processes have
# ready access to higher level orchestration (e.g Cirrus-CI)
# variables.
echo -e "\n# Begin single-use VM global variables (${BASH_SOURCE[0]})" \
    > "/etc/ci_environment"
(
    while read -r env_var; do
        printf -- "%s=%q\n" "${env_var}" "${!env_var}"
    done <<<"$(passthrough_envars)"
) >> "/etc/ci_environment"

# This is a possible manual maintenance gaff, i.e. forgetting to update a
# *_NAME variable in .cirrus.yml.  check to be sure at least one comparison
# matches the actual OS being run.  Ignore details, such as debian point-release
# number and/or '-aarch64' suffix.
# shellcheck disable=SC2154
grep -q "$DISTRO_NV" <<<"$OS_REL_VER" || \
    grep -q "$OS_REL_VER" <<<"$DISTRO_NV" || \
    grep -q "rawhide" <<<"$DISTRO_NV" || \
    die "Automation spec. '$DISTRO_NV'; actual host '$OS_REL_VER'"

# Only allow this script to execute once
if ((${SETUP_ENVIRONMENT:-0})); then
    # Comes from automation library
    # shellcheck disable=SC2154
    warn "Not executing $SCRIPT_FILENAME again"
    exit 0
fi

cd "${GOSRC}/"

mkdir -p /etc/containers/containers.conf.d

# Only cgroups v2 is supported, die if anything else.
[[ "$CG_FS_TYPE" == "cgroup2fs" ]] || \
    die "Only cgroups v2 CI VMs are supported, not: '$CG_FS_TYPE'"

if ((CONTAINER==0)); then  # Not yet running inside a container
    showrun echo "conditional setup for CONTAINER == 0"
    # Discovered reemergence of BFQ scheduler bug in kernel 5.8.12-200
    # which causes a kernel panic when system is under heavy I/O load.
    # Disable the I/O scheduler (a.k.a. elevator) for all environments,
    # leaving optimization up to underlying storage infrastructure.
    testfs="/"  # mountpoint that experiences the most I/O during testing
    msg "Querying block device owning partition hosting the '$testfs' filesystem"
    # Need --nofsroot b/c btrfs appends subvolume label to `source` name
    testdev=$(findmnt --canonicalize --noheadings --nofsroot \
              --output source --mountpoint $testfs)
    msg "    found partition: '$testdev'"
    testdisk=$(lsblk --noheadings --output pkname --paths $testdev)
    msg "    found block dev: '$testdisk'"
    testsched="/sys/block/$(basename $testdisk)/queue/scheduler"
    if [[ -n "$testdev" ]] && [[ -n "$testdisk" ]] && [[ -e "$testsched" ]]; then
        msg "    Found active I/O scheduler: $(cat $testsched)"
        if [[ ! "$(<$testsched)" =~ \[none\]  ]]; then
            msg "    Disabling elevator for '$testsched'"
            echo "none" > "$testsched"
        else
            msg "    Elevator already disabled"
        fi
    else
        warn "Sys node for elevator doesn't exist: '$testsched'"
    fi
fi

# Which distribution are we testing on.
case "$OS_RELEASE_ID" in
    debian)
        showrun echo "No-op conditional setup for debian"
        ;;
    fedora)
        showrun echo "conditional setup for fedora"
        if ((CONTAINER==0)); then
            # All SELinux distros need this for systemd-in-a-container
            msg "Enabling container_manage_cgroup"
            showrun setsebool container_manage_cgroup true
        fi
        ;;
    *) die_unknown OS_RELEASE_ID
esac

# Force the requested storage driver for both system and e2e tests.
# This is (sigh) different because e2e tests have their own special way
# of ignoring system defaults.
# shellcheck disable=SC2154
showrun echo "Setting CI_DESIRED_STORAGE [=$CI_DESIRED_STORAGE$CI_DESIRED_COMPOSEFS] for *system* tests"
conf=/etc/containers/storage.conf
if [[ -e $conf ]]; then
    die "FATAL! INTERNAL ERROR! Cannot override $conf"
fi
cat <<EOF >$conf
[storage]
driver = "$CI_DESIRED_STORAGE"
EOF

if [[ -n "$CI_DESIRED_COMPOSEFS" ]]; then
    # composefs only works as root so we must set it in the rootful config
    mkdir /etc/containers/storage.rootful.conf.d/
    conf=/etc/containers/storage.rootful.conf.d/99-composefs.conf
    cat <<EOF >>$conf

# BEGIN CI-enabled composefs
[storage.options]
pull_options = {enable_partial_images = "true", use_hard_links = "false", ostree_repos="", convert_images = "true"}

[storage.options.overlay]
use_composefs = "true"
# END CI-enabled composefs
EOF
fi

# mount a tmpfs for the container storage to speed up the IO
# side effect is we clear all potentially pre existing data so we know we always start "clean"
mount -t tmpfs -o size=75%,mode=0700 none /var/lib/containers

# shellcheck disable=SC2154
showrun echo "Setting CI_DESIRED_STORAGE [=$CI_DESIRED_STORAGE] for *e2e* tests"
echo "STORAGE_FS=$CI_DESIRED_STORAGE" >>/etc/ci_environment

if ((CONTAINER==0)); then  # not yet inside a container
    # Load null_blk to use /dev/nullb0 for testing block
    # devices limits
    modprobe null_blk nr_devices=1 || :
fi

# Required to be defined by caller: The environment where primary testing happens
# shellcheck disable=SC2154
showrun echo "about to set up for TEST_ENVIRON [=$TEST_ENVIRON]"
case "$TEST_ENVIRON" in
    host)
        # The e2e tests wrongly guess `--cgroup-manager` option
        # under some runtime contexts like rootless.
        # shellcheck disable=SC2154
        if [[ "$CG_FS_TYPE" == "cgroup2fs" ]] || [[ "$PRIV_NAME" == "root" ]]
        then
            warn "Forcing CGROUP_MANAGER=systemd"
            echo "CGROUP_MANAGER=systemd" >> /etc/ci_environment
        else
            warn "Forcing CGROUP_MANAGER=cgroupfs"
            echo "CGROUP_MANAGER=cgroupfs" >> /etc/ci_environment
        fi
        ;;
    container)
        if ((CONTAINER==0)); then  # not yet inside a container
            warn "Force loading iptables modules"
            # Since CRIU 3.11, uses iptables to lock and unlock
            # the network during checkpoint and restore.  Needs
            # the following two modules loaded on the host.
            modprobe ip6table_nat || :
            modprobe iptable_nat || :
        else
            warn "Forcing CGROUP_MANAGER=cgroupfs"
            echo "CGROUP_MANAGER=cgroupfs" >> /etc/ci_environment

            # There's no practical way to detect userns w/in a container
            # affected/related tests are sensitive to this variable.
            warn "Disabling usernamespace integration testing"
            echo "SKIP_USERNS=1" >> /etc/ci_environment

            # In F35 the hard-coded default
            # (from containers-common-1-32.fc35.noarch) is 'journald' despite
            # the upstream repository having this line commented-out.
            # Containerized integration tests cannot run with 'journald'
            # as there is no daemon/process there to receive them.
            cconf="/usr/share/containers/containers.conf"
            note="- commented-out by setup_environment.sh"
            if grep -Eq '^log_driver.+journald' "$cconf"; then
                warn "Patching out $cconf journald log_driver"
                sed -r -i -e "s/^log_driver(.*)/# log_driver\1 $note/" "$cconf"
            fi
        fi
        ;;
    *) die_unknown TEST_ENVIRON
esac

# Required to be defined by caller: Are we testing as root or a regular user
showrun echo "about to set up for PRIV_NAME [=$PRIV_NAME]"
case "$PRIV_NAME" in
    root)
        # shellcheck disable=SC2154
        if [[ "$TEST_FLAVOR" = "sys" || "$TEST_FLAVOR" = "apiv2" ]]; then
            # Used in local image-scp testing
            setup_rootless
            echo "PODMAN_ROOTLESS_USER=$ROOTLESS_USER" >> /etc/ci_environment
            echo "PODMAN_ROOTLESS_UID=$ROOTLESS_UID" >> /etc/ci_environment
        fi
        ;;
    rootless)
        # load kernel modules since the rootless user has no permission to do so
        modprobe ip6_tables || :
        modprobe ip6table_nat || :
        setup_rootless
        ;;
    *) die_unknown PRIV_NAME
esac

# Root user namespace
for which in uid gid;do
    if ! grep -qE '^containers:' /etc/sub$which; then
        echo 'containers:10000000:1048576' >>/etc/sub$which
    fi
done

# FIXME! experimental workaround for #16973, the "lookup cdn03.quay.io" flake.
#
# If you are reading this on or after April 2023:
#   * If we're NOT seeing the cdn03 flake any more, well, someone
#     should probably figure out how to fix systemd-resolved, then
#     remove this workaround.
#
#   * If we're STILL seeing the cdn03 flake, well, this "fix"
#     didn't work and should be removed.
#
# Either way, this block of code should be removed after March 31 2023
# because it creates a system that is not representative of real-world Fedora.
#
# 2024-01-25 update: ha ha. This fix has proven so popular that it is
# being used by other groups who were seeing the cdn03 flake. Looks like
# we're stuck with it.
if ((CONTAINER==0)); then
    nsswitch=/etc/authselect/nsswitch.conf
    if [[ -e $nsswitch ]]; then
        if grep -q -E 'hosts:.*resolve' $nsswitch; then
            showrun echo "Disabling systemd-resolved"
            sed -i -e 's/^\(hosts: *\).*/\1files dns myhostname/' $nsswitch
            systemctl stop systemd-resolved
            rm -f /etc/resolv.conf

            # NetworkManager may already be running, or it may not....
            systemctl start NetworkManager
            sleep 1
            systemctl restart NetworkManager

            # ...and it may create resolv.conf upon start/restart, or it
            # may not. Keep restarting until it does. (Yes, I realize
            # this is cargocult thinking. Don't care. Not worth the effort
            # to diagnose and solve properly.)
            retries=10
            while ! test -e /etc/resolv.conf;do
                retries=$((retries - 1))
                if [[ $retries -eq 0 ]]; then
                    die "Timed out waiting for resolv.conf"
                fi
                systemctl restart NetworkManager
                sleep 5
            done
        fi
    fi
fi

# Required to be defined by caller: Are we testing podman or podman-remote client
# shellcheck disable=SC2154
case "$PODBIN_NAME" in
    podman) ;;
    remote) ;;
    *) die_unknown PODBIN_NAME
esac

# As of July 2024, CI VMs come built-in with a registry.
LCR=/var/cache/local-registry/local-cache-registry
if [[ -x $LCR ]]; then
    # Images in cache registry are prepopulated at the time
    # VMs are built. If any PR adds a dependency on new images,
    # those must be fetched now, at VM start time. This should
    # be rare, and must be fixed in next automation_images build.
    while read new_image; do
        $LCR cache $new_image
    done < <(grep '^[^#]' test/NEW-IMAGES || true)
fi

# Required to be defined by caller: The primary type of testing that will be performed
# shellcheck disable=SC2154
showrun echo "about to set up for TEST_FLAVOR [=$TEST_FLAVOR]"
case "$TEST_FLAVOR" in
    altbuild)
        # Defined in .cirrus.yml
        # shellcheck disable=SC2154
        if [[ "$ALT_NAME" =~ RPM ]]; then
            showrun bigto dnf install -y glibc-minimal-langpack go-rpm-macros rpkg rpm-build shadow-utils-subid-devel
        fi
        ;;
    docker-py)
        remove_packaged_podman_files
        showrun make install PREFIX=/usr ETCDIR=/etc

        virtualenv .venv/docker-py
        source .venv/docker-py/bin/activate
        showrun pip install --upgrade pip
        showrun pip install --requirement $GOSRC/test/python/requirements.txt
        ;;
    build) make clean ;;
    compose_v2)
        showrun dnf -y remove docker-compose
        showrun curl --fail -SL https://github.com/docker/compose/releases/download/v2.32.3/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose
        showrun chmod +x /usr/local/bin/docker-compose
        ;& # Continue with next item
    apiv2)
        msg "Installing previously downloaded/cached packages"
        virtualenv .venv/requests
        source .venv/requests/bin/activate
        showrun pip install --upgrade pip
        showrun pip install --requirement $GOSRC/test/apiv2/python/requirements.txt
        ;&  # continue with next item
    int)
        showrun make .install.ginkgo
        ;&
    sys)
        # when run nightly check for system test leaks
        # shellcheck disable=SC2154
        if [[ "$CIRRUS_CRON" != '' ]]; then
            export PODMAN_BATS_LEAK_CHECK=1
        fi
        ;&
    upgrade_test) ;&
    bud) ;&
    bindings) ;&
    endpoint)
        showrun echo "Entering shared endpoint setup"
        # Use existing host bits when testing is to happen inside a container
        # since this script will run again in that environment.
        # shellcheck disable=SC2154
        if [[ "$TEST_ENVIRON" =~ host ]]; then
            if ((CONTAINER)); then
                die "Refusing to config. host-test in container";
            fi
            remove_packaged_podman_files
            make install PREFIX=/usr ETCDIR=/etc EXTRA_BUILDTAGS="$TEST_BUILD_TAGS"
        elif [[ "$TEST_ENVIRON" == "container" ]]; then
            if ((CONTAINER)); then
                remove_packaged_podman_files
                make install PREFIX=/usr ETCDIR=/etc EXTRA_BUILDTAGS="$TEST_BUILD_TAGS"
            fi
        else
            die "Invalid value for \$TEST_ENVIRON=$TEST_ENVIRON"
        fi

        install_test_configs
        ;;
    farm)
        showrun loginctl enable-linger $ROOTLESS_USER
        showrun ssh $ROOTLESS_USER@localhost systemctl --user enable --now podman.socket
        remove_packaged_podman_files
        showrun make install PREFIX=/usr ETCDIR=/etc
        install_test_configs
        ;;
    machine-linux)
        showrun dnf install -y podman-gvproxy* virtiofsd
        # Bootstrap this link if it isn't yet in the package; xref
        # https://github.com/containers/podman/pull/22920
        if ! test -L /usr/libexec/podman/virtiofsd; then
            showrun ln -sfr /usr/libexec/virtiofsd /usr/libexec/podman/virtiofsd
        fi
        remove_packaged_podman_files
        showrun make install PREFIX=/usr ETCDIR=/etc
        # machine-os image changes too frequently, can't use image cache
        install_test_configs nocache
        ;;
    swagger)
        showrun make .install.swagger
        ;;
    release) ;;
    *) die_unknown TEST_FLAVOR
esac

# See ./contrib/cirrus/CIModes.md.
# Vars defined by cirrus-ci
# shellcheck disable=SC2154
if [[ ! "$OS_RELEASE_ID" =~ "debian" ]] && \
   [[ "$CIRRUS_CHANGE_TITLE" =~ CI:NEXT ]]
then
    showrun echo "Entering setup for CI:NEXT"
    # shellcheck disable=SC2154
    if [[ "$CIRRUS_PR_DRAFT" != "true" ]]; then
        die "Magic 'CI:NEXT' string can only be used on DRAFT PRs"
    fi

    showrun dnf copr enable rhcontainerbot/podman-next -y

    # DNF ignores repos that don't exist.  For example, updates-testing is not
    # enabled on Fedora N-1 CI VMs.  Don't updated everything, isolate just the
    # podman-next COPR updates.
    showrun dnf update -y \
      "--enablerepo=copr:copr.fedorainfracloud.org:rhcontainerbot:podman-next" \
      "--disablerepo=copr:copr.fedorainfracloud.org:sbrivio:passt" \
      "--disablerepo=fedora*" "--disablerepo=updates*"
fi

# Must be the very last command.  Prevents setup from running twice.
echo 'SETUP_ENVIRONMENT=1' >> /etc/ci_environment
echo -e "\n# End of global variable definitions" \
    >> /etc/ci_environment

msg "Global CI Environment vars.:"
grep -Ev '^#' /etc/ci_environment | sort | indent

showrun echo "finished"
