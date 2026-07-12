% podmansh 1

## NAME
podmansh - Execute login shell within the Podman `podmansh` container

## SYNOPSIS
**podmansh**

## DESCRIPTION

Execute a user shell within a container when the user logs into the system. The container that the users get added to can be defined via a Podman Quadlet file. This user only has access to volumes and capabilities configured into the Quadlet file.

Administrators can create a Quadlet in /etc/containers/systemd/users, which systemd will start for all users when they log in. The administrator can create a specific Quadlet with the container name `podmansh`, then enable users to use the login shell /usr/bin/podmansh.  These user login shells are automatically executed inside  the `podmansh` container via Podman.

Optionally, the administrator can place Quadlet files in the /etc/containers/systemd/users/${UID} directory for a user. Only this UID will execute these Quadlet services when that user logs in.

The user is confined to the container environment via all of the security mechanisms, including SELinux. The only information that will be available from the system comes from volumes leaked into the container.

Systemd will automatically create the container when the user session is started. Systemd will take down the container when all connections to the user session are removed. This means users can log in to the system multiple times, with each session connected to the same container.

Administrators can use volumes to expose specific host data from the host system to the user, without the user being exposed to other parts of the system.

Timeout for podmansh can be set using the `podmansh_timeout` option in containers.conf.

## Setup
Create user login session using useradd while running as root.

```
# useradd -s /usr/bin/podmansh lockedu
# grep lockedu /etc/passwd
lockedu:x:4008:4008::/home/lockedu:/usr/bin/podmansh
```

Create a Podman Quadlet file that looks something like one of the following.

Fully locked down container, no access to host OS.

```
# USERID=$(id -u lockedu)
# mkdir -p /etc/containers/systemd/users/${USERID}
# cat > /etc/containers/systemd/users/${USERID}/podmansh.container << _EOF
[Unit]
Description=The podmansh container
After=local-fs.target

[Container]
Image=registry.fedoraproject.org/fedora
ContainerName=podmansh
UserNS=keep-id
RunInit=yes
DropCapability=all
NoNewPrivileges=true

Exec=sleep infinity

[Install]
RequiredBy=default.target
_EOF
```

Alternatively, while running as root, create a Quadlet where the user is allowed to become root within the user namespace. They can also permanently read/write content from their home directory which is volume mounted from the actual host's users account, rather than being inside of the container.

```
# useradd -s /usr/bin/podmansh confinedu
# grep confinedu /etc/passwd
confinedu:x:4009:4009::/home/confinedu:/usr/bin/podmansh
# USERID=$(id -u confinedu)
# mkdir -p /etc/containers/systemd/users/${USERID}
# cat > /etc/containers/systemd/users/${USERID}/podmansh.container << _EOF
[Unit]
Description=The podmansh container
After=local-fs.target

[Container]
Image=registry.fedoraproject.org/fedora
ContainerName=podmansh
UserNS=keep-id
RunInit=yes

Volume=%h/data:%h:Z
Exec=sleep infinity

[Service]
ExecStartPre=/usr/bin/mkdir -p %h/data

[Install]
RequiredBy=default.target
_EOF
```

Another example, while running as root, create a Quadlet where the users inside this container are allowed to execute containers with SELinux separation and able to read and write content in the $HOME/data directory.

```
# useradd -s /usr/bin/podmansh fullu
# grep fullu /etc/passwd
fullu:x:4010:4010::/home/fullu:/usr/bin/podmansh
# USERID=$(id -u fullu)
# mkdir -p /etc/containers/systemd/users/${USERID}
# cat > /etc/containers/systemd/users/${USERID}/podmansh.container << _EOF
[Unit]
Description=The podmansh container
After=local-fs.target

[Container]
Image=registry.fedoraproject.org/fedora
ContainerName=podmansh
UserNS=keep-id
RunInit=yes
PodmanArgs=--security-opt=unmask=/sys/fs/selinux \
	--security-opt=label=nested \
	--security-opt=label=user:container_user_u \
	--security-opt=label=type:container_user_t \
	--security-opt=label=role:container_user_r \
	--security-opt=label=level:s0-s0:c0.c1023

Volume=%h/data:%h:Z
WorkingDir=%h
Volume=/sys/fs/selinux:/sys/fs/selinux
Exec=sleep infinity

[Service]
ExecStartPre=/usr/bin/mkdir -p %h/data

[Install]
RequiredBy=default.target
_EOF
```

## SEE ALSO
**[containers.conf(5)](containers.conf.5.md)**, **[podman(1)](podman.1.md)**, **[podman-exec(1)](podman-exec.1.md)**, **[podman-systemd.unit(5)](podman-systemd.unit.5.md)**

## HISTORY
May 2023, Originally compiled by Dan Walsh <dwalsh@redhat.com>
