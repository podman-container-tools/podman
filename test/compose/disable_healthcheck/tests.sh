podman inspect --format='{{.Config.Healthcheck.Test}}' noHc
like $output "[NONE]" "$testname: healthcheck properly disabled"
