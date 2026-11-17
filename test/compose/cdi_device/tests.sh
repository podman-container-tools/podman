# -*- bash -*-

ctr_name="cdi_device-test-1"

podman exec "$ctr_name" sh -c 'stat -c "%t:%T" /dev-host/kmsg'

expected=$output

podman exec "$ctr_name" sh -c 'stat -c "%t:%T" /dev/kmsg1'

is "$output" "$expected" "$testname : device /dev/kmsg1 has the same rdev as /dev/kmsg on the host"
