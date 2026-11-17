# -*- bash -*-

output=$(podman_compose up -d 2>&1)

# Horrible output check here but we really want to make sure that there are
# no unexpected warning/errors and the normal messages are send on stderr as
# well so we cannot check for an empty stderr.
expected=" Container uptwice_idempotent-app-1  Running"
is "$output" "$expected" "no container recreation in compose up (#24950)"
