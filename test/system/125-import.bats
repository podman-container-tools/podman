#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman import
#

load helpers

# bats test_tags=ci:parallel
@test "podman import" {
    local archive=$PODMAN_TMPDIR/archive.tar
    local random_content=$(random_string 12)
    # Generate a random name and tag (must be lower-case)
    local random_name=x0$(random_string 12 | tr A-Z a-z)
    local random_tag=t0$(random_string 7 | tr A-Z a-z)
    local fqin=localhost/$random_name:$random_tag

    cname=c-import-$(safename)
    run_podman run --name $cname $IMAGE sh -c "echo ${random_content} > /random.txt"
    run_podman export $cname -o $archive
    run_podman rm -t 0 -f $cname

    # Simple import
    run_podman import -q $archive
    iid="$output"
    run_podman run --rm $iid cat /random.txt
    is "$output" "$random_content" "simple import"
    run_podman rmi -f $iid

    # Simple import via stdin
    run_podman import -q - < <(cat $archive)
    iid="$output"
    run_podman run --rm $iid cat /random.txt
    is "$output" "$random_content" "simple import via stdin"
    run_podman rmi -f $iid

    # Tagged import
    run_podman import -q $archive $fqin
    run_podman run --rm $fqin cat /random.txt
    is "$output" "$random_content" "tagged import"
    run_podman rmi -f $fqin

    # Tagged import via stdin
    run_podman import -q - $fqin < <(cat $archive)
    run_podman run --rm $fqin cat /random.txt
    is "$output" "$random_content" "tagged import via stdin"
    run_podman rmi -f $fqin
}

# Integration tag to catch future breakage in tar, e.g. #19407
# bats test_tags=ci:parallel
@test "podman export, alter tarball, re-import" {
    # Create a test file following test
    mkdir $PODMAN_TMPDIR/tmp
    touch $PODMAN_TMPDIR/testfile1
    echo "modified tar file" >> $PODMAN_TMPDIR/tmp/testfile2

    # Create Dockerfile for test
    dockerfile=$PODMAN_TMPDIR/Dockerfile

    cat >$dockerfile <<EOF
FROM $IMAGE
ADD testfile1 /tmp
WORKDIR /tmp
EOF

    b_img=img-before-$(safename)
    b_cnt=ctr-before-$(safename)
    a_img=img-after-$(safename)
    a_cnt=ctr-after-$(safename)

    # Build from Dockerfile FROM non-existing local image
    # --layers=false needed to work around buildah#5674 parallel flake
    run_podman build -t $b_img --layers=false $PODMAN_TMPDIR
    run_podman create --name $b_cnt $b_img

    # Export built container as tarball
    run_podman export -o $PODMAN_TMPDIR/$b_cnt.tar $b_cnt
    run_podman rm -t 0 -f $b_cnt

    # Modify tarball contents
    echo "$_LOG_PROMPT tar --delete -f (tmpdir)/$b_cnt.tar tmp/testfile1"
    tar --delete -f $PODMAN_TMPDIR/$b_cnt.tar tmp/testfile1
    echo "$_LOG_PROMPT tar -C (tmpdir) -rf (tmpdir)/$b_cnt.tar tmp/testfile2"
    tar -C $PODMAN_TMPDIR -rf $PODMAN_TMPDIR/$b_cnt.tar tmp/testfile2

    # Import tarball and Tag imported image
    run_podman import -q $PODMAN_TMPDIR/$b_cnt.tar \
        --change "CMD sh -c \
        \"trap 'exit 33' 2;
        while true; do sleep 0.05;done\"" $a_img

    # Run imported image to confirm tarball modification, block on non-special signal
    run_podman run --name $a_cnt -d $a_img

    # Confirm testfile1 is deleted from tarball
    run_podman 1 exec $a_cnt cat /tmp/testfile1
    is "$output" ".*can't open '/tmp/testfile1': No such file or directory"

    # Confirm testfile2 is added to tarball
    run_podman exec $a_cnt cat /tmp/testfile2
    is "$output" "modified tar file" "modify tarball content"

    # Kill can send non-TERM/KILL signal to container to exit
    run_podman kill --signal 2 $a_cnt
    run_podman wait $a_cnt

    # Confirm exit within timeout
    run_podman ps -a --filter name=$a_cnt --format '{{.Status}}'
    is "$output" "Exited (33) .*" "Exit by non-TERM/KILL"

    run_podman rm -t 0 -f $a_cnt
    run_podman rmi $b_img $a_img

}
