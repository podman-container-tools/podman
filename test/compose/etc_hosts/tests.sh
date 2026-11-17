# -*- bash -*-

ctr_name="etc_hosts-test-1"

podman exec "$ctr_name" sh -c 'grep "foobar" /etc/hosts'
like "$output" "10\.123\.0\." "$testname : no entries are copied from the host"

podman exec "$ctr_name" sh -c 'getent hosts foobar | awk "{print \$1}"'
like "$output" "10\.123\.0\." "$testname : hostname is resolved to IP address of the alias"
