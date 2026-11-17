package quadlet

import (
	"fmt"
	"os"
	"path"
	"sort"
)

// Overwritten at build time
var (
	_binDir = "/usr/bin"
)

func podmanBinary() string {
	podman := os.Getenv("PODMAN")
	if len(podman) > 0 {
		return podman
	}
	return path.Join(_binDir, "podman")
}

/* This is a helper for constructing podman commandlines */
type PodmanCmdline struct {
	Args []string
}

func (c *PodmanCmdline) add(args ...string) {
	c.Args = append(c.Args, args...)
}

func (c *PodmanCmdline) addf(format string, a ...any) {
	c.add(fmt.Sprintf(format, a...))
}

func (c *PodmanCmdline) addKeys(arg string, keys map[string]*string) {
	ks := make([]string, 0, len(keys))
	for k := range keys {
		ks = append(ks, k)
	}
	sort.Strings(ks)

	for _, k := range ks {
		if keys[k] != nil {
			c.add(arg, fmt.Sprintf("%s=%s", k, *keys[k]))
		} else {
			c.add(arg, k)
		}
	}
}

func (c *PodmanCmdline) addBool(arg string, val bool) {
	if val {
		c.add(arg)
	} else {
		c.addf("%s=false", arg)
	}
}

func NewPodmanCmdline(args ...string) *PodmanCmdline {
	c := &PodmanCmdline{
		Args: make([]string, 0),
	}

	c.add(podmanBinary())
	c.add(args...)
	return c
}
