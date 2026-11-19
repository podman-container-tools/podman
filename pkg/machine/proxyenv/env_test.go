package proxyenv

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/pkg/config"
)

// wantScriptTemplate is the expected script, PROXIES is replaced with the
// expected list of quoted proxy variables.
const wantScriptTemplate = `#!/bin/bash

SYSTEMD_SYSTEM_CONF=/etc/systemd/system.conf.d/default-env.conf
SYSTEMD_USER_CONF=/etc/systemd/user.conf.d/default-env.conf
ENVD_CONF=/etc/environment.d/default-env.conf
PROFILE_CONF=/etc/profile.d/default-env.sh

mkdir -p /etc/profile.d /etc/environment.d /etc/systemd/system.conf.d/ /etc/systemd/user.conf.d/
rm -f $SYSTEMD_SYSTEM_CONF $SYSTEMD_USER_CONF $ENVD_CONF $PROFILE_CONF

echo "[Manager]" >> $SYSTEMD_SYSTEM_CONF
echo "[Manager]" >> $SYSTEMD_USER_CONF
for proxy in PROXIES; do
	systemd_proxy="${proxy//%/%%}"
	printf "DefaultEnvironment=\"%s\"\n" "$systemd_proxy"  >> $SYSTEMD_SYSTEM_CONF
	printf "DefaultEnvironment=\"%s\"\n" "$systemd_proxy"  >> $SYSTEMD_USER_CONF
	printf "%s\n" "$proxy"  >> $ENVD_CONF
	printf "export %s\n" "$proxy" >> $PROFILE_CONF
done

systemctl daemon-reload

# The user manager may not be running yet (e.g. user@.service still
# starting). That's fine: it will read the new user.conf.d when it
# starts, so a failure here must not stop the machine start.
systemctl --user -M core@ daemon-reload || true
`

func Test_getProxyScript(t *testing.T) {
	type env struct {
		name  string
		value string
	}
	type args struct {
		isWSL bool
		envs  []env
	}
	tests := []struct {
		name        string
		args        args
		wantProxies string
		// Environment variable names are case-insensitive on Windows: setting
		// http_proxy also makes HTTP_PROXY visible, so both are forwarded.
		wantProxiesWindows string
	}{
		{
			name: "all vars set",
			args: args{
				isWSL: false,
				envs: []env{
					{
						name:  "http_proxy",
						value: "proxy1",
					},
					{
						name:  "https_proxy",
						value: "sproxy1",
					},
					{
						name:  "no_proxy",
						value: "no1,no2",
					},
				},
			},
			wantProxies:        `"http_proxy=proxy1" "https_proxy=sproxy1" "no_proxy=no1,no2"`,
			wantProxiesWindows: `"http_proxy=proxy1" "https_proxy=sproxy1" "no_proxy=no1,no2" "HTTP_PROXY=proxy1" "HTTPS_PROXY=sproxy1" "NO_PROXY=no1,no2"`,
		},
		{
			name: "percent sign in proxy value is escaped for systemd",
			args: args{
				isWSL: false,
				envs: []env{
					{
						name:  "http_proxy",
						value: "http://user%40example.com@proxy:3128",
					},
				},
			},
			wantProxies:        `"http_proxy=http://user%40example.com@proxy:3128"`,
			wantProxiesWindows: `"http_proxy=http://user%40example.com@proxy:3128" "HTTP_PROXY=http://user%40example.com@proxy:3128"`,
		},
	}

	// Unset all proxy env vars first
	for _, envVar := range config.ProxyEnv {
		t.Setenv(envVar, "") // needed for restoral during cleanup
		os.Unsetenv(envVar)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, e := range tt.args.envs {
				t.Setenv(e.name, e.value)
			}
			wantProxies := tt.wantProxies
			if runtime.GOOS == "windows" {
				wantProxies = tt.wantProxiesWindows
			}
			want := strings.Replace(wantScriptTemplate, "PROXIES", wantProxies, 1)
			got := getProxyScript(tt.args.isWSL, "core")
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(got)
			assert.NoError(t, err)
			assert.Equal(t, want, buf.String())
		})
	}
}
