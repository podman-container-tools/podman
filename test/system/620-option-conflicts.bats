#!/usr/bin/env bats   -*- bats -*-
#
# options that cannot be set together
#

load helpers


# bats test_tags=ci:parallel
@test "options that cannot be set together" {
    skip_if_remote "not much point testing remote, and container-cleanup fails anyway"

    tests="
create,run        | --cpu-period=1 | --cpus=2               | $IMAGE
create,run        | --cpu-quota=1  | --cpus=2               | $IMAGE
create,run        | --no-hosts     | --add-host=foo:1.1.1.1 | $IMAGE
container cleanup | --all          | --exec=foo
container cleanup | --exec=foo     | --rmi                  | foo
"

    # Run all tests, continue even if any fail
    defer-assertion-failures

    # FIXME: parse_table is what does all the work, giving us test cases.
    while read subcommands opt1 opt2 args; do
        opt1_name=${opt1%=*}
        opt2_name=${opt2%=*}

        readarray -d, -t subcommand_list <<<$subcommands
        for subcommand in "${subcommand_list[@]}"; do
            run_podman 125 $subcommand $opt1 $opt2 $args
            is "$output" "Error: $opt1_name and $opt2_name cannot be set together" \
               "podman $subcommand $opt1 $opt2"

            # Reverse order
            run_podman 125 $subcommand $opt2 $opt1 $args
            is "$output" "Error: $opt1_name and $opt2_name cannot be set together" \
               "podman $subcommand $opt2 $opt1"
        done
    done < <(parse_table "$tests")

    # Different error message; cannot be tested with the other ones above
    for opt in arch os; do
        for cmd in create pull; do
            run_podman 125 $cmd --platform=foo --$opt=bar sdfsdf
            is "$output" "Error: --platform option can not be specified with --arch or --os" \
               "podman $cmd --platform + --$opt"
        done
    done

    # --userns and --pod have a different error message format
    podname=p-$(safename)
    run_podman pod create --name $podname
    run_podman 125 run --uidmap=0:1000:1000 --pod=$podname $IMAGE true
    is "$output" "Error: cannot set user namespace mode when joining pod with infra container: invalid argument" \
       "podman run --uidmap + --pod"
    run_podman pod rm -f $podname
}


# vim: filetype=sh
