# Podman-remote client for Windows with QEMU VM

***
**_NOTE:_** For running Podman on Windows, refer to the [Podman for Windows](podman-for-windows.md) guide, which uses the recommended approach of a Podman-managed Linux backend. For Mac, see the [Podman installation instructions](https://podman.io/getting-started/installation). This guide covers the advanced usage of Podman with a custom Linux VM.
***

## Introduction

This is an experimental setup using QEMU VM for running Podman for the already supported [Podman-remote](https://docs.podman.io/en/latest/markdown/podman-remote.1.html) client on Windows.
The officially supported and recommended way of running Podman on Windows is using [Podman machine](https://docs.podman.io/en/latest/markdown/podman-machine.1.html).

## Prerequisites

* Windows 10 Build 18362 or later (Build 19044/Version 21H2 or later recommended)
* SSH client feature installed on the machine
* Hyper-V acceleration should be operational on the machine
* Directory `C:\qemu-remote\` will be used for storing needed assets
* Port `57561` is free to use for ssh over a loopback interface

## Obtaining and installing

### QEMU

Download QEMU (7.2.0 minimal) from https://qemu.weilnetz.de/w64/

Then download the Fedora CoreOS (FCOS) image for QEMU from https://getfedora.org/coreos/download?tab=metal_virtualized&stream=testing&arch=x86_64

One will need `.xz` format extraction tool like xz itself or 7-zip. Use it to extract the `.qcow2` image to C:\qemu-remote\fedora-coreos-37.20221127.2.0-qemu.x86_64.qcow2

With xz the command line (when run from the same directory) will be
```
xz -d fedora-coreos-37.20221127.2.0-qemu.x86_64.qcow2.xz
```

### Podman

Download and install the latest release of Podman for Windows. Podman releases can be obtained from the official Podman GitHub release page: https://github.com/containers/podman/releases

#### Older Podman releases

When using older Podman releases (4.3.x and earlier), where `gvproxy.exe` is missing from the installation directory,
it could be obtained from the official releases https://github.com/containers/gvisor-tap-vsock/releases
One would need version `0.5.0` or a more recent release. Download `gvproxy-windows.exe` and copy it to
the Podman installation directory (or any other location, which is added to the PATH environment variable)
renaming the binary to `gvproxy.exe`.

### SSH

Generate ssh keys with an empty passphrase

ssh-keygen -t ed25519 -f C:\qemu-remote\remote

### Ignition for FCOS

Create ignition file C:\qemu-remote\remote.ign with the content of
```
{"ignition":{"config":{"replace":{"verification":{}}},"proxy":{},"security":{"tls":{}},"timeouts":{},"version":"3.2.0"},"passwd":{"users":[{"name":"core","sshAuthorizedKeys":["YOURSSHKEYHERE"],"uid":501},{"name":"root","sshAuthorizedKeys":["YOURSSHKEYHERE"]}]},"storage":{"directories":[{"group":{"name":"core"},"path":"/home/core/.config","user":{"name":"core"},"mode":493},{"group":{"name":"core"},"path":"/home/core/.config/containers","user":{"name":"core"},"mode":493},{"group":{"name":"core"},"path":"/home/core/.config/systemd","user":{"name":"core"},"mode":493},{"group":{"name":"core"},"path":"/home/core/.config/systemd/user","user":{"name":"core"},"mode":493},{"group":{"name":"core"},"path":"/home/core/.config/systemd/user/default.target.wants","user":{"name":"core"},"mode":493},{"group":{"name":"root"},"path":"/etc/containers/registries.conf.d","user":{"name":"root"},"mode":493},{"group":{"name":"root"},"path":"/etc/systemd/system.conf.d","user":{"name":"root"},"mode":493},{"group":{"name":"root"},"path":"/etc/environment.d","user":{"name":"root"},"mode":493}],"files":[{"group":{"name":"core"},"path":"/home/core/.config/systemd/user/linger-example.service","user":{"name":"core"},"contents":{"source":"data:,%5BUnit%5D%0ADescription=A%20systemd%20user%20unit%20demo%0AAfter=network-online.target%0AWants=network-online.target%20podman.socket%0A%5BService%5D%0AExecStart=%2Fusr%2Fbin%2Fsleep%20infinity%0A","verification":{}},"mode":484},{"group":{"name":"core"},"path":"/home/core/.config/containers/containers.conf","user":{"name":"core"},"contents":{"source":"data:,%5Bcontainers%5D%0Anetns=%22bridge%22%0A","verification":{}},"mode":484},{"group":{"name":"root"},"overwrite":true,"path":"/etc/subuid","user":{"name":"root"},"contents":{"source":"data:,core:100000:1000000","verification":{}},"mode":484},{"group":{"name":"root"},"overwrite":true,"path":"/etc/subgid","user":{"name":"root"},"contents":{"source":"data:,core:100000:1000000","verification":{}},"mode":484},{"group":{"name":"root"},"path":"/etc/systemd/system/user@.service.d/delegate.conf","user":{"name":"root"},"contents":{"source":"data:,%5BService%5D%0ADelegate=memory%20pids%20cpu%20io%0A","verification":{}},"mode":420},{"group":{"name":"core"},"path":"/var/lib/systemd/linger/core","user":{"name":"core"},"contents":{"verification":{}},"mode":420},{"group":{"name":"root"},"path":"/etc/containers/containers.conf","user":{"name":"root"},"contents":{"source":"data:,%5Bengine%5D%0Amachine_enabled=true%0A","verification":{}},"mode":420},{"group":{"name":"root"},"path":"/etc/containers/podman-machine","user":{"name":"root"},"contents":{"source":"data:,qemu%0A","verification":{}},"mode":420},{"group":{"name":"root"},"path":"/etc/containers/registries.conf.d/999-podman-machine.conf","user":{"name":"root"},"contents":{"source":"data:,unqualified-search-registries=%5B%22docker.io%22%5D%0A","verification":{}},"mode":420},{"group":{},"path":"/etc/tmpfiles.d/podman-docker.conf","user":{},"contents":{"source":"data:,L+%20%20%2Frun%2Fdocker.sock%20%20%20-%20%20%20%20-%20%20%20%20-%20%20%20%20%20-%20%20%20%2Frun%2Fpodman%2Fpodman.sock%0A","verification":{}},"mode":420},{"group":{"name":"root"},"path":"/etc/profile.d/docker-host.sh","user":{"name":"root"},"contents":{"source":"data:,export%20DOCKER_HOST=%22unix:%2F%2F$%28podman%20info%20-f%20%22%7B%7B.Host.RemoteSocket.Path%7D%7D%22%29%22%0A","verification":{}},"mode":420}],"links":[{"group":{"name":"core"},"path":"/home/core/.config/systemd/user/default.target.wants/linger-example.service","user":{"name":"core"},"hard":false,"target":"/home/core/.config/systemd/user/linger-example.service"},{"group":{"name":"root"},"overwrite":true,"path":"/usr/local/bin/docker","user":{"name":"root"},"hard":false,"target":"/usr/bin/podman"},{"group":{"name":"root"},"overwrite":false,"path":"/etc/localtime","user":{"name":"root"},"hard":false,"target":"\\usr\\share\\zoneinfo"}]},"systemd":{"units":[{"enabled":true,"name":"podman.socket"},{"contents":"[Unit]\nRequires=dev-virtio\\\\x2dports-vport1p1.device\nAfter=remove-moby.service sshd.socket sshd.service\nOnFailure=emergency.target\nOnFailureJobMode=isolate\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/bin/sh -c '/usr/bin/echo Ready \u003e/dev/vport1p1'\n[Install]\nRequiredBy=default.target\n","enabled":true,"name":"ready.service"},{"enabled":false,"mask":true,"name":"docker.service"},{"enabled":false,"mask":true,"name":"docker.socket"},{"contents":"[Unit]\nDescription=Remove moby-engine\n# Run once for the machine\nAfter=systemd-machine-id-commit.service\nBefore=zincati.service\nConditionPathExists=!/var/lib/%N.stamp\n\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/usr/bin/rpm-ostree override remove moby-engine\nExecStart=/usr/bin/rpm-ostree ex apply-live --allow-replacement\nExecStartPost=/bin/touch /var/lib/%N.stamp\n\n[Install]\nWantedBy=default.target\n","enabled":true,"name":"remove-moby.service"},{"contents":"[Unit]\nDescription=Environment setter from QEMU FW_CFG\n[Service]\nType=oneshot\nRemainAfterExit=yes\nEnvironment=FWCFGRAW=/sys/firmware/qemu_fw_cfg/by_name/opt/com.coreos/environment/raw\nEnvironment=SYSTEMD_CONF=/etc/systemd/system.conf.d/default-env.conf\nEnvironment=ENVD_CONF=/etc/environment.d/default-env.conf\nEnvironment=PROFILE_CONF=/etc/profile.d/default-env.sh\nExecStart=/usr/bin/bash -c '/usr/bin/test -f ${FWCFGRAW} \u0026\u0026\\\n\techo \"[Manager]\\n#Got from QEMU FW_CFG\\nDefaultEnvironment=$(/usr/bin/base64 -d ${FWCFGRAW} | sed -e \"s+|+ +g\")\\n\" \u003e ${SYSTEMD_CONF} ||\\\n\techo \"[Manager]\\n#Got nothing from QEMU FW_CFG\\n#DefaultEnvironment=\\n\" \u003e ${SYSTEMD_CONF}'\nExecStart=/usr/bin/bash -c '/usr/bin/test -f ${FWCFGRAW} \u0026\u0026 (\\\n\techo \"#Got from QEMU FW_CFG\"\u003e ${ENVD_CONF};\\\n\tIFS=\"|\";\\\n\tfor iprxy in $(/usr/bin/base64 -d ${FWCFGRAW}); do\\\n\t\techo \"$iprxy\" \u003e\u003e ${ENVD_CONF}; done ) || \\\n\techo \"#Got nothing from QEMU FW_CFG\"\u003e ${ENVD_CONF}'\nExecStart=/usr/bin/bash -c '/usr/bin/test -f ${FWCFGRAW} \u0026\u0026 (\\\n\techo \"#Got from QEMU FW_CFG\"\u003e ${PROFILE_CONF};\\\n\tIFS=\"|\";\\\n\tfor iprxy in $(/usr/bin/base64 -d ${FWCFGRAW}); do\\\n\t\techo \"export $iprxy\" \u003e\u003e ${PROFILE_CONF}; done ) || \\\n\techo \"#Got nothing from QEMU FW_CFG\"\u003e ${PROFILE_CONF}'\nExecStartPost=/usr/bin/systemctl daemon-reload\n[Install]\nWantedBy=sysinit.target\n","enabled":true,"name":"envset-fwcfg.service"}]}}
```

Replace "YOURSSHKEYHERE" with the actual pub keys you generated.

## Launching

### gvproxy

One needs to run gvproxy first to make it ready for the QEMU VM launched afterward. Run it  with the command below:
```
gvproxy.exe -listen-qemu unix://C:/qemu-remote/vlan_remote.sock -pid-file C:\qemu-remote\proxy.pid -ssh-port 57561 -forward-sock C:\qemu-remote\podman.sock -forward-dest /run/user/501/podman/podman.sock -forward-user core -forward-identity C:\qemu-remote\remote
```

### QEMU

Launch QEMU with the following command (the following configures it to use 4 CPUs and 8 GB RAM, but it could be adjusted for less):

```
qemu-system-x86_64w.exe -m 8192 -smp 4 -fw_cfg name=opt/com.coreos/config,file=C:\qemu-remote\remote.ign -netdev stream,id=vlan,server=off,addr.type=unix,addr.path=C:\qemu-remote\vlan_remote.sock -device virtio-net-pci,netdev=vlan,mac=5a:94:ef:e4:0c:ee -device virtio-serial -chardev socket,path=C:\qemu-remote\ready.sock,server=on,wait=off,id=apodman-machine-default_ready -device virtserialport,chardev=apodman-machine-default_ready,name=org.fedoraproject.port.0 -pidfile C:\qemu-remote\vm.pid -machine q35,accel=whpx:tcg -cpu max,vmx=off,monitor=off -drive if=virtio,file=C:\qemu-remote\fedora-coreos-37.20221127.2.0-qemu.x86_64.qcow2
```

### First time launch extras

Observe QEMU loading and wait for the message of SSH keys being provisioned to the machine. Next, before making the first ssh connection, one would need to add it to known hosts.
We are using `127.0.0.1` instead of `localhost` to force IPv4.

```
ssh-keyscan -p 57561 127.0.0.1 >> %USERPROFILE%\.ssh\known_hosts
```

### Add new connection to Podman

Create a connection named "qemuremote"

```
podman system connection add --identity C:\qemu-remote\remote -p 57561 qemuremote ssh://core@127.0.0.1
```

#### Optional

Make it default for simplicity of operation/testing

```
podman system connection default qemuremote
```

## Using Podman

Choose the active connection to be "qemuremote" (not needed if one made it default).

Run some basic network enabled workload:

```
podman run -d --rm -p 8080:80 nginx
```

Test it with

```
curl http -v http://localhost:8080
```

## Shutting down the machine

The built-in machinery of Podman machine will not work for a custom machine. One needs to gracefully shut it down by connecting via SSH:

```
ssh -i C:\qemu-remote\remote -p 57561 core@127.0.0.1
```

And then executing

```
sudo poweroff
```
