#!/usr/bin/env bats

load helpers

@test "podman container runlabel test" {
    skip_if_remote "container runlabel is not supported for remote"
    tmpdir=$PODMAN_TMPDIR/runlabel-test
    mkdir -p $tmpdir
    containerfile=$tmpdir/Containerfile
    rand1=$(random_string 30)
    rand2=$(random_string 30)
    rand3=$(random_string 30)
    cat >$containerfile <<EOF
FROM $IMAGE
LABEL  INSTALL  podman  run  -t  -i  --rm  \\\${OPT1}  --privileged  -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e HOMEDIR=\\\$HOME -e NAME=\\\${NAME} -e IMAGE=\\\${IMAGE} -e CONFDIR=/etc/\\\${NAME} -e LOGDIR=/var/log/\\\${NAME} -e DATADIR=/var/lib/\\\${NAME} \\\${IMAGE} \\\${OPT2} /bin/install.sh \\\${OPT3}
EOF

    run_podman build -t runlabel_image $tmpdir

    run_podman container runlabel --opt1=${rand1} --opt2=${rand2} --opt3=${rand3} --name test1 --display  install runlabel_image
    is "$output"   "command: ${PODMAN} run -t -i --rm ${rand1} --privileged -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e HOMEDIR=${HOME} -e NAME=test1 -e IMAGE=localhost/runlabel_image:latest -e CONFDIR=/etc/test1 -e LOGDIR=/var/log/test1 -e DATADIR=/var/lib/test1 localhost/runlabel_image:latest ${rand2} /bin/install.sh ${rand3}"   "generating runlabel install command"

    run_podman container runlabel --opt3=${rand3} --display  install runlabel_image
    is "$output"   "command: ${PODMAN} run -t -i --rm --privileged -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e HOMEDIR=${HOME} -e NAME=runlabel_image -e IMAGE=localhost/runlabel_image:latest -e CONFDIR=/etc/runlabel_image -e LOGDIR=/var/log/runlabel_image -e DATADIR=/var/lib/runlabel_image localhost/runlabel_image:latest /bin/install.sh ${rand3}" "generating runlabel without name and --opt1, --opt2"

    run_podman 125 container runlabel --opt1=${rand1} --opt2=${rand2} --opt3=${rand3} --name test1 --display  run runlabel_image
    is "$output"   "Error: cannot find the value of label: run in image: runlabel_image"   "generating runlabel run command"

    run_podman rmi -f runlabel_image
}

# vim: filetype=sh
