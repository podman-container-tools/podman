# Setting up Podman service for systemd socket activation

## system-wide (podman service run as root)

 1. copy the `podman.service` and `podman.socket` files into `/etc/systemd/system`
 1. `systemctl daemon-reload`
 1. `systemctl enable podman.socket`
 1. `systemctl start podman.socket`
 1. `systemctl status podman.socket podman.service`

Assuming the status messages show no errors, the libpod service is ready to respond to the APIv2 on the unix domain socket `/run/podman/podman.sock`

### podman.service
You can refer to [this example](https://github.com/containers/podman/blob/main/contrib/systemd/system/podman.service.in) for a sample podman.service file.

Note: replace `@@PODMAN@@` with path to podman binary, such as `/usr/bin/podman`

### podman.socket
You can refer to [this example](https://github.com/containers/podman/blob/main/contrib/systemd/system/podman.socket) for a sample podman.socket file.

## user (podman service run as given user aka "rootless")

 1. `mkdir -p ~/.config/systemd/user`
 1. copy the `podman.service` and `podman.socket` files into `~/.config/systemd/user`
 1. `systemctl --user enable podman.socket`
 1. `systemctl --user start podman.socket`
 1. `systemctl --user status podman.socket podman.service`

Assuming the status messages show no errors, the libpod service is ready to respond to the APIv2 on the unix domain socket `/run/user/$(id -u)/podman/podman.sock`
