//go:build !remote && (linux || freebsd)

package libpod

import (
	"net/http"
)

func PlayKube(w http.ResponseWriter, r *http.Request) {
	KubePlay(w, r)
}

func PlayKubeDown(w http.ResponseWriter, r *http.Request) {
	KubePlayDown(w, r)
}
