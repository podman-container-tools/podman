# DOCKER_HOST initialization

if [ -z "${DOCKER_HOST-}" ]; then
    if [ "$(id -u)" -eq 0 ]; then
	export DOCKER_HOST=unix:///run/podman/podman.sock
    else
	if [ -n "${XDG_RUNTIME_DIR-}" ]; then
	    export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock
	fi
    fi
fi
