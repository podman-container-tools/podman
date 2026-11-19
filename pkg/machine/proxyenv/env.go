package proxyenv

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/etchosts"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

// proxyReloadScriptTemplate reloads the systemd managers so that they pick up
// the changes to the proxy configuration files.
const proxyReloadScriptTemplate = `systemctl daemon-reload

# The user manager may not be running yet (e.g. user@.service still
# starting). That's fine: it will read the new user.conf.d when it
# starts, so a failure here must not stop the machine start.
systemctl --user -M %s@ daemon-reload || true
`

// proxySetupScriptTemplate writes the proxy environment variables in the
// machine configuration files.
const proxySetupScriptTemplate = `#!/bin/bash

SYSTEMD_SYSTEM_CONF=/etc/systemd/system.conf.d/default-env.conf
SYSTEMD_USER_CONF=/etc/systemd/user.conf.d/default-env.conf
ENVD_CONF=/etc/environment.d/default-env.conf
PROFILE_CONF=/etc/profile.d/default-env.sh

mkdir -p /etc/profile.d /etc/environment.d /etc/systemd/system.conf.d/ /etc/systemd/user.conf.d/
rm -f $SYSTEMD_SYSTEM_CONF $SYSTEMD_USER_CONF $ENVD_CONF $PROFILE_CONF

echo "[Manager]" >> $SYSTEMD_SYSTEM_CONF
echo "[Manager]" >> $SYSTEMD_USER_CONF
for proxy in %s; do
	systemd_proxy="${proxy//%%/%%%%}"
	printf "DefaultEnvironment=\"%%s\"\n" "$systemd_proxy"  >> $SYSTEMD_SYSTEM_CONF
	printf "DefaultEnvironment=\"%%s\"\n" "$systemd_proxy"  >> $SYSTEMD_USER_CONF
	printf "%%s\n" "$proxy"  >> $ENVD_CONF
	printf "export %%s\n" "$proxy" >> $PROFILE_CONF
done

` + proxyReloadScriptTemplate

// proxyCleanupScriptTemplate removes the proxy configuration files written by
// a previous machine start. The systemd managers are reloaded only if a file
// has been removed, to not slow down the start of machines without proxies.
const proxyCleanupScriptTemplate = `#!/bin/bash

removed=false
for conf in /etc/systemd/system.conf.d/default-env.conf \
	/etc/systemd/user.conf.d/default-env.conf \
	/etc/environment.d/default-env.conf \
	/etc/profile.d/default-env.sh; do
	if [ -e "$conf" ]; then
		rm -f "$conf"
		removed=true
	fi
done

if [ "$removed" = false ]; then
	exit 0
fi

` + proxyReloadScriptTemplate

func getProxyScript(isWSL bool, remoteUser string) io.Reader {
	var envs []string
	for _, key := range config.ProxyEnv {
		if value, ok := os.LookupEnv(key); ok {
			// WSL does not use host.containers.internal as valid name for the VM.
			if !isWSL {
				value = strings.ReplaceAll(value, "127.0.0.1", etchosts.HostContainersInternal)
				value = strings.ReplaceAll(value, "localhost", etchosts.HostContainersInternal)
			}
			// %q to quote the value correctly
			envs = append(envs, fmt.Sprintf("%q", key+"="+value))
		}
	}

	var script string
	if len(envs) == 0 {
		script = fmt.Sprintf(proxyCleanupScriptTemplate, remoteUser)
	} else {
		script = fmt.Sprintf(proxySetupScriptTemplate, strings.Join(envs, " "), remoteUser)
	}
	logrus.Tracef("Final environment variable setup script: %s", script)
	return strings.NewReader(script)
}

func ApplyProxies(mc *vmconfigs.MachineConfig) error {
	return machine.LocalhostSSHWithStdin("root", mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, []string{"/usr/bin/bash"},
		getProxyScript(mc.WSLHypervisor != nil, mc.SSH.RemoteUsername))
}
