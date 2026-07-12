package proxyenv

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/pkg/config"
)

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
		name string
		args args
		want string
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
			want: `#!/bin/bash

SYSTEMD_SYSTEM_CONF=/etc/systemd/system.conf.d/default-env.conf
SYSTEMD_USER_CONF=/etc/systemd/user.conf.d/default-env.conf
ENVD_CONF=/etc/environment.d/default-env.conf
PROFILE_CONF=/etc/profile.d/default-env.sh

mkdir -p /etc/profile.d /etc/environment.d /etc/systemd/system.conf.d/ /etc/systemd/user.conf.d/
rm -f $SYSTEMD_SYSTEM_CONF $SYSTEMD_USER_CONF $ENVD_CONF $PROFILE_CONF

echo "[Manager]" >> $SYSTEMD_SYSTEM_CONF
echo "[Manager]" >> $SYSTEMD_USER_CONF
for proxy in "http_proxy=proxy1" "https_proxy=sproxy1" "no_proxy=no1,no2"; do
	systemd_proxy="${proxy//%/%%}"
	printf "DefaultEnvironment=\"%s\"\n" "$systemd_proxy"  >> $SYSTEMD_SYSTEM_CONF
	printf "DefaultEnvironment=\"%s\"\n" "$systemd_proxy"  >> $SYSTEMD_USER_CONF
	printf "%s\n" "$proxy"  >> $ENVD_CONF
	printf "export %s\n" "$proxy" >> $PROFILE_CONF
done

systemctl daemon-reload
`,
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
			want: `#!/bin/bash

SYSTEMD_SYSTEM_CONF=/etc/systemd/system.conf.d/default-env.conf
SYSTEMD_USER_CONF=/etc/systemd/user.conf.d/default-env.conf
ENVD_CONF=/etc/environment.d/default-env.conf
PROFILE_CONF=/etc/profile.d/default-env.sh

mkdir -p /etc/profile.d /etc/environment.d /etc/systemd/system.conf.d/ /etc/systemd/user.conf.d/
rm -f $SYSTEMD_SYSTEM_CONF $SYSTEMD_USER_CONF $ENVD_CONF $PROFILE_CONF

echo "[Manager]" >> $SYSTEMD_SYSTEM_CONF
echo "[Manager]" >> $SYSTEMD_USER_CONF
for proxy in "http_proxy=http://user%40example.com@proxy:3128"; do
	systemd_proxy="${proxy//%/%%}"
	printf "DefaultEnvironment=\"%s\"\n" "$systemd_proxy"  >> $SYSTEMD_SYSTEM_CONF
	printf "DefaultEnvironment=\"%s\"\n" "$systemd_proxy"  >> $SYSTEMD_USER_CONF
	printf "%s\n" "$proxy"  >> $ENVD_CONF
	printf "export %s\n" "$proxy" >> $PROFILE_CONF
done

systemctl daemon-reload
`,
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
			got := getProxyScript(tt.args.isWSL)
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(got)
			assert.NoError(t, err)
			str := buf.String()
			assert.Equal(t, tt.want, str)
		})
	}
}
