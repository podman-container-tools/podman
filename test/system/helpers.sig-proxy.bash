# -*- bash -*-
#
# BATS helpers for sig-proxy functionality
#

# Command to run in each of the tests.
SLEEPLOOP='trap "echo BYE;exit 0" INT;echo READY;while :;do sleep 0.1;done'

# Main test code: wait for container to exist and be ready, send it a
# signal, wait for container to acknowledge and exit.
function _test_sigproxy() {
    local cname=$1
    local kidpid=$2

    # Wait for container to appear
    local timeout=10
    while :;do
          sleep 0.5
          run_podman '?' container exists $cname
          if [[ $status -eq 0 ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              run_podman ps -a
              die "Timed out waiting for container $cname to start"
          fi
    done

    # Now that container exists, wait for it to declare itself READY
    wait_for_ready $cname

    # Signal, and wait for container to exit
    kill -INT $kidpid
    timeout=20
    while :;do
          sleep 0.5
          run_podman logs $cname
          if [[ "$output" =~ BYE ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              run_podman ps -a
              die "Timed out waiting for BYE from container"
          fi
    done

    run_podman rm -f -t0 $cname
}
