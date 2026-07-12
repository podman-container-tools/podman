# -*- bash -*-

### Feature Checks #############################################################

# has_ipv4() - Check if one default route is available for IPv4
function has_ipv4() {
    [ -n "$(ip -j -4 route show | jq -rM '.[] | select(.dst == "default")')" ]
}

# has_ipv6() - Check if one default route is available for IPv6
function has_ipv6() {
    [ -n "$(ip -j -6 route show | jq -rM '.[] | select(.dst == "default")')" ]
}

# skip_if_no_ipv4() - Skip current test if IPv4 traffic can't be routed
# $1:	Optional message to display
function skip_if_no_ipv4() {
    if ! has_ipv4; then
        local msg=$(_add_label_if_missing "$1" "IPv4")
        skip "${msg:-not applicable with no routable IPv4}"
    fi
}

# skip_if_no_ipv6() - Skip current test if IPv6 traffic can't be routed
# $1:	Optional message to display
function skip_if_no_ipv6() {
    if ! has_ipv6; then
        local msg=$(_add_label_if_missing "$1" "IPv6")
        skip "${msg:-not applicable with no routable IPv6}"
    fi
}

### Addresses, Routes, Links ###################################################

# ipv4_get_addr_global() - Print first global IPv4 address reported by netlink
# $1:	Optional output of 'ip -j -4 address show' from a different context
function ipv4_get_addr_global() {
    local expr='[.[].addr_info[] | select(.scope=="global")] | .[0].local'
    echo "${1:-$(ip -j -4 address show)}" | jq -rM "${expr}"
}

# ipv6_get_addr_global() - Print first global IPv6 address reported by netlink
# $1:	Optional output of 'ip -j -6 address show' from a different context
function ipv6_get_addr_global() {
    local expr='[.[].addr_info[] | select(.scope=="global")] | .[0].local'
    echo "${1:-$(ip -j -6 address show)}" | jq -rM "${expr}"
}

# random_rfc1918_subnet() - Pseudorandom unused subnet in 172.16/12 prefix
#
# Use the class B set, because much of our CI environment (Google, RH)
# already uses up much of the class A, and it's really hard to test
# if a block is in use.
#
# This returns THREE OCTETS! It is up to our caller to append .0/24, .255, &c.
#
function random_rfc1918_subnet() {
    local retries=1024

    while [ "$retries" -gt 0 ];do
        # 172.16.0.0 -> 172.31.255.255
        local n1=172
        local n2=$(( 16 + $RANDOM & 15 ))
        local n3=$(( $RANDOM & 255 ))

        if ! subnet_in_use $n1 $n2 $n3; then
            echo "$n1.$n2.$n3"
            return
        fi

        retries=$(( retries - 1 ))
    done

    die "Could not find a random not-in-use rfc1918 subnet"
}

# subnet_in_use() - true if subnet already routed on host
function subnet_in_use() {
    local subnet_script=${PODMAN_TMPDIR-/var/tmp}/subnet-in-use
    rm -f $subnet_script

    # This would be a nightmare to do in bash. ipcalc, ipcalc-ng, sipcalc
    # would be nice but are unavailable some environments (cough RHEL).
    # Likewise python/perl netmask modules. So, use bare-bones perl.
    cat >$subnet_script <<"EOF"
#!/usr/bin/env perl

use strict;
use warnings;

# 3 octets, in binary: 172.16.x -> 1010 1100 0000 1000 xxxx xxxx ...
my $subnet_to_check = sprintf("%08b%08b%08b", @ARGV);

my $found = 0;

# Input is "ip route list", one or more lines like '10.0.0.0/8 via ...'
while (<STDIN>) {
    # Only interested in x.x.x.x/n lines
    if (m!^([\d.]+)/(\d+)!) {
        my ($ip, $bits) = ($1, $2);

        # Our caller has /24 granularity, so treat /30 on host as /24.
        $bits = 24 if $bits > 24;

        # Temporary: entire subnet as binary string. 4 octets, split,
        # then represented as a 32-bit binary string.
        my $net = sprintf("%08b%08b%08b%08b", split(/\./, $ip));

        # Now truncate those 32 bits down to the route's netmask size.
        # This is the actual subnet range in use on the host.
        my $net_truncated = sprintf("%.*s", $bits, $net);

        # Desired subnet is in use if it matches a host route prefix
#        print STDERR "--- $subnet_to_check in $net_truncated (@ARGV in $ip/$bits)\n";
        $found = 1 if $subnet_to_check =~ /^$net_truncated/;
    }
}

# Convert to shell exit status (0 = success)
exit !$found;
EOF

    chmod 755 $subnet_script

    # This runs 'ip route list', converts x.x.x.x/n to its binary prefix,
    # then checks if our desired subnet matches that prefix (i.e. is in
    # that range). Existing routes with size greater than 24 are
    # normalized to /24 because that's the granularity of our
    # random_rfc1918_subnet code.
    #
    # Contrived examples:
    #    127.0.0.0/1   -> 0
    #    128.0.0.0/1   -> 1
    #    10.0.0.0/8    -> 00001010
    #
    # I'm so sorry for the ugliness.
    ip route list | $subnet_script $*
}

# ipv4_get_route_default() - Print first default IPv4 route reported by netlink
# $1:	Optional output of 'ip -j -4 route show' from a different context
function ipv4_get_route_default() {
    local jq_gw='[.[] | select(.dst == "default").gateway] | .[0]'
    local jq_nh='[.[] | select(.dst == "default").nexthops[0].gateway] | .[0]'
    local out

    out="$(echo "${1:-$(ip -j -4 route show)}" | jq -rM "${jq_gw}")"
    if [ "${out}" = "null" ]; then
        out="$(echo "${1:-$(ip -j -4 route show)}" | jq -rM "${jq_nh}")"
    fi

    echo "${out}"
}

# ipv6_get_route_default() - Print first default IPv6 route reported by netlink
# $1:	Optional output of 'ip -j -6 route show' from a different context
function ipv6_get_route_default() {
    local jq_gw='[.[] | select(.dst == "default").gateway] | .[0]'
    local jq_nh='[.[] | select(.dst == "default").nexthops[0].gateway] | .[0]'
    local out

    out="$(echo "${1:-$(ip -j -6 route show)}" | jq -rM "${jq_gw}")"
    if [ "${out}" = "null" ]; then
        out="$(echo "${1:-$(ip -j -6 route show)}" | jq -rM "${jq_nh}")"
    fi

    echo "${out}"
}

# ether_get_mtu() - Get MTU of first Ethernet-like link
# $1:	Optional output of 'ip -j link show' from a different context
function ether_get_mtu() {
    local jq_expr='[.[] | select(.link_type == "ether").mtu] | .[0]'
    echo "${1:-$(ip -j link show)}" | jq -rM "${jq_expr}"
}

# ether_get_name() - Get name of first Ethernet-like interface
# $1:	Optional output of 'ip -j link show' from a different context
function ether_get_name() {
    local jq_expr='[.[] | select(.link_type == "ether").ifname] | .[0]'
    echo "${1:-$(ip -j link show)}" | jq -rM "${jq_expr}"
}


### Ports and Ranges ###########################################################

# reserve_port() - create a lock file reserving a port, or return false
function reserve_port() {
    local port=$1

    mkdir -p $PORT_LOCK_DIR
    local lockfile=$PORT_LOCK_DIR/$port
    local locktmp=$PORT_LOCK_DIR/.$port.$$
    echo $BATS_SUITE_TEST_NUMBER >$locktmp

    if ln $locktmp $lockfile; then
        rm -f $locktmp
        return
    fi
    # Port already reserved
    rm -f $locktmp
    false
}

# unreserve_port() - free a temporarily-reserved port
function unreserve_port() {
    local port=$1

    local lockfile=$PORT_LOCK_DIR/$port
    test -e $lockfile || die "Cannot unreserve non-reserved port $port"
    assert "$(< $lockfile)" = "$BATS_SUITE_TEST_NUMBER" \
           "Port $port is not reserved by this test"
    rm -f $lockfile
}


# random_free_port() - Get unbound port with pseudorandom number
# $1:	Optional, dash-separated interval, [5000, 5999] by default
# $2:	Optional binding address, any IPv4 address by default
# $3:	Optional protocol, tcp or udp
function random_free_port() {
    local range=${1:-5000-5999}
    local address=${2:-0.0.0.0}
    local protocol=${3:-tcp}

    local port
    for port in $(shuf -i ${range}); do
        # First make sure no other tests are using it
        if reserve_port $port; then
            if port_is_free $port $address $protocol; then
                echo $port
                return
            fi

            unreserve_port $port
        fi
    done

    die "Could not find open port in range $range"
}

# random_free_port_range() - Get range of unbound ports with pseudorandom start
# $1:	Size of range (i.e. number of ports)
# $2:	Optional binding address, any IPv4 address by default
# $3:	Optional protocol, tcp or udp
function random_free_port_range() {
    local size=${1?Usage: random_free_port_range SIZE [ADDRESS [tcp|udp]]}
    local address=${2:-0.0.0.0}
    local protocol=${3:-tcp}

    local maxtries=10
    while [[ $maxtries -gt 0 ]]; do
        local firstport=$(random_free_port)
        local lastport=
        for i in $(seq 1 $((size - 1))); do
            lastport=$((firstport + i))
            if ! reserve_port $lastport; then
                lastport=
                break
            fi
            if ! port_is_free $lastport $address $protocol; then
                echo "# port $lastport is in use; trying another." >&3
                unreserve_port $lastport
                lastport=
                break
            fi
        done
        if [[ -n "$lastport" ]]; then
            echo "$firstport-$lastport"
            return
        fi

        unreserve_port $firstport

        maxtries=$((maxtries - 1))
    done

    die "Could not find free port range with size $size"
}

# port_is_bound() - Check if TCP or UDP port is bound for a given address
# $1:	Port number
# $2:	Optional protocol, or optional IPv4 or IPv6 address, default: tcp
# $3:	Optional IPv4 or IPv6 address, or optional protocol, default: any
function port_is_bound() {
    local port=${1?Usage: port_is_bound PORT [tcp|udp] [ADDRESS]}

    if   [ "${2}" = "tcp" ] || [ "${2}" = "udp" ]; then
        local address="${3}"
        local proto="${2}"
    elif [ "${3}" = "tcp" ] || [ "${3}" = "udp" ]; then
        local address="${2}"
        local proto="${3}"
    else
        local address="${2}"	# Might be empty
        local proto="tcp"
    fi

    # Use ss to check the local ports
    run -0 ss -${proto:0:1}nlH state all sport = $port
    # grep for exact address:port match or for "*:port" which means the port is bound to all addresses and hence bound for any address
    grep -q "$address:$port" <<<"$output" || grep -q "*:$port" <<<"$output"
}

# port_is_free() - Check if TCP or UDP port is free to bind for a given address
# $1:	Port number
# $2:	Optional protocol, or optional IPv4 or IPv6 address, default: tcp
# $3:	Optional IPv4 or IPv6 address, or optional protocol, default: any
function port_is_free() {
    ! port_is_bound ${@}
}

# wait_for_port() - Return once port is bound (available for use by clients)
# $1:	Host or address to check for possible binding
# $2:	Port number
# $3:	Optional timeout, 5 seconds if not given
function wait_for_port() {
    local host=$1
    local port=$2
    local _timeout=${3:-5}

    # Wait
    while [ $_timeout -gt 0 ]; do
        port_is_bound ${port} "${host}" && return
        sleep 1
        _timeout=$(( $_timeout - 1 ))
    done

    die "Timed out waiting for $host:$port"
}

# tcp_port_probe() - Check if a TCP port has an active listener
# $1:	Port number
# $2:	Optional address, 0.0.0.0 by default
function tcp_port_probe() {
    local address="${2:-0.0.0.0}"

    (exec echo -n >/dev/tcp/"$address/$1") >/dev/null 2>&1
}

### Pasta Helpers ##############################################################

function default_ifname() {
    local jq_expr='[.[] | select(.dst == "default").dev] | .[0]'
    local jq_expr_nh='[.[] | select(.dst == "default").nexthops[0].dev] | .[0]'
    local ip_ver="${1}"
    local out

    out="$(ip -j -"${ip_ver}" route show | jq -rM "${jq_expr}")"
    if [ "${out}" = "null" ]; then
        out="$(ip -j -"${ip_ver}" route show | jq -rM "${jq_expr_nh}")"
    fi

    echo "${out}"
}

function default_addr() {
    local ip_ver="${1}"
    local ifname="${2:-$(default_ifname "${ip_ver}")}"

    local expr='[.[0].addr_info[] | select(.deprecated != true)][0].local'
    ip -j -"${ip_ver}" addr show "${ifname}" | jq -rM "${expr}"
}
