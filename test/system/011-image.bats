#!/usr/bin/env bats

load helpers

function setup() {
    skip_if_remote "--sign-by does not work with podman-remote"

    basic_setup

    export _GNUPGHOME_TMP=$PODMAN_TMPDIR/.gnupg
    mkdir --mode=0700 $_GNUPGHOME_TMP $PODMAN_TMPDIR/signatures

    cat >$PODMAN_TMPDIR/keydetails <<EOF
    %echo Generating a basic OpenPGP key
    Key-Type: RSA
    Key-Length: 2048
    Subkey-Type: RSA
    Subkey-Length: 2048
    Name-Real: Foo
    Name-Comment: Foo
    Name-Email: foo@bar.com
    Expire-Date: 0
    %no-ask-passphrase
    %no-protection
    # Do a commit here, so that we can later print "done" :-)
    %commit
    %echo done
EOF
    GNUPGHOME=$_GNUPGHOME_TMP gpg --verbose --batch --gen-key $PODMAN_TMPDIR/keydetails
}

function check_signature() {
    # This test requires that $IMAGE be 100% the same as the registry one
    run_podman rmi -a -f
    _prefetch $IMAGE

    local sigfile=$1
    find $PODMAN_TMPDIR/signatures -print
    run_podman inspect --format '{{.Digest}}' $PODMAN_TEST_IMAGE_FQN
    local repodigest=${output/:/=}

    local dir="$PODMAN_TMPDIR/signatures/libpod/${PODMAN_TEST_IMAGE_NAME}@${repodigest}"
    test -d $dir || die "Missing signature directory $dir"
    test -e "$dir/$sigfile" || die "Missing signature file '$sigfile'"

    # Confirm good signature
    run env GNUPGHOME=$_GNUPGHOME_TMP gpg --verify "$dir/$sigfile"
    is "$output" ".*Good signature from .Foo.*<foo@bar.com>" \
       "gpg --verify $sigfile"
}


@test "podman image - sign with no sigfile" {
    GNUPGHOME=$_GNUPGHOME_TMP run_podman image sign --sign-by foo@bar.com --directory $PODMAN_TMPDIR/signatures  "containers-storage:$PODMAN_TEST_IMAGE_FQN"
    check_signature "signature-1"
}

# vim: filetype=sh
