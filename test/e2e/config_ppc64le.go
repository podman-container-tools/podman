//go:build linux || freebsd

package integration

var (
	STORAGE_FS          = "overlay"
	STORAGE_OPTIONS     = "--storage-driver overlay"
	ROOTLESS_STORAGE_FS = "overlay"
	CACHE_IMAGES        = []string{ALPINE, BB, FEDORA_MINIMAL, NGINX_IMAGE, REDIS_IMAGE, INFRA_IMAGE, CITEST_IMAGE}
	NGINX_IMAGE         = "quay.io/libpod/alpine_nginx-ppc64le:latest"
	BB_GLIBC            = "docker.io/ppc64le/busybox:glibc"
	CITEST_IMAGE        = "quay.io/libpod/testimage:20241011"
	REGISTRY_IMAGE      string
)
