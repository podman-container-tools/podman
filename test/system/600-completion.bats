#!/usr/bin/env bats   -*- bats -*-
#
# Test podman shell completion
#
# Shell completion is provided via the cobra library
# It is implement by calling a hidden subcommand called "__complete"
#

load helpers

function setup() {
    # $PODMAN may be a space-separated string, e.g. if we include a --url.
    local -a podman_as_array=($PODMAN)
    # __completeNoDesc must be the first arg if we running the completion cmd
    # set the var for the run_completion function
    PODMAN_COMPLETION="${podman_as_array[0]} __completeNoDesc ${podman_as_array[@]:1}"

    basic_setup
}

# Returns true if we are able to podman-pause
function _can_pause() {
    # Even though we're just trying completion, not an actual unpause,
    # podman barfs with:
    #    Error: unpause is not supported for cgroupv1 rootless containers
    if is_rootless && is_cgroupsv1; then
        return 1
    fi
    return 0
}

function check_shell_completion() {
    local count=0

    # Newline character; used for confirming string output
    local nl="
"

    for cmd in $(_podman_commands "$@"); do
        # Skip the compose command which is calling `docker-compose --help`
        # and hence won't match the assumptions made below.
        if [[ "$cmd" == "compose" ]]; then
            continue
        fi
        # Human-readable podman command string, with multiple spaces collapsed
        name="podman"
        if is_remote; then
            name="podman-remote"
        fi
        command_string="$name $* $cmd"
        command_string=${command_string//  / } # 'podman  x' -> 'podman x'

        run_podman "$@" $cmd --help
        local full_help="$output"

        # The line immediately after 'Usage:' gives us a 1-line synopsis
        usage=$(echo "$full_help" | grep -A1 '^Usage:' | tail -1)
        assert "$usage" != "" "podman $cmd: no Usage message found"

        # If usage ends in '[command]', recurse into subcommands
        if expr "$usage" : '.*\[command\]$' >/dev/null; then
            check_shell_completion "$@" $cmd
            continue
        fi

        # Trim to command path so we only have the args
        args="${usage/$command_string/}"
        # Trim leading whitespaces
        args="${args#"${args%%[![:space:]]*}"}"

        # Extra args is used to match the correct argument number for the command
        # This is important because some commands provide different suggestions based
        # on the number of arguments.
        extra_args=()

        for arg in $args; do

            match=false
            i=0
            while true; do

                case $arg in

                # If we have options than we need to check if we are getting flag completion
                "[options]")
                    # skip this for remote it fails if a command only has the latest flag e.g podman top
                    if ! is_remote; then
                        run_completion "$@" $cmd "--"
                        # If this fails there is most likely a problem with the cobra library
                        is "${lines[0]}" "--.*" \
                           "$* $cmd: flag(s) listed in suggestions"
                        assert "${#lines[@]}" -gt 2 \
                               "$* $cmd: No flag suggestions"
                        _check_completion_end NoFileComp
                    fi
                    # continue the outer for args loop
                    continue 2
                    ;;

                *CONTAINER*)
                    # podman unpause fails early on rootless cgroupsv1
                    if [[ $cmd = "unpause" ]] && ! _can_pause; then
                        continue 2
                    fi

                    name=$random_container_name
                    # special case podman cp suggest containers names with a colon
                    if [[ $cmd = "cp" ]]; then
                        name="$name:"
                    fi

                    run_completion "$@" $cmd "${extra_args[@]}" ""
                    is "$output" ".*-$name${nl}" \
                       "$* $cmd: actual container listed in suggestions"

                    match=true
                    # resume
                    ;;&

                *POD*)
                    run_completion "$@" $cmd "${extra_args[@]}" ""
                    is "$output" ".*-$random_pod_name${nl}" \
                       "$* $cmd: actual pod listed in suggestions"
                    _check_completion_end NoFileComp

                    match=true
                    # resume
                    ;;&

                *IMAGE*)
                    run_completion "$@" $cmd "${extra_args[@]}" ""
                    is "$output" ".*localhost/$random_image_name:$random_image_tag${nl}" \
                       "$* $cmd: actual image listed in suggestions"

                    # check that we complete the image with tag after at least one char is typed
                    run_completion "$@" $cmd "${extra_args[@]}" "${random_image_name:0:1}"
                    is "$output" ".*$random_image_name:$random_image_tag${nl}" \
                       "$* $cmd: image name:tag included in suggestions"

                    # check that we complete the image id after at least two chars are typed
                    run_completion "$@" $cmd "${extra_args[@]}" "${random_image_id:0:2}"
                    is "$output" ".*$random_image_id${nl}" \
                       "$* $cmd: image id included in suggestions when two leading characters present in command line"

                    match=true
                    # resume
                    ;;&

                *NETWORK*)
                    run_completion "$@" $cmd "${extra_args[@]}" ""
                    is "$output" ".*$random_network_name${nl}" \
                       "$* $cmd: actual network listed in suggestions"
                    _check_completion_end NoFileComp

                    match=true
                    # resume
                    ;;&

                *VOLUME*)
                    run_completion "$@" $cmd "${extra_args[@]}" ""
                    is "$output" ".*$random_volume_name${nl}" \
                       "$* $cmd: actual volume listed in suggestions"
                    _check_completion_end NoFileComp

                    match=true
                    # resume
                    ;;&

                *REGISTRY*)
                    run_completion "$@" $cmd "${extra_args[@]}" ""
                    _check_completion_end NoFileComp
                    assert "${#lines[@]}" -gt 2 "$* $cmd: No REGISTRIES found in suggestions"
                    # We can assume quay.io as we force our own CONTAINERS_REGISTRIES_CONF below.
                    assert "${lines[0]}" == "quay.io" "unqualified-search-registries from registries.conf listed"

                    match=true
                    # resume
                    ;;&

                *SECRET*)
                    run_completion "$@" $cmd "${extra_args[@]}" ""
                    is "$output" ".*$random_secret_name${nl}" \
                       "$* $cmd: actual secret listed in suggestions"
                    _check_completion_end NoFileComp

                    match=true
                    # resume
                    ;;&

                *PATH* | *CONTEXT* | *FILE* | *COMMAND* | *ARG...* | *URI*)
                    # default shell completion should be done for everything which accepts a path
                    run_completion "$@" $cmd "${extra_args[@]}" ""

                    # cp is a special case it returns ShellCompDirectiveNoSpace
                    if [[ "$cmd" == "cp" ]]; then
                        _check_completion_end NoSpace
                    else
                        _check_completion_end Default
                        # machine os apply is special and offers images and normal shell completion
                        if [[ "$cmd" != "apply"  ]]; then
                            _check_no_suggestions
                        fi
                    fi
                    ;;

                *)
                    if [[ "$match" == "false" ]]; then
                        dprint "UNKNOWN arg: $arg for $command_string ${extra_args[*]}"
                    fi
                    ;;

                esac

                # Increment the argument array
                extra_args+=("arg")

                i=$(($i + 1))
                # If the argument ends with ...] than we accept 0...n args
                # Loop two times to make sure we are not only completing the first arg
                if [[ ! ${arg} =~ "..." ]] || [[ i -gt 1 ]]; then
                    break
                fi

            done

        done

        # If the command takes no more parameters make sure we are getting no completion
        if [[ ! ${args##* } =~ "..." ]]; then
            run_completion "$@" $cmd "${extra_args[@]}" ""
            _check_completion_end NoFileComp
            _check_no_suggestions
        fi

    done

}

# run the completion cmd
function run_completion() {
    PODMAN="$PODMAN_COMPLETION" run_podman "$@"
}

# check for the given ShellCompDirective (always last line)
function _check_completion_end() {
    is "${lines[-1]}" "Completion ended with directive: ShellCompDirective$1" "Completion has wrong ShellCompDirective set"
}

# Check that there are no suggestions in the output.
# We could only check stdout and not stderr but this is not possible with bats.
# By default we always have two extra lines at the end for the ShellCompDirective.
# Then we could also have other extra lines for debugging, they will always start
# with [Debug], e.g. `[Debug] [Error] no container with name or ID "t12" found: no such container`.
function _check_no_suggestions() {
    if [ ${#lines[@]} -gt 2 ]; then
        # Checking for line count is not enough since we may include additional debug output.
        # Lines starting with [Debug] are allowed.
        local i=0
        length=$((${#lines[@]} - 2))
        while [[ i -lt length ]]; do
            assert "${lines[$i]:0:7}" == "[Debug]"  "Unexpected non-Debug output line: ${lines[$i]}"
            i=$((i + 1))
        done
    fi
}


# bats test_tags=ci:parallel
@test "podman shell completion test" {

    random_container_name="c-$(safename)"
    random_pod_name="p-$(safename)"
    random_image_name="i-$(safename)"
    random_image_tag=$(random_string 5)
    random_network_name="n-$(safename)"
    random_volume_name="v-$(safename)"
    random_secret_name="s-$(safename)"
    random_secret_content=$(random_string 30)
    secret_file=$PODMAN_TMPDIR/$(random_string 10)

    echo $random_secret_content > $secret_file

    # create a container for each state since some commands are only suggesting running container for example
    run_podman create --name created-$random_container_name $IMAGE
    run_podman run --name running-$random_container_name -d $IMAGE top
    run_podman run --name pause-$random_container_name -d $IMAGE top
    if _can_pause; then
        run_podman pause pause-$random_container_name
    fi
    run_podman run --name exited-$random_container_name -d $IMAGE echo exited

    # create pods for each state
    run_podman pod create --name created-$random_pod_name
    run_podman pod create --name running-$random_pod_name
    run_podman pod create --name degraded-$random_pod_name
    run_podman pod create --name exited-$random_pod_name
    run_podman run -d --name running-$random_pod_name-con --pod running-$random_pod_name $IMAGE top
    run_podman run -d --name degraded-$random_pod_name-con --pod degraded-$random_pod_name $IMAGE echo degraded
    run_podman run -d --name exited-$random_pod_name-con --pod exited-$random_pod_name $IMAGE echo exited
    run_podman pod stop exited-$random_pod_name

    # create image name (just tag with new names no need to pull)
    run_podman image tag $IMAGE $random_image_name:$random_image_tag
    run_podman image list --format '{{.ID}}' --filter reference=$random_image_name
    random_image_id="${lines[0]}"

    # create network
    run_podman network create $random_network_name

    # create volume
    run_podman volume create $random_volume_name

    # create secret
    run_podman secret create $random_secret_name $secret_file

    # create our own registries.conf so we know what registry is set
    local CONTAINERS_REGISTRIES_CONF="$PODMAN_TMPDIR/registries.conf"
    echo 'unqualified-search-registries = ["quay.io"]' > "$CONTAINERS_REGISTRIES_CONF"
    export CONTAINERS_REGISTRIES_CONF

    # Called with no args -- start with 'podman --help'. check_shell_completion() will
    # recurse for any subcommands.
    check_shell_completion

    # check inspect with format flag
    run_completion inspect -f "{{."
    assert "$output" =~ ".*^\{\{\.Args\}\}\$.*" "Defaulting to container type is completed"

    run_completion inspect created-$random_container_name -f "{{."
    assert "$output" =~ ".*^\{\{\.Args\}\}\$.*" "Container type is completed"

    run_completion inspect $random_image_name -f "{{."
    assert "$output" =~ ".*^\{\{\.Digest\}\}\$.*" "Image type is completed"

    run_completion inspect $random_volume_name -f "{{."
    assert "$output" =~ ".*^\{\{\.Anonymous\}\}\$.*" "Volume type is completed"

    run_completion inspect created-$random_pod_name -f "{{."
    assert "$output" =~ ".*^\{\{\.BlkioDeviceReadBps\}\}\$.*" "Pod type is completed"

    run_completion inspect $random_network_name -f "{{."
    assert "$output" =~ ".*^\{\{\.DNSEnabled\}\}\$.*" "Network type is completed"

    # cleanup
    run_podman secret rm $random_secret_name
    rm -f $secret_file

    run_podman volume rm $random_volume_name

    run_podman network rm $random_network_name

    run_podman image untag $IMAGE $random_image_name:$random_image_tag

    for state in created running degraded exited; do
        run_podman pod rm -t 0 --force $state-$random_pod_name
    done

    for state in created running pause exited; do
        run_podman rm --force $state-$random_container_name
    done
}

# bats test_tags=ci:parallel
@test "podman shell completion for paths in container/image" {
    skip_if_remote "mounting via remote does not work"
    for cmd in create run; do
        run_completion $cmd $IMAGE ""
        assert "$output" =~ ".*^/etc/\$.*" "etc directory suggested (cmd: podman $cmd)"
        assert "$output" =~ ".*^/home/\$.*" "home directory suggested (cmd: podman $cmd)"
        assert "$output" =~ ".*^/root/\$.*" "root directory suggested (cmd: podman $cmd)"

        # check completion for subdirectory
        run_completion $cmd $IMAGE "/etc"
        # It should be safe to assume the os-release file always exists in $IMAGE
        assert "$output" =~ ".*^/etc/os-release\$.*" "/etc files suggested (cmd: podman $cmd /etc)"
        # check completion for partial file name
        run_completion $cmd $IMAGE "/etc/os-"
        assert "$output" =~ ".*^/etc/os-release\$.*" "/etc files suggested (cmd: podman $cmd /etc/os-)"

        # regression check for https://bugzilla.redhat.com/show_bug.cgi?id=2209809
        # check for relative directory without slash in path.
        run_completion $cmd $IMAGE "e"
        assert "$output" =~ ".*^etc/\$.*" "etc dir suggested (cmd: podman $cmd e)"

        # check completion with relative path components
        # It is important the we will still use the image root and not escape to the host
        run_completion $cmd $IMAGE "../../"
        assert "$output" =~ ".*^../../etc/\$.*" "relative etc directory suggested (cmd: podman $cmd ../../)"
        assert "$output" =~ ".*^../../home/\$.*" "relative home directory suggested (cmd: podman $cmd ../../)"
    done

    ctrname="c-$(safename)"
    random_file=$(random_string 30)
    run_podman run --name $ctrname $IMAGE sh -c "touch /tmp/$random_file && touch /tmp/${random_file}2 && mkdir /emptydir"

    # check completion for podman cp
    run_completion cp ""
    assert "$output" =~ ".*^$ctrname\:\$.*" "podman cp suggest container names"

    run_completion cp "$ctrname:"
    assert "$output" =~ ".*^$ctrname\:/etc/\$.*" "podman cp suggest paths in container"

    run_completion cp "$ctrname:/tmp"
    assert "$output" =~ ".*^$ctrname\:/tmp/$random_file\$.*" "podman cp suggest custom file in container"

    run_completion cp "$ctrname:/tmp/$random_file"
    assert "$output" =~ ".*^$ctrname\:/tmp/$random_file\$.*" "podman cp suggest /tmp/$random_file file in container"
    assert "$output" =~ ".*^$ctrname\:/tmp/${random_file}2\$.*" "podman cp suggest /tmp/${random_file}2 file in container"

    run_completion cp "$ctrname:/emptydir"
    assert "$output" =~ ".*^$ctrname\:/emptydir/\$.*ShellCompDirectiveNoSpace" "podman cp suggest empty dir with no space directive (:2)"

    # cleanup container
    run_podman rm $ctrname
}

# bats test_tags=ci:parallel
@test "podman run --sysctl completion for sysctl" {
    skip_if_remote "sysctl option not working via remote"

    run_completion run --sysctl net.

    assert "$output" =~ "^net\." \
      "Only suggestions with 'net.' should be present for podman run --sysctl net."

    _check_completion_end NoFileComp
}

@test "podman network create --interface-name" {
    run_completion network create --interface-name l

    assert "$output" =~ '.*lo.*' "Loopback interface should be present by default"

    _check_completion_end NoFileComp
}
