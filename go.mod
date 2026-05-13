module go.podman.io/podman/v6

// Warning: if there is a "toolchain" directive anywhere in this file (and most of the
// time there shouldn't be), its version must be an exact match to the "go" directive.

go 1.25.6

require (
	github.com/Microsoft/go-winio v0.6.2
	github.com/blang/semver/v4 v4.0.0
	github.com/checkpoint-restore/checkpointctl v1.5.0
	github.com/checkpoint-restore/go-criu/v8 v8.2.0
	github.com/containers/gvisor-tap-vsock v0.8.9
	github.com/containers/libhvee v0.11.0
	github.com/containers/ocicrypt v1.3.0
	github.com/containers/psgo v1.10.0
	github.com/containers/winquit v1.1.0
	github.com/coreos/go-systemd/v22 v22.7.0
	github.com/crc-org/vfkit v0.6.3
	github.com/cyphar/filepath-securejoin v0.6.1
	github.com/digitalocean/go-qemu v0.0.0-20250212194115-ee9b0668d242
	github.com/docker/distribution v2.8.3+incompatible
	github.com/docker/go-connections v0.7.0
	github.com/docker/go-plugins-helpers v0.0.0-20240701071450-45e2431495c8
	github.com/docker/go-units v0.5.0
	github.com/godbus/dbus/v5 v5.2.2
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.6.0
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/schema v1.4.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hugelgupf/p9 v0.4.1
	github.com/jinzhu/copier v0.4.0
	github.com/json-iterator/go v1.1.12
	github.com/kevinburke/ssh_config v1.5.0
	github.com/klauspost/pgzip v1.2.6
	github.com/linuxkit/virtsock v0.0.0-20241009230534-cb6a20cc0422
	github.com/mattn/go-shellwords v1.0.13
	github.com/mattn/go-sqlite3 v1.14.45
	github.com/mdlayher/vsock v1.2.1
	github.com/moby/docker-image-spec v1.3.1
	github.com/moby/moby/api v1.54.2
	github.com/moby/moby/client v0.4.1
	github.com/moby/sys/capability v0.4.0
	github.com/moby/sys/user v0.4.0
	github.com/moby/term v0.5.2
	github.com/nxadm/tail v1.4.11
	github.com/onsi/ginkgo/v2 v2.29.0
	github.com/onsi/gomega v1.41.0
	github.com/opencontainers/cgroups v0.0.6
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/opencontainers/runtime-tools v0.9.1-0.20260316125833-8a4db579f5c8
	github.com/opencontainers/selinux v1.15.1
	github.com/openshift/imagebuilder v1.2.21
	github.com/rootless-containers/rootlesskit/v2 v2.3.6
	github.com/shirou/gopsutil/v4 v4.26.5
	github.com/sirupsen/logrus v1.9.4
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.11.1
	github.com/vbauerster/mpb/v8 v8.12.1
	github.com/vishvananda/netlink v1.3.1
	go.etcd.io/bbolt v1.4.3
	go.podman.io/buildah v1.44.0
	go.podman.io/common v0.68.0
	go.podman.io/image/v5 v5.40.0
	go.podman.io/storage v1.63.0
	golang.org/x/crypto v0.53.0
	golang.org/x/net v0.55.0
	golang.org/x/sync v0.21.0
	golang.org/x/sys v0.46.0
	golang.org/x/term v0.44.0
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.11
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v3 v3.0.1
	sigs.k8s.io/yaml v1.6.0
	tags.cncf.io/container-device-interface v1.1.0
)

require (
	cyphar.com/go-pathrs v0.2.4 // indirect
	dario.cat/mergo v1.0.2 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/aead/serpent v0.0.0-20160714141033-fba169763ea6 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.4 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.18.2 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/containernetworking/plugins v1.9.1 // indirect
	github.com/containers/common v0.64.2 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/luksy v0.0.0-20251208191447-ca096313c38f // indirect
	github.com/coreos/go-oidc/v3 v3.17.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/digitalocean/go-libvirt v0.0.0-20220804181439-8648fbde413e // indirect
	github.com/disiqueira/gotree/v3 v3.0.2 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker-credential-helpers v0.9.7 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/fsouza/go-dockerclient v1.13.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.21.1 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/google/pprof v0.0.0-20260402051712-545e8a4df936 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mistifyio/go-zfs/v4 v4.0.0 // indirect
	github.com/moby/buildkit v0.30.0 // indirect
	github.com/moby/go-archive v0.2.0 // indirect
	github.com/moby/patternmatcher v0.6.1 // indirect
	github.com/moby/sys/devices v0.1.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/opencontainers/runc v1.4.2 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.10 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/proglottis/gpgme v0.1.6 // indirect
	github.com/seccomp/libseccomp-golang v0.11.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.11.0 // indirect
	github.com/sigstore/fulcio v1.8.5 // indirect
	github.com/sigstore/protobuf-specs v0.5.0 // indirect
	github.com/sigstore/sigstore v1.10.6 // indirect
	github.com/skeema/knownhosts v1.3.2 // indirect
	github.com/smallstep/pkcs7 v0.1.1 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20230803200340-78284954bff6 // indirect
	github.com/sylabs/sif/v2 v2.24.0 // indirect
	github.com/tchap/go-patricia/v2 v2.3.3 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/vbatts/tar-split v0.12.3 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.68.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	golang.org/x/tools v0.45.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260406210006-6f92a3bedf2d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260406210006-6f92a3bedf2d // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	tags.cncf.io/container-device-interface/specs-go v1.1.0 // indirect
)
replace go.podman.io/common => github.com/zvenigorodsky/container-libs/common v0.0.0-20260513201046-29c417f5d0a4
