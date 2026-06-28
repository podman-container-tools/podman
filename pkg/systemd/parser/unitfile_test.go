package parser

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const memcachedService = `# It's not recommended to modify this file in-place, because it will be
# overwritten during upgrades.  If you want to customize, the best
# way is to use the "systemctl edit" command to create an override unit.
#
# For example, to pass additional options, create an override unit
# (as is done by systemctl edit) and enter the following:
#
#     [Service]
#     Environment=OPTIONS="-l 127.0.0.1,::1"


[Unit]
Description=memcached daemon
Before=httpd.service
After=network.target

[Service]
EnvironmentFile=/etc/sysconfig/memcached
ExecStart=/usr/bin/memcached -p ${PORT} -u ${USER} -m ${CACHESIZE} -c ${MAXCONN} $OPTIONS

# Set up a new file system namespace and mounts private /tmp and /var/tmp
# directories so this service cannot access the global directories and
# other processes cannot access this service's directories.
PrivateTmp=true

# Mounts the /usr, /boot, and /etc directories read-only for processes
# invoked by this unit.
ProtectSystem=full

# Ensures that the service process and all its children can never gain new
# privileges
NoNewPrivileges=true

# Sets up a new /dev namespace for the executed processes and only adds API
# pseudo devices such as /dev/null, /dev/zero or /dev/random (as well as
# the pseudo TTY subsystem) to it, but no physical devices such as /dev/sda.
PrivateDevices=true

# Required for dropping privileges and running as a different user
CapabilityBoundingSet=CAP_SETGID CAP_SETUID CAP_SYS_RESOURCE

# Restricts the set of socket address families accessible to the processes
# of this unit. Protects against vulnerabilities such as CVE-2016-8655
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX


# Some security features are not in the older versions of systemd used by
# e.g. RHEL7/CentOS 7. The below settings are automatically edited at package
# build time to uncomment them if the target platform supports them.

# Attempts to create memory mappings that are writable and executable at
# the same time, or to change existing memory mappings to become executable
# are prohibited.
##safer##MemoryDenyWriteExecute=true

# Explicit module loading will be denied. This allows to turn off module
# load and unload operations on modular kernels. It is recommended to turn
# this on for most services that do not need special file systems or extra
# kernel modules to work.
##safer##ProtectKernelModules=true

# Kernel variables accessible through /proc/sys, /sys, /proc/sysrq-trigger,
# /proc/latency_stats, /proc/acpi, /proc/timer_stats, /proc/fs and /proc/irq
# will be made read-only to all processes of the unit. Usually, tunable
# kernel variables should only be written at boot-time, with the sysctl.d(5)
# mechanism. Almost no services need to write to these at runtime; it is hence
# recommended to turn this on for most services.
##safer##ProtectKernelTunables=true

# The Linux Control Groups (cgroups(7)) hierarchies accessible through
# /sys/fs/cgroup will be made read-only to all processes of the unit.
# Except for container managers no services should require write access
# to the control groups hierarchies; it is hence recommended to turn this
# on for most services
##safer##ProtectControlGroups=true

# Any attempts to enable realtime scheduling in a process of the unit are
# refused.
##safer##RestrictRealtime=true

# Takes away the ability to create or manage any kind of namespace
##safer##RestrictNamespaces=true

[Install]
WantedBy=multi-user.target
`

const systemdloginService = `#  SPDX-License-Identifier: LGPL-2.1-or-later
#
#  This file is part of systemd.
#
#  systemd is free software; you can redistribute it and/or modify it
#  under the terms of the GNU Lesser General Public License as published by
#  the Free Software Foundation; either version 2.1 of the License, or
#  (at your option) any later version.

[Unit]
Description=User Login Management
Documentation=man:sd-login(3)
Documentation=man:systemd-logind.service(8)
Documentation=man:logind.conf(5)
Documentation=man:org.freedesktop.login1(5)

Wants=user.slice modprobe@drm.service
After=nss-user-lookup.target user.slice modprobe@drm.service

# Ask for the dbus socket.
Wants=dbus.socket
After=dbus.socket

[Service]
BusName=org.freedesktop.login1
CapabilityBoundingSet=CAP_SYS_ADMIN CAP_MAC_ADMIN CAP_AUDIT_CONTROL CAP_CHOWN CAP_DAC_READ_SEARCH CAP_DAC_OVERRIDE \
# comment inside a continuation line \
CAP_FOWNER \
CAP_SYS_TTY_CONFIG CAP_LINUX_IMMUTABLE
DeviceAllow=block-* r
DeviceAllow=char-/dev/console rw
DeviceAllow=char-drm rw
DeviceAllow=char-input rw
DeviceAllow=char-tty rw
DeviceAllow=char-vcs rw
ExecStart=/usr/lib/systemd/systemd-logind
FileDescriptorStoreMax=512
IPAddressDeny=any
LockPersonality=yes
MemoryDenyWriteExecute=yes
NoNewPrivileges=yes
PrivateTmp=yes
ProtectProc=invisible
ProtectClock=yes
ProtectControlGroups=yes
ProtectHome=yes
ProtectHostname=yes
ProtectKernelLogs=yes
ProtectKernelModules=yes
ProtectSystem=strict
ReadWritePaths=/etc /run
Restart=always
RestartSec=0
RestrictAddressFamilies=AF_UNIX AF_NETLINK
RestrictNamespaces=yes
RestrictRealtime=yes
RestrictSUIDSGID=yes
RuntimeDirectory=systemd/sessions systemd/seats systemd/users systemd/inhibit systemd/shutdown
RuntimeDirectoryPreserve=yes
StateDirectory=systemd/linger
SystemCallArchitectures=native
SystemCallErrorNumber=EPERM
SystemCallFilter=@system-service


# Increase the default a bit in order to allow many simultaneous logins since
# we keep one fd open per session.
LimitNOFILE=524288  \`

const systemdnetworkdService = `#  SPDX-License-Identifier: LGPL-2.1-or-later
#
#  This file is part of systemd.
#
#  systemd is free software; you can redistribute it and/or modify it
#  under the terms of the GNU Lesser General Public License as published by
#  the Free Software Foundation; either version 2.1 of the License, or
#  (at your option) any later version.

[Unit]
Description=Network Configuration
Documentation=man:systemd-networkd.service(8)
ConditionCapability=CAP_NET_ADMIN
DefaultDependencies=no
# systemd-udevd.service can be dropped once tuntap is moved to netlink
After=systemd-networkd.socket systemd-udevd.service network-pre.target systemd-sysusers.service systemd-sysctl.service
Before=network.target multi-user.target shutdown.target
Conflicts=shutdown.target
Wants=systemd-networkd.socket network.target

[Service]
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_BROADCAST CAP_NET_RAW
BusName=org.freedesktop.network1
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_BROADCAST CAP_NET_RAW
DeviceAllow=char-* rw
ExecStart=!!/usr/lib/systemd/systemd-networkd
ExecReload=networkctl reload
LockPersonality=yes
MemoryDenyWriteExecute=yes
NoNewPrivileges=yes
ProtectProc=invisible
ProtectClock=yes
ProtectControlGroups=yes
ProtectHome=yes
ProtectKernelLogs=yes
ProtectKernelModules=yes
ProtectSystem=strict
Restart=on-failure
RestartKillSignal=SIGUSR2
RestartSec=0
RestrictAddressFamilies=AF_UNIX AF_NETLINK AF_INET AF_INET6 AF_PACKET AF_ALG
RestrictNamespaces=yes
RestrictRealtime=yes
RestrictSUIDSGID=yes
RuntimeDirectory=systemd/netif
RuntimeDirectoryPreserve=yes
SystemCallArchitectures=native
SystemCallErrorNumber=EPERM
SystemCallFilter=@system-service
Type=notify
User=systemd-network


[Install]
WantedBy=multi-user.target
Also=systemd-networkd.socket
Alias=dbus-org.freedesktop.network1.service

# We want to enable systemd-networkd-wait-online.service whenever this service
# is enabled. systemd-networkd-wait-online.service has
# WantedBy=network-online.target, so enabling it only has an effect if
# network-online.target itself is enabled or pulled in by some other unit.
Also=systemd-networkd-wait-online.service
`

var samples = []string{memcachedService, systemdloginService, systemdnetworkdService}

const sampleDropinService string = "sample-unit.service"

var sampleDropinServicePaths = []string{
	"sample-unit.service.d",
	"sample-.service.d",
	"service.d",
}

const sampleDropinTemplate string = "sample-template-unit@.service"

var sampleDropinTemplatePaths = []string{
	"sample-template-unit@.service.d",
	"sample-template-@.service.d",
	"sample-template-.service.d",
	"sample-@.service.d",
	"sample-.service.d",
	"service.d",
}

const sampleDropinTemplateInstance string = "sample-template-unit@base-instance.service"

var sampleDropinTemplateInstancePaths = []string{
	"sample-template-unit@base-instance.service.d",
	"sample-template-unit@.service.d",
	"sample-template-@.service.d",
	"sample-template-.service.d",
	"sample-@.service.d",
	"sample-.service.d",
	"service.d",
}

var sampleDropinPaths = map[string][]string{
	sampleDropinService:          sampleDropinServicePaths,
	sampleDropinTemplate:         sampleDropinTemplatePaths,
	sampleDropinTemplateInstance: sampleDropinTemplateInstancePaths,
}

func filterComments(input string) string {
	lines := strings.Split(input, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		filtered = append(filtered, line)
	}
	// merge continuation lines
	joined := strings.ReplaceAll(strings.Join(filtered, "\n"), "\\\n", "")

	// and remove any trailing new line, backslash or space
	return strings.TrimRight(joined, "\n\\ ")
}

func TestRanges_Roundtrip(t *testing.T) {
	for i := range samples {
		sample := samples[i]

		f := NewUnitFile()
		if e := f.Parse(sample); e != nil {
			panic(e)
		}

		asStr, e := f.ToString()
		if e != nil {
			panic(e)
		}

		assert.Equal(t, filterComments(sample), filterComments(asStr))
	}
}

func TestUnitDropinPaths_Search(t *testing.T) {
	for filename, expectedPaths := range sampleDropinPaths {
		f := UnitFile{
			Filename: filename,
		}
		generatedPaths := f.GetUnitDropinPaths()

		assert.True(t, reflect.DeepEqual(expectedPaths, generatedPaths))
	}
}

func TestCommentsIgnored(t *testing.T) {
	unitWithComments := `[Container]
# comment
Name=my-container
; another comment
`
	f := NewUnitFile()
	if e := f.Parse(unitWithComments); e != nil {
		panic(e)
	}

	groups := f.ListGroups()
	assert.Len(t, groups, 1)
	assert.Equal(t, "Container", groups[0])

	comments := make([]string, 0, 2)
	for _, line := range f.groups[0].lines {
		if line.isComment {
			comments = append(comments, line.value)
		}
	}

	assert.Len(t, comments, 2)
	assert.Equal(t, "# comment", comments[0])
	assert.Equal(t, "; another comment", comments[1])
}

func TestLookupQuoteStripping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`hello`, "hello"},
		{`"hello`, `"hello`},
		{`/bin/sh -c "echo "hello""`, `/bin/sh -c "echo "hello""`},
		{`"/bin/sh -c "echo "hello"""`, `/bin/sh -c "echo "hello""`},
		{`""`, ""},
		{`hello"`, `hello"`},
		{`/bin/sh -c "echo 'hello world!'"`, `/bin/sh -c "echo 'hello world!'"`},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Lookup(%q)", tt.input), func(t *testing.T) {
			unitData := "[Container]\nKey=" + tt.input + "\n"
			f := NewUnitFile()
			err := f.Parse(unitData)
			assert.NoError(t, err)

			val, ok := f.Lookup("Container", "Key")
			assert.True(t, ok)
			assert.Equal(t, tt.expected, val)
		})
	}
}

func TestLookupIntParsesSignedNumbers(t *testing.T) {
	unit := NewUnitFile()
	unit.Add("Service", "Positive", "42")
	unit.Add("Service", "ExplicitPositive", "+42")
	unit.Add("Service", "Negative", "-42")
	unit.Add("Service", "Hex", "0x2a")
	unit.Add("Service", "Octal", "052")

	tests := []struct {
		key      string
		expected int64
	}{
		{"Positive", 42},
		{"ExplicitPositive", 42},
		{"Negative", -42},
		{"Hex", 42},
		{"Octal", 42},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.expected, unit.LookupInt("Service", tt.key, 0))
		})
	}
}

func FuzzParser(f *testing.F) {
	for _, sample := range samples {
		f.Add([]byte(sample))
	}

	f.Fuzz(func(_ *testing.T, orig []byte) {
		unitFile := NewUnitFile()
		unitFile.Path = "foo/bar"
		unitFile.Filename = "bar"
		_ = unitFile.Parse(string(orig))
	})
}
