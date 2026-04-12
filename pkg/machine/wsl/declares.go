//go:build windows

package wsl

const (
	ErrorSuccessRebootInitiated = 1641
	ErrorSuccessRebootRequired  = 3010
	currentMachineVersion       = 3
)

const containersConf = `[containers]

[engine]
cgroup_manager = "cgroupfs"
`

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
ps -ef | grep -v grep | grep -q systemd && exit 0
nohup unshare --kill-child --fork --pid --mount --mount-proc --propagation shared /lib/systemd/systemd >/dev/null 2>&1 &
sleep 0.1
`

const wslmotd = `
You will be automatically entered into a nested process namespace where
systemd is running. If you need to access the parent namespace, hit ctrl-d
or type exit. This also means to log out you need to exit twice.

`

const sysdpid = "SYSDPID=`ps -eo cmd,pid | grep -m 1 ^/lib/systemd/systemd | awk '{print $2}'`"

const profile = sysdpid + `
if [ ! -z "$SYSDPID" ] && [ "$SYSDPID" != "1" ]; then
    cat /etc/wslmotd
	/usr/local/bin/enterns
fi
`

const enterns = "#!/bin/bash\n" + sysdpid + `
if [ ! -z "$SYSDPID" ] && [ "$SYSDPID" != "1" ]; then
        NSENTER=("nsenter" "-m" "-p" "-t" "$SYSDPID" "--wd=$PWD")

        if [ "$UID" != "0" ]; then
                NSENTER=("sudo" "${NSENTER[@]}")
                if [ "$#" != "0" ]; then
                        NSENTER+=("sudo" "-u" "$USER")
                else
                        NSENTER+=("su" "-l" "$USER")
                fi
        fi
        "${NSENTER[@]}" "$@"
fi`

const waitTerm = sysdpid + `
if [ ! -z "$SYSDPID" ]; then
	timeout 60 tail -f /dev/null --pid $SYSDPID
fi
`

const wslConf = `[user]
default=[USER]
`

const wslConfUserNet = `
[network]
generateResolvConf = false
`

const resolvConfUserNet = `
nameserver 192.168.127.1
`

// createResolvScript writes /etc/resolv.conf with the user-mode networking
// nameserver as primary and the host's DNS servers (from the WSL-shared
// resolv.conf) as fallback. This allows DNS to keep working when
// gvproxy/gvforwarder cannot forward queries, e.g. after a VPN connects
// and changes the host resolver.
const createResolvScript = `set -e
rm -f /etc/resolv.conf
echo 'nameserver 192.168.127.1' > /etc/resolv.conf
if [ -f /mnt/wsl/resolv.conf ]; then
    grep '^nameserver' /mnt/wsl/resolv.conf | grep -v '192\.168\.127\.1' >> /etc/resolv.conf 2>/dev/null || true
fi
`

// WSL kernel does not have sg and crypto_user modules
const overrideSysusers = `[Service]
LoadCredential=
`

const bindMountConfigDirSystemService = `
[Unit]
Description=Bind mount for config directory
Before=podman.socket

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

	configBindSysUnitName = "podman-mnt-config.service"
	configBindSysUnitPath = sysSystemdPath + "/" + configBindSysUnitName
	configBindSysUnitWant = sysSystemdWants + "/" + configBindSysUnitName
)

const configBindServices = "mkdir -p " + userSystemdWants + " " + sysSystemdWants + " " + podmanSocketDropinPath + "\n" +
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
