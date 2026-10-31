//go:build windows

package wsl

const (
	ErrorSuccessRebootInitiated = 1641
	ErrorSuccessRebootRequired  = 3010
	currentMachineVersion       = 3
)

const appendPort = `grep -q Port\ %d /etc/ssh/sshd_config || echo Port %d >> /etc/ssh/sshd_config`

const changePort = `sed -E -i 's/^Port[[:space:]]+[0-9]+/Port %d/' /etc/ssh/sshd_config`

const configServices = `ln -fs /usr/lib/systemd/system/sshd.service /etc/systemd/system/multi-user.target.wants/sshd.service
ln -fs /usr/lib/systemd/system/podman.socket /etc/systemd/system/sockets.target.wants/podman.socket
ln -fs /usr/lib/systemd/user/podman.socket /etc/systemd/user/sockets.target.wants/podman.socket
rm -f /etc/systemd/system/getty.target.wants/console-getty.service
rm -f /etc/systemd/system/getty.target.wants/getty@tty1.service
rm -f /etc/systemd/system/multi-user.target.wants/systemd-resolved.service
rm -f /etc/systemd/system/sysinit.target.wants//systemd-resolved.service
rm -f /etc/systemd/system/dbus-org.freedesktop.resolve1.service
ln -fs /dev/null /etc/systemd/system/console-getty.service
ln -fs /dev/null /etc/systemd/system/systemd-oomd.socket
mkdir -p /etc/systemd/system/systemd-sysusers.service.d/
echo CREATE_MAIL_SPOOL=no >> /etc/default/useradd
adduser -m [USER] -G wheel
sed -ir 's/65536/1000000/' /etc/subuid
sed -ir 's/65536/1000000/' /etc/subgid
mkdir -p /home/[USER]/.config/systemd/[USER]/
chown [USER]:[USER] /home/[USER]/.config
`

const sudoers = `%wheel        ALL=(ALL)       NOPASSWD: ALL
`

const bootstrap = `#!/bin/bash
# Wait for systemd to be up. It's ok if the command
# "systemctl is-system-running" returns non-zero.
timeout 60 systemctl is-system-running --wait >/dev/null 2>&1 || true

# Trick to keep the WSL distribution alive or WSL idles it.
nohup sleep infinity </dev/null >/dev/null 2>&1 &

# Sleep to avoid a nohup/SIGHUP race
sleep 0.1
`

// Machines created by Podman v6.1 and earlier don't use the WSL systemd
// support: their /root/bootstrap script starts systemd in a separate PID and
// mount namespace. These machines are still supported and are identified by
// the script used to enter that namespace, that newer machines don't have.
const legacyEnterns = "/usr/local/bin/enterns"

// isLegacy exits with 0 if the machine uses the legacy bootstrap script. It
// exits with 1 otherwise.
const isLegacy = `[ -e ` + legacyEnterns + ` ]`

// setLegacy sets LEGACY to true for machines created by Podman v6.1 or
// earlier, and to false otherwise.
const setLegacy = `if ` + isLegacy + `; then LEGACY=true; else LEGACY=false; fi
`

// setMainPID sets LEGACY, and MAINPID to the PID of the process that keeps
// the machine running: systemd in its namespace for legacy machines, the
// keepalive started by the bootstrap script otherwise. MAINPID is empty if
// the process isn't running.
const setMainPID = setLegacy + `if $LEGACY; then
	MAINPID=$(ps -eo cmd,pid | grep -m 1 '^/lib/systemd/systemd' | awk '{print $NF}')
else
	MAINPID=$(ps -eo cmd,pid | grep -m 1 '^sleep infinity' | awk '{print $NF}')
fi
`

// isSystemRunning prints "running" if the machine is running and "offline"
// otherwise. Can be used for both legacy and native machines.
const isSystemRunning = setMainPID + `if $LEGACY; then
	if [ -n "$MAINPID" ] && [ "$MAINPID" != "1" ]; then echo running; else echo offline; fi
else
	systemctl is-system-running
fi
`

// termTimeoutSeconds is the maximum number of seconds to wait for a clean
// systemd shutdown.
const termTimeoutSeconds = "60"

// waitTermLegacy waits termTimeoutSeconds for the namespaced systemd to exit.
// It should be used on legacy machines only, machines that don't use the
// WSL-native systemd.
const waitTermLegacy = setMainPID + `
if [ -n "$MAINPID" ]; then timeout ` + termTimeoutSeconds + ` tail -f /dev/null --pid $MAINPID; fi
`

// termNative stops systemd, which stops the services and containers first.
const termNative = `systemctl poweroff`

// termLegacy stops systemd on legacy machines that used the systemd trick.
const termLegacy = legacyEnterns + ` systemctl exit 0`

// restartSSHD restarts sshd if it's running. Skipped on legacy machines, where
// systemd can't be reached from outside its namespace.
const restartSSHD = setLegacy + `
if ! $LEGACY; then systemctl try-restart sshd.service; fi
`

const wslConf = `[boot]
systemd=true

[user]
default=[USER]
`

// The scripts below only change the generateResolvConf line of
// /etc/wsl.conf, to keep any other configuration.

// disableUserNetResolvConf removes generateResolvConf from /etc/wsl.conf,
// so WSL generates /etc/resolv.conf again.
const disableUserNetResolvConf = `set -e
sed -i '/^generateResolvConf/d' /etc/wsl.conf
`

// enableUserNetResolvConf sets "generateResolvConf = false" in the [network]
// section of /etc/wsl.conf, adding the section if it doesn't exist.
const enableUserNetResolvConf = disableUserNetResolvConf + `
if grep -q '^\[network\]' /etc/wsl.conf; then
	sed -i '/^\[network\]/a generateResolvConf = false' /etc/wsl.conf
else
	printf '\n[network]\ngenerateResolvConf = false\n' >> /etc/wsl.conf
fi
`

const resolvConfUserNet = `
nameserver 192.168.127.1
`

// WSL kernel does not have sg and crypto_user modules
const overrideSysusers = `[Service]
LoadCredential=
`

const bindMountConfigDirSystemService = `
[Unit]
Description=Bind mount for config directory
Before=multi-user.target
After=local-fs.target

[Service]
RemainAfterExit=true
Type=oneshot
ExecStart=mount --bind %[1]s /etc/containers
ExecStop=umount /etc/containers
`

const bindMountSystemService = `
[Unit]
Description=Bind mount for system podman sockets
After=podman.socket

[Service]
RemainAfterExit=true
Type=oneshot
# Ensure user services can register sockets as well
ExecStartPre=mkdir -p -m 777 /mnt/wsl/podman-sockets
ExecStartPre=mkdir -p -m 777 /mnt/wsl/podman-sockets/%[1]s
ExecStartPre=touch /mnt/wsl/podman-sockets/%[1]s/podman-root.sock
ExecStart=mount --bind %%t/podman/podman.sock /mnt/wsl/podman-sockets/%[1]s/podman-root.sock
ExecStop=umount /mnt/wsl/podman-sockets/%[1]s/podman-root.sock
`

const bindMountUserService = `
[Unit]
Description=Bind mount for user podman sockets
After=podman.socket

[Service]
RemainAfterExit=true
Type=oneshot
# Consistency with system service (supports racing)
ExecStartPre=mkdir -p -m 777 /mnt/wsl/podman-sockets
ExecStartPre=mkdir -p -m 777 /mnt/wsl/podman-sockets/%[1]s
ExecStartPre=touch /mnt/wsl/podman-sockets/%[1]s/podman-user.sock
# Relies on /etc/fstab entry for user mounting
ExecStart=mount /mnt/wsl/podman-sockets/%[1]s/podman-user.sock
ExecStop=umount /mnt/wsl/podman-sockets/%[1]s/podman-user.sock
`

const bindMountFsTab = `/run/user/1000/podman/podman.sock /mnt/wsl/podman-sockets/%s/podman-user.sock none noauto,user,bind,defaults 0 0
`

const (
	defaultTargetWants     = "default.target.wants"
	userSystemdPath        = "/home/%[1]s/.config/systemd/user"
	sysSystemdPath         = "/etc/systemd/system"
	userSystemdWants       = userSystemdPath + "/" + defaultTargetWants
	sysSystemdWants        = sysSystemdPath + "/" + defaultTargetWants
	bindUnitFileName       = "podman-mnt-bindings.service"
	bindUserUnitPath       = userSystemdPath + "/" + bindUnitFileName
	bindUserUnitWant       = userSystemdWants + "/" + bindUnitFileName
	bindSysUnitPath        = sysSystemdPath + "/" + bindUnitFileName
	bindSysUnitWant        = sysSystemdWants + "/" + bindUnitFileName
	podmanSocketDropin     = "podman.socket.d"
	podmanSocketDropinPath = sysSystemdPath + "/" + podmanSocketDropin

	configBindSysUnitName           = "podman-mnt-config.service"
	configBindSysUnitPath           = sysSystemdPath + "/" + configBindSysUnitName
	configBindSysUnitWantsDirectory = sysSystemdPath + "/local-fs.target.wants"
	configBindSysUnitWant           = configBindSysUnitWantsDirectory + "/" + configBindSysUnitName
)

const configBindServices = "mkdir -p " + userSystemdWants + " " + sysSystemdWants + " " + podmanSocketDropinPath + " " + configBindSysUnitWantsDirectory + "\n" +
	"ln -fs " + bindUserUnitPath + " " + bindUserUnitWant + "\n" +
	"ln -fs " + bindSysUnitPath + " " + bindSysUnitWant + "\n" +
	"ln -fs " + configBindSysUnitPath + " " + configBindSysUnitWant + "\n"

const overrideSocketGroup = `
[Socket]
SocketMode=0660
SocketGroup=wheel
`

const wslInstallError = `Could not %s. See previous output for any potential failure details.
If you can not resolve the issue, and rerunning fails, try the "wsl --install" process
outlined in the following article:

http://docs.microsoft.com/en-us/windows/wsl/install

`

const wslOldVersion = `Automatic installation of WSL can not be performed on this version of Windows
Either update to Build 19041 (or later), or perform the manual installation steps
outlined in the following article:

http://docs.microsoft.com/en-us/windows/wsl/install\

`

const (
	gvProxy      = "gvproxy.exe"
	winSSHProxy  = "win-sshproxy.exe"
	pipePrefix   = "npipe:////./pipe/"
	globalPipe   = "docker_engine"
	userModeDist = "podman-net-usermode"
	rootfulSock  = "/run/podman/podman.sock"
	rootlessSock = "/run/user/1000/podman/podman.sock"
)
