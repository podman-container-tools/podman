# -*- bash -*-

test_port 5000 = "Podman rulez!"
podman container inspect -l --format '{{.Config.Labels}}'
like "$output" "io.podman:the_best" "$testname : Container label is set"
