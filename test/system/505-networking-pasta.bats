#!/usr/bin/env bats   -*- bats -*-
#
# SPDX-License-Identifier: Apache-2.0
#
# Networking with pasta(1)
#
# Copyright (c) 2022 Red Hat GmbH
# Author: Stefano Brivio <sbrivio@redhat.com>

load helpers
load helpers.network

# All tests in this file must be able to run in parallel
# bats file_tags=ci:parallel

function setup() {
    basic_setup
    skip_if_not_rootless "pasta networking only available in rootless mode"

    XFER_FILE="${PODMAN_TMPDIR}/pasta.bin"
}

# _set_opt() - meta-helper for pasta_test_do.
#
# Sets an option, but panics if option is already set (e.g. UDP+TCP, IPv4/v6)
function _set_opt() {
    local opt_name=$1
    local -n opt_ref=$1
    local newval=$2

    if [[ -n "$opt_ref" ]]; then
        # $kw sneakily inherited from caller
        die "'$kw' in test name sets $opt_name='$newval', but $opt_name has already been set to '$opt_ref'"
    fi
    opt_ref=$newval
}

# pasta_test_do() - Run tests involving clients and servers
#
# This helper function is invoked without arguments; it determines what to do
# based on the @test name.
function pasta_test_do() {
    local ip_ver iftype proto range delta bind_type bytes

    # Normalize test name back to human-readable form. BATS gives us a
    # sanitized string with non-alnum converted to '-XX' (dash-hexbyte)
    # and spaces converted to underscores. Convert all of those to spaces.
    # This then gives us only the important (mutable) part of the test:
    #
    #    test_TCP_translated_..._forwarding-2c_IPv4-2c_loopback
    # ->      TCP translated ... forwarding    IPv4    loopback
    # ->      TCP translated     forwarding    IPv4    loopback
    local test_name=$(printf "$(sed \
                      -e 's/^test_//'                 \
                      -e 's/-\([0-9a-f]\{2\}\)/ /gI' \
                      -e 's/_/ /g'                   \
                      <<<"${BATS_TEST_NAME}")")

    # We now have the @test name as specified in the script, minus punctuation.
    # From each of the name components, determine an action.
    #
    #    TCP translated port range forwarding  IPv4  loopback
    #    |   |          |    |     |           |     \__ iftype=loopback
    #    |   |          |    |     |           \________ ip_ver=4
    #    |   |          |    |     \____________________ bytes=1
    #    |   |          |    \__________________________ range=3
    #    |   |          \_______________________________ (ignored)
    #    |   \__________________________________________ delta=1
    #    \______________________________________________ proto=tcp
    #
    # Each keyword maps to one option. Conflicts ("TCP ... UDP") are fatal
    # errors, as are unknown keywords.
    for kw in $test_name; do
        case $kw in
            TCP|UDP)           _set_opt proto ${kw,,} ;;
            IPv*)              _set_opt ip_ver $(expr "$kw" : "IPv\(.\)") ;;
            Single)            _set_opt range 1 ;;
            range)             _set_opt range 3 ;;
            Address|Interface) _set_opt bind_type ${kw,,} ;;
            bound)             assert "$bind_type" != "" "WHAT-bound???" ;;
            [Tt]ranslated)     _set_opt delta    1 ;;
            loopback|tap)      _set_opt iftype $kw ;;
            port)              ;;   # always occurs with 'forwarding'; ignore
            forwarding)        _set_opt bytes   1 ;;
            large|small)       _set_opt bytes $kw ;;
            transfer)          assert "$bytes" != "" "'transfer' must be preceded by 'large' or 'small'" ;;
            *)                 die "cannot grok '$kw' in test name" ;;
        esac
    done

    # Sanity checks: all test names must include IPv4/6 and TCP/UDP
    test -n "$ip_ver" || die "Test name must include IPv4 or IPv6"
    test -n "$proto"  || die "Test name must include TCP or UDP"
    test -n "$bytes"  || die "Test name must include 'forwarding' or 'large/small transfer'"

    # Major decision point: simple forwarding test, or multi-byte transfer?
    if [[ $bytes -eq 1 ]]; then
        # Simple forwarding check
        # We can't always determine these from the test name. Use sane defaults.
        range=${range:-1}
        delta=${delta:-0}
        bind_type=${bind_type:-port}
    else
        # Data transfer. Translate small/large to dd-recognizable sizes
        case "$bytes" in
            small)  bytes="2k" ;;
            large)  case "$proto" in
                        tcp) bytes="10M" ;;
                        udp) bytes=$(($(cat /proc/sys/net/core/wmem_default) / 4)) ;;
                        *)   die "Internal error: unknown proto '$proto'" ;;
                    esac
                    ;;
            *)      die "Internal error: unknown transfer size '$bytes'" ;;
        esac

        # On data transfers, no other input args can be set in test name.
        # Confirm that they are not defined, and set to a suitable default.
        kw="something"
        _set_opt range     1
        _set_opt delta     0
        _set_opt bind_type port
    fi

    # Dup check: make sure we haven't already run this combination of settings.
    # This serves two purposes:
    #  1) prevent developer from accidentally copy/pasting the same test
    #  2) make sure our test-name-parsing code isn't missing anything important
    local tests_run=${BATS_FILE_TMPDIR}/tests_run
    touch ${tests_run}
    local testid="IPv${ip_ver} $proto $iftype $bind_type range=$range delta=$delta bytes=$bytes"
    if grep -q -F -x -- "$testid" ${tests_run}; then
        die "Duplicate test! Have already run $testid"
    fi
    echo "$testid" >>${tests_run}

    # Done figuring out test params. Now do the real work.
    # Calculate and set addresses,
    if [ ${ip_ver} -eq 4 ]; then
        skip_if_no_ipv4 "IPv4 not routable on the host"
    elif [ ${ip_ver} -eq 6 ]; then
        skip_if_no_ipv6 "IPv6 not routable on the host"
    else
        skip "Unsupported IP version"
    fi

    if [ ${iftype} = "loopback" ]; then
        local ifname="lo"
    else
        local ifname="$(default_ifname "${ip_ver}")"
    fi

    local addr="$(default_addr "${ip_ver}" "${ifname}")"

    # ports,
    if [ ${range} -gt 1 ]; then
        local port="$(random_free_port_range ${range} ${addr} ${proto})"
        local xport="$((${port%-*} + delta))-$((${port#*-} + delta))"
        local seq="$(echo ${port} | tr '-' ' ')"
        local xseq="$(echo ${xport} | tr '-' ' ')"
    else
        local port=$(random_free_port "" ${addr} ${proto})
        local xport="$((port + delta))"
        local seq="${port} ${port}"
        local xseq="${xport} ${xport}"
    fi

    local proto_upper="$(echo "${proto}" | tr [:lower:] [:upper:])"

    # socat options for first <address> in server ("LISTEN" address types),
    local bind="${proto_upper}${ip_ver}-LISTEN:\${port}"
    if [ "${proto}" = "udp" ]; then
        bind="${bind},null-eof"
    fi

    # socat options for second <address> in server ("STDOUT" or "EXEC"),
    local recvhelper=
    if [ "${bytes}" = "1" ]; then
        recv="STDOUT"
    else
        # To ease debugging in case of problems, use a helper that
        # gives us byte count, hash, and first/last few bytes
        recvhelper=/home/podman/bytecheck
        recv="EXEC:$recvhelper"
    fi

    # and port forwarding configuration for Podman and pasta.
    #
    # TODO: Use Podman options once/if
    # https://github.com/containers/podman/issues/14425 is solved
    case ${bind_type} in
    "interface")
        local pasta_spec=":--${proto}-ports,${addr}%${ifname}/${port}:${xport}"
        local podman_spec=
        ;;
    "address")
        local pasta_spec=
        local podman_spec="[${addr}]:${port}:${xport}/${proto}"
        ;;
    *)
        local pasta_spec=
        local podman_spec="[${addr}]:${port}:${xport}/${proto}"
        ;;
    esac

    # Fill in file for data transfer tests, and expected output strings
    if [ "${bytes}" != "1" ]; then
        dd if=/dev/urandom bs=${bytes} count=1 of="${XFER_FILE}"
        run_podman run -i --rm $IMAGE $recvhelper < ${XFER_FILE}
        local expect="$output"
    else
        printf "x" > "${XFER_FILE}"
        local expect="$(for i in $(seq ${seq}); do printf "x"; done)"
    fi

    # Start server
    local cname="c-socat-$(safename)"
    run_podman run -d --name="$cname" --net=pasta"${pasta_spec}" -p "${podman_spec}" "${IMAGE}" \
                   sh -c 'for port in $(seq '"${xseq}"'); do '\
'                             socat -u '"${bind}"' '"${recv}"' & '\
'                         done; wait'

    # Make sure all ports in the container are bound.
    for cport in $(seq ${xseq}); do
        retries=50
        while [[ $retries -gt 0 ]]; do
            run_podman exec $cname ss -Hln -$ip_ver --$proto sport = "$cport"
            if [[ "$output" =~ "$cport" ]]; then
                break
            fi
            retries=$((retries - 1))
            sleep 0.1
        done
        assert $retries -gt 0 "Timed out waiting for bound port $cport in container"
    done

    if [ ${ip_ver} -eq 6 ] && [ ${iftype} = "tap" ]; then
        # For IPv6 tests via tap interface, we use pairs of link-local
        # addresses to communicate. While we disable DAD on all the
        # (global unicast) addresses we copy from the host, or
        # otherwise set, link-local addresses are automatically added
        # by the kernel with DAD, so we need to wait until it's done.
        sleep 2
    fi

    for hport in $(seq ${seq}); do
        local connect="${proto_upper}${ip_ver}:[${addr}]:${hport}"
        [ "${proto}" = "udp" ] && connect="${connect},shut-null"
        # Ports are bound: we can start the client in the background.
        timeout --foreground -v --kill=5 90 socat -u "OPEN:${XFER_FILE}" "${connect}" &
    done

    # Wait for the client children to finish.
    wait

    # Get server output, --follow is used to wait for the container to exit,
    run_podman logs --follow $cname
    # which should give us the expected output back.
    # ...except, sigh, #23482: seems to be a bug in socat, issues spurious warning
    if [[ "$recv" =~ EXEC ]]; then
        output=$(grep -vE 'socat.*waitpid.*No child process' <<<"$output")
    fi
    assert "${output}" = "${expect}" "Mismatch between data sent and received"

    run_podman rm $cname
}

### Addresses ##################################################################

@test "IPv4 default address assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --rm --net=pasta $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"
    local host_address="$(default_addr 4)"

    assert "${container_address}" = "${host_address}" \
           "Container address not matching host"
}

@test "IPv4 address assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --rm --net=pasta:-a,192.0.2.1 $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"

    assert "${container_address}" = "192.0.2.1" \
           "Container address not matching configured value"
}

@test "No IPv4" {
    skip_if_no_ipv4 "IPv4 not routable on the host"
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --rm --net=pasta:-6 $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"

    assert "${container_address}" = "null" \
           "Container has IPv4 global address with IPv4 disabled"
}

@test "IPv6 default address assignment" {
    skip "Host ipv6 set can vary vastly and may not have one true default address"
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --rm --net=pasta $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"
    local host_addresses="$(all_addr 6)"

    echo "${host_addresses}" | grep -qw "${container_address}"
    assert $? -eq 0 "Container address not matching host"
}

@test "IPv6 address assignment" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --rm --net=pasta:-a,2001:db8::1 $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"

    assert "${container_address}" = "2001:db8::1" \
           "Container address not matching configured value"
}

@test "No IPv6" {
    skip_if_no_ipv6 "IPv6 not routable on the host"
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --rm --net=pasta:-4 $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"

    assert "${container_address}" = "null" \
           "Container has IPv6 global address with IPv6 disabled"
}

@test "podman puts pasta IP in /etc/hosts" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    pname="p-$(safename)"
    ip="$(default_addr 4)"

    run_podman pod create --net=pasta --name "${pname}"
    run_podman run --pod="${pname}" "${IMAGE}" getent hosts "${pname}"

    assert "$(echo ${output} | cut -f1 -d' ')" = "${ip}" "Correct /etc/hosts entry missing"

    run_podman pod rm "${pname}"
}

### Routes #####################################################################

@test "IPv4 default route" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --rm --net=pasta $IMAGE ip -j -4 route show

    local container_route="$(ipv4_get_route_default "${output}")"
    local host_route="$(ipv4_get_route_default)"

    assert "${container_route}" = "${host_route}" \
           "Container route not matching host"
}

@test "IPv4 default route assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --rm --net=pasta:-a,192.0.2.2,-g,192.0.2.1 $IMAGE \
        ip -j -4 route show

    local container_route="$(ipv4_get_route_default "${output}")"

    assert "${container_route}" = "192.0.2.1" \
           "Container route not matching configured value"
}

@test "IPv6 default route" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --rm --net=pasta $IMAGE ip -j -6 route show

    local container_route="$(ipv6_get_route_default "${output}")"
    local host_route="$(ipv6_get_route_default)"

    assert "${container_route}" = "${host_route}" \
           "Container route not matching host"
}

@test "IPv6 default route assignment" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --rm --net=pasta:-a,2001:db8::2,-g,2001:db8::1 $IMAGE \
        ip -j -6 route show

    local container_route="$(ipv6_get_route_default "${output}")"

    assert "${container_route}" = "2001:db8::1" \
           "Container route not matching configured value"
}

### Interfaces #################################################################

@test "Default MTU" {
    run_podman run --rm --net=pasta $IMAGE ip -j link show

    container_tap_mtu="$(ether_get_mtu "${output}")"

    assert "${container_tap_mtu}" = "65520" \
           "Container's default MTU not 65220 bytes by default"
}

@test "MTU assignment" {
    run_podman run --rm --net=pasta:-m,1280 $IMAGE ip -j link show

    container_tap_mtu="$(ether_get_mtu "${output}")"

    assert "${container_tap_mtu}" = "1280" \
           "Container's default MTU not matching configured 1280 bytes"
}

@test "Loopback interface state" {
    run_podman run --rm --net=pasta $IMAGE ip -j link show

    local jq_expr='.[] | select(.link_type == "loopback").flags | '\
'              contains(["UP"])'

    container_loopback_up="$(printf '%s' "${output}" | jq -rM "${jq_expr}")"

    assert "${container_loopback_up}" = "true" \
           "Container's loopback interface not up"
}

### DNS ########################################################################

@test "Basic nameserver lookup" {
    run_podman run --rm --net=pasta $IMAGE nslookup l.root-servers.net
}

@test "Default nameserver forwarding" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    # pasta is the default now so no need to set it
    run_podman run --rm $IMAGE grep nameserver /etc/resolv.conf
    assert "${lines[0]}" == "nameserver 169.254.1.1" "default dns forward server"
}

@test "Custom DNS forward address, IPv4" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    local addr=198.51.100.1

    run_podman run --rm --net=pasta:--dns-forward,$addr \
        $IMAGE grep nameserver /etc/resolv.conf
    assert "${lines[0]}" == "nameserver $addr" "custom dns forward server"

    run_podman run --rm --net=pasta:--dns-forward,$addr \
        $IMAGE nslookup l.root-servers.net $addr
}

@test "Custom DNS forward address, IPv6" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    # TODO: In fact, this requires not just IPv6 connectivity on the
    #       host, but an IPv6 reachable nameserver which is harder to
    #       test for.  We could remove that requirement if pasta could
    #       forward between IPv4 and IPv6 addresses but as of
    #       2024_09_06.6b38f07 that's unsupported.  Skip the test for
    #       now.
    skip "Currently unsupported"
    # local addr=2001:db8::1
    #
    # run_podman run --rm --net=pasta:--dns-forward,$addr \
    #     $IMAGE grep nameserver /etc/resolv.conf
    # assert "${lines[0]}" == "nameserver $addr" "custom dns forward server"
    # run_podman run --rm --net=pasta:--dns-forward,$addr \
    #     $IMAGE nslookup l.root-servers.net $addr
    #
    # TODO: In addition to the IPv6 nameserver requirement above,
    #       there seem to be two problems running this test.  It's
    #       unclear if those are in busybox, musl or pasta.
    #
    # 1. With this, Podman writes "nameserver 2001:db8::1" to
    #    /etc/resolv.conf, without zone, and the query originates from ::1.
    #    Passing:
    #   --dns "2001:db8::2%eth0"
    #    results in:
    #   Error: 2001:db8::2%eth0 is not an ip address
    #    Fix the issue in Podman once we figure out 2. below.
    #
    #
    # run_podman run --dns 2001:db8::1 \
    #   --net=pasta:--dns-forward,2001:db8::1 $IMAGE \
    #   sh -c 'echo 2001:db8::1%eth0 >/etc/resolv.conf; nslookup ::1'
    #
    # 2. This fixes the source address of the query, but the answer is
    #    discarded. Figure out if it's an issue in Busybox, in musl, if we
    #    should just include a full-fledged resolver in the test image, etc.
}

### TCP/IPv4 Port Forwarding ###################################################

@test "Single TCP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Single TCP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "TCP port range forwarding, IPv4, tap" {
    pasta_test_do
}

@test "TCP port range forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Translated TCP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Translated TCP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "TCP translated port range forwarding, IPv4, tap" {
    pasta_test_do
}

@test "TCP translated port range forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Address-bound TCP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Address-bound TCP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Interface-bound TCP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Interface-bound TCP port forwarding, IPv4, loopback" {
    pasta_test_do
}

### TCP/IPv6 Port Forwarding ###################################################

@test "Single TCP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Single TCP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "TCP port range forwarding, IPv6, tap" {
    pasta_test_do
}

@test "TCP port range forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Translated TCP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Translated TCP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "TCP translated port range forwarding, IPv6, tap" {
    pasta_test_do
}

@test "TCP translated port range forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Address-bound TCP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Address-bound TCP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Interface-bound TCP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Interface-bound TCP port forwarding, IPv6, loopback" {
    pasta_test_do
}

### UDP/IPv4 Port Forwarding ###################################################

@test "Single UDP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Single UDP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "UDP port range forwarding, IPv4, tap" {
    pasta_test_do
}

@test "UDP port range forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Translated UDP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Translated UDP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "UDP translated port range forwarding, IPv4, tap" {
    pasta_test_do
}

@test "UDP translated port range forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Address-bound UDP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Address-bound UDP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Interface-bound UDP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Interface-bound UDP port forwarding, IPv4, loopback" {
    pasta_test_do
}

### UDP/IPv6 Port Forwarding ###################################################

@test "Single UDP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Single UDP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "UDP port range forwarding, IPv6, tap" {
    pasta_test_do
}

@test "UDP port range forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Translated UDP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Translated UDP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "UDP translated port range forwarding, IPv6, tap" {
    pasta_test_do
}

@test "UDP translated port range forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Address-bound UDP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Address-bound UDP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Interface-bound UDP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Interface-bound UDP port forwarding, IPv6, loopback" {
    pasta_test_do
}

### TCP/IPv4 transfer ##########################################################

@test "TCP/IPv4 small transfer, tap" {
    pasta_test_do
}

@test "TCP/IPv4 small transfer, loopback" {
    pasta_test_do
}

@test "TCP/IPv4 large transfer, tap" {
    pasta_test_do
}

@test "TCP/IPv4 large transfer, loopback" {
    pasta_test_do
}

### TCP/IPv6 transfer ##########################################################

@test "TCP/IPv6 small transfer, tap" {
    pasta_test_do
}

@test "TCP/IPv6 small transfer, loopback" {
    pasta_test_do
}

@test "TCP/IPv6 large transfer, tap" {
    pasta_test_do
}

@test "TCP/IPv6 large transfer, loopback" {
    pasta_test_do
}

### UDP/IPv4 transfer ##########################################################

@test "UDP/IPv4 small transfer, tap" {
    pasta_test_do
}

@test "UDP/IPv4 small transfer, loopback" {
    pasta_test_do
}

@test "UDP/IPv4 large transfer, tap" {
    pasta_test_do
}

@test "UDP/IPv4 large transfer, loopback" {
    pasta_test_do
}

### UDP/IPv6 transfer ##########################################################

@test "UDP/IPv6 small transfer, tap" {
    pasta_test_do
}

@test "UDP/IPv6 small transfer, loopback" {
    pasta_test_do
}

@test "UDP/IPv6 large transfer, tap" {
    pasta_test_do
}

@test "UDP/IPv6 large transfer, loopback" {
    pasta_test_do
}

### Lifecycle ##################################################################

@test "pasta(1) quits when the namespace is gone" {
    local pidfile="${PODMAN_TMPDIR}/pasta.pid"

    run_podman run --rm "--net=pasta:--pid,${pidfile}" $IMAGE true

    # Allow time for process to vanish, in case there's high load
    local pid=$(< $pidfile)
    local timeout=5
    while [[ $timeout -gt 0 ]]; do
        if ! ps -p $pid; then
            return
        fi

        # Still alive. Wait and retry
        sleep 1
        timeout=$((timeout - 1))
    done

    die "Timed out waiting for pid $pid to terminate"
}

### Options ####################################################################
@test "Unsupported protocol in port forwarding" {
    local port=$(random_free_port "" "" tcp)

    run_podman 126 run --rm --net=pasta -p "${port}:${port}/sctp" $IMAGE true
    is "$output" "Error: .*can't forward protocol: sctp"
}

@test "Use options from containers.conf" {
    skip_if_remote "containers.conf must be set for the server"

    containersconf=$PODMAN_TMPDIR/containers.conf
    mac="9a:dd:31:ea:92:98"
    cat >$containersconf <<EOF
[network]
default_rootless_network_cmd = "pasta"
pasta_options = ["-I", "myname", "--ns-mac-addr", "$mac"]
EOF

    # 2023-06-29 DO NOT INCLUDE "--net=pasta" on this line!
    # This tests containers.conf:default_rootless_network_cmd (pr #19032)
    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman run --rm $IMAGE ip link show myname
    assert "$output" =~ "$mac" "mac address is set on custom interface"

    # now, again but this time overwrite a option on the cli.
    mac2="aa:bb:cc:dd:ee:ff"
    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman run --rm \
        --net=pasta:--ns-mac-addr,"$mac2" $IMAGE ip link show myname
    assert "$output" =~ "$mac2" "mac address from cli is set on custom interface"
}

# https://github.com/containers/podman/issues/22653
@test "pasta/bridge and host.containers.internal" {
    skip_if_no_ipv4 "IPv4 not routable on the host"
    pasta_ip="$(default_addr 4)"
    host_ips=$(ip -4 -j addr | jq -r '.[] | select(.ifname != "lo") | .addr_info[].local')

    netname=n_$(safename)
    run_podman network create $netname

    for network in "pasta" "$netname"; do
        # special exit code logic needed here, it is possible that there is no host.containers.internal
        # when there is only one ip one the host and that one is used by pasta.
        # As such we have to deal with both cases.
        run_podman '?' run --rm --network=$network $IMAGE grep host.containers.internal /etc/hosts
        if [ "$status" -eq 0 ]; then
            assert "$output" !~ "$pasta_ip" "pasta host ip must not be assigned ($network)"
            # even more special we use a new --map-guest-addr pasta option and
            # to map 169.254.1.2 to the host, https://github.com/containers/common/pull/2136
            assert "$host_ips 169.254.1.2" =~ "$(cut -f1 <<<$output)" "ip is one of the host ips ($network)"
        elif [ "$status" -eq 1 ]; then
            # if only pasta ip then we cannot have a host.containers.internal entry
            # make sure this fact is actually the case
            assert "$pasta_ip" == "$host_ips" "pasta ip must the only one one the host ($network)"
        else
            die "unexpected exit code '$status' from grep or podman ($network)"
        fi
    done

    run_podman network rm $netname

    first_host_ip=$(head -n 1 <<<"$host_ips")
    run_podman run --rm --network=pasta:-a,192.168.0.2,-g,192.168.0.1,-n,24 $IMAGE grep host.containers.internal /etc/hosts
    assert "$output" =~ "^($first_host_ip|169.254.1.2)" "uses first host ip or special 169.254.1.2 --map-guest-addr"
}
