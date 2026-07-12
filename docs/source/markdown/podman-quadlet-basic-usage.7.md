% podman-quadlet-basic-usage 7

# NAME

podman\-quadlet\-basic\-usage - Basic usage examples and step-by-step guide for Podman Quadlet

# DESCRIPTION

This guide introduces common usage patterns for Podman Quadlet. It provides step-by-step examples for defining
containers, exposing ports, creating volumes, and establishing dependencies using declarative `.container`, `.volume`,
and related unit files.

Quadlet simplifies container lifecycle management by translating these files into systemd services, making them
manageable with `systemctl`.

# EXAMPLE 1: RUNNING A SIMPLE CONTAINER

## Step 1: Create `hello.container`

```ini
[Unit]
Description=Hello Alpine Container

[Container]
Image=alpine
Exec=echo Hello from Quadlet!

[Install]
WantedBy=multi-user.target
```

## Step 2: Place the file

For rootless use:
```bash
mkdir -p ~/.config/containers/systemd
cp hello.container ~/.config/containers/systemd/
```

For rootful use:
```bash
sudo cp hello.container /etc/containers/systemd/
```

## Step 3: Reload and start the service

For rootless use:
```bash
systemctl --user daemon-reload
systemctl --user start hello.service
```

For rootful use:
```bash
sudo systemctl daemon-reload
sudo systemctl start hello.service
```

Note quadlet services cannot be enabled as they are a generated systemd unit,
see [podman-systemd.unit(5)](podman-systemd.unit.5.md#enabling-unit-files) for more information.


## Expected Output:

For rootless, check logs using:
```bash
journalctl --user -u hello.service
```

For rootful:
```bash
journalctl -u hello.service
```

You should see: `Hello from Quadlet!`

That means the container started, executed the echo command, and exited.

# EXAMPLE 2: CREATING A NAMED VOLUME

## Step 1: Create `mydata.volume`

```ini
[Volume]
VolumeName=mydata
Label=purpose=demo
```

## Step 2: Place and reload

For rootless use:
```bash
mkdir -p ~/.config/containers/systemd
cp mydata.volume ~/.config/containers/systemd/
systemctl --user daemon-reload
```

For rootful use:
```bash
sudo cp mydata.volume /etc/containers/systemd/
sudo systemctl daemon-reload
```

## Step 3: Create the volume

For rootless use:
```bash
systemctl --user start mydata-volume.service
```

For rootful use:
```bash
systemctl start mydata-volume.service
```

# EXAMPLE 3: CONTAINER USING A VOLUME

## Create `with-volume.container`

```ini
[Unit]
Description=Container with Mounted Volume

[Container]
Image=alpine
Exec=sh -c "ls /data && echo Hello > /data/hello.txt"
Volume=mydata.volume:/data

[Install]
WantedBy=multi-user.target
```

This container lists all files in the volume and creates `hello.txt`.

## Start the container and check the status

For rootless use:
```bash
cp with-volume.container ~/.config/containers/systemd/
systemctl --user daemon-reload
systemctl --user start with-volume.service
systemctl --user status with-volume.service
```

For rootful use:
```bash
sudo cp with-volume.container /etc/containers/systemd/
sudo systemctl daemon-reload
sudo systemctl start with-volume.service
sudo systemctl status with-volume.service
```

When started for the first time, the `hello.txt` will not appear in the
`systemctl status` output, because it has not been created yet. But when
started for the second time, the output will be:

```
hello.txt
```

This means the volume is used and is persistent.

# EXAMPLE 4: EXPOSING A CONTAINER PORT TO THE HOST

## Create `webserver.container`

```ini
[Unit]
Description=Nginx Webserver

[Container]
Image=nginx:alpine
PublishPort=8080:80

[Install]
WantedBy=multi-user.target
```

## Start the web server

For rootless use:
```bash
cp webserver.container ~/.config/containers/systemd/
systemctl --user daemon-reload
systemctl --user start webserver.service
```

For rootful use:
```bash
sudo cp webserver.container /etc/containers/systemd/
sudo systemctl daemon-reload
sudo systemctl start webserver.service
```

Visit `http://localhost:8080` in your browser.

# TIPS

To start a container on system boot, use:

```
[Install]
WantedBy=multi-user.target
```

If `systemctl` cannot find `foo.service`, it usually means there is a syntax
error in your Quadlet file. To find the details, use:

```bash
systemd-analyze --user --generators=true verify foo.service
```

# SEE ALSO

[podman-systemd.unit(5)](podman-systemd.unit.5.md),
[podman-container.unit(5)](podman-container.unit.5.md),
[podman-volume.unit(5)](podman-volume.unit.5.md),
[systemd.unit(5)](https://www.freedesktop.org/software/systemd/man/systemd.unit.html)

# AUTHORS

Podman Team <https://podman.io>
