// Copyright 2023 The Capability Authors.
// Copyright 2013 Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package capability

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
)

const (
	linuxCapVer1 = 0x19980330 // No longer supported.
	linuxCapVer2 = 0x20071026 // No longer supported.
	linuxCapVer3 = 0x20080522
)

// capValid reports whether the running kernel supports capability c.
func capValid(c Cap) (bool, error) {
	err := prctl(syscall.PR_CAPBSET_READ, uintptr(c), 0)
	if errors.Is(err, syscall.EINVAL) {
		return false, nil
	}
	return err == nil, err
}

// lastCap returns the highest capability supported by the running kernel.
var lastCap = sync.OnceValues(func() (Cap, error) {
	last, err := findLastCap()
	if err != nil {
		return 0, fmt.Errorf("unable to get last capability: %w", err)
	}
	return last, nil
})

func findLastCap() (Cap, error) {
	// Based on the fact Go 1.24+ supports Linux >= 3.2, and
	//   - CAP_WAKE_ALARM (35) was added in 3.0;
	//   - CAP_CHECKPOINT_RESTORE (40) was added in 5.9, and it is
	//     the last added capability as of October 2026 (Linux 7.3);
	//
	// the last capability is at least minLastCap, and most probably
	// lastKnownCap (but newer kernels may have more).
	//
	// maxLastCap is the highest capability which can be represented
	// by the kernel capability ABI v3 (two 32-bit words per set).
	const (
		minLastCap   = CAP_WAKE_ALARM
		lastKnownCap = CAP_CHECKPOINT_RESTORE
		maxLastCap   = Cap(63)
	)

	// Start from lastKnownCap, and go up if it's valid, or down if it's
	// not. For most kernels, this takes two syscalls.
	c := lastKnownCap
	ok, err := capValid(c)
	if err != nil {
		return 0, err
	}
	if ok {
		// Go up while the next one is valid.
		for c < maxLastCap {
			ok, err := capValid(c + 1)
			if err != nil {
				return 0, err
			}
			if !ok {
				break
			}
			c++
		}
		return c, nil
	}
	// Go down until a valid one is found.
	for c > minLastCap {
		c--
		ok, err := capValid(c)
		if err != nil {
			return 0, err
		}
		if ok {
			return c, nil
		}
	}
	return 0, fmt.Errorf("PR_CAPBSET_READ returned EINVAL for %s", minLastCap)
}

func capUpperMask() uint32 {
	last, err := lastCap()
	if err != nil || last < 32 {
		return 0
	}
	return (uint32(1) << (uint(last) - 31)) - 1
}

func mkStringCap(c Capabilities, which CapType) (ret string) {
	last, err := lastCap()
	if err != nil {
		return ""
	}
	for i, first := Cap(0), true; i <= last; i++ {
		if !c.Get(which, i) {
			continue
		}
		if first {
			first = false
		} else {
			ret += ", "
		}
		ret += i.String()
	}
	return
}

func mkString(c Capabilities, max CapType) (ret string) {
	ret = "{"
	for i := CapType(1); i <= max; i <<= 1 {
		ret += " " + i.String() + "=\""
		if c.Empty(i) {
			ret += "empty"
		} else if c.Full(i) {
			ret += "full"
		} else {
			ret += c.StringCap(i)
		}
		ret += "\""
	}
	ret += " }"
	return
}

var capVersion = sync.OnceValues(func() (uint32, error) {
	var hdr capHeader
	err := capget(&hdr, nil)
	return hdr.version, err
})

func newPid(pid int) (c Capabilities, retErr error) {
	ver, err := capVersion()
	if err != nil {
		retErr = fmt.Errorf("unable to get capability version from the kernel: %w", err)
		return
	}
	switch ver {
	case linuxCapVer1, linuxCapVer2:
		retErr = errors.New("old/unsupported capability version (kernel older than 2.6.26?)")
	default:
		// Either linuxCapVer3, or an unknown/future version (such as v4).
		// In the latter case, we fall back to v3 as the latest version known
		// to this package, as kernel should be backward-compatible to v3.
		p := new(capsV3)
		p.hdr.version = linuxCapVer3
		p.hdr.pid = int32(pid)
		c = p
	}
	return
}

func ignoreEINVAL(err error) error {
	if errors.Is(err, syscall.EINVAL) {
		err = nil
	}
	return err
}

type capsV3 struct {
	hdr     capHeader
	data    [2]capData
	bounds  [2]uint32
	ambient [2]uint32
}

func (c *capsV3) Get(which CapType, what Cap) bool {
	var i uint
	if what > 31 {
		i = uint(what) >> 5
		what %= 32
	}

	switch which {
	case EFFECTIVE:
		return (1<<uint(what))&c.data[i].effective != 0
	case PERMITTED:
		return (1<<uint(what))&c.data[i].permitted != 0
	case INHERITABLE:
		return (1<<uint(what))&c.data[i].inheritable != 0
	case BOUNDING:
		return (1<<uint(what))&c.bounds[i] != 0
	case AMBIENT:
		return (1<<uint(what))&c.ambient[i] != 0
	}

	return false
}

func (c *capsV3) getData(which CapType, dest []uint32) {
	switch which {
	case EFFECTIVE:
		dest[0] = c.data[0].effective
		dest[1] = c.data[1].effective
	case PERMITTED:
		dest[0] = c.data[0].permitted
		dest[1] = c.data[1].permitted
	case INHERITABLE:
		dest[0] = c.data[0].inheritable
		dest[1] = c.data[1].inheritable
	case BOUNDING:
		dest[0] = c.bounds[0]
		dest[1] = c.bounds[1]
	case AMBIENT:
		dest[0] = c.ambient[0]
		dest[1] = c.ambient[1]
	}
}

func (c *capsV3) Empty(which CapType) bool {
	var data [2]uint32
	c.getData(which, data[:])
	return data[0] == 0 && data[1] == 0
}

func (c *capsV3) Full(which CapType) bool {
	var data [2]uint32
	c.getData(which, data[:])
	if (data[0] & 0xffffffff) != 0xffffffff {
		return false
	}
	mask := capUpperMask()
	return (data[1] & mask) == mask
}

func (c *capsV3) Set(which CapType, caps ...Cap) {
	for _, what := range caps {
		var i uint
		if what > 31 {
			i = uint(what) >> 5
			what %= 32
		}

		if which&EFFECTIVE != 0 {
			c.data[i].effective |= 1 << uint(what)
		}
		if which&PERMITTED != 0 {
			c.data[i].permitted |= 1 << uint(what)
		}
		if which&INHERITABLE != 0 {
			c.data[i].inheritable |= 1 << uint(what)
		}
		if which&BOUNDING != 0 {
			c.bounds[i] |= 1 << uint(what)
		}
		if which&AMBIENT != 0 {
			c.ambient[i] |= 1 << uint(what)
		}
	}
}

func (c *capsV3) Unset(which CapType, caps ...Cap) {
	for _, what := range caps {
		var i uint
		if what > 31 {
			i = uint(what) >> 5
			what %= 32
		}

		if which&EFFECTIVE != 0 {
			c.data[i].effective &= ^(1 << uint(what))
		}
		if which&PERMITTED != 0 {
			c.data[i].permitted &= ^(1 << uint(what))
		}
		if which&INHERITABLE != 0 {
			c.data[i].inheritable &= ^(1 << uint(what))
		}
		if which&BOUNDING != 0 {
			c.bounds[i] &= ^(1 << uint(what))
		}
		if which&AMBIENT != 0 {
			c.ambient[i] &= ^(1 << uint(what))
		}
	}
}

func (c *capsV3) Fill(kind CapType) {
	if kind&CAPS == CAPS {
		c.data[0].effective = 0xffffffff
		c.data[0].permitted = 0xffffffff
		c.data[0].inheritable = 0
		c.data[1].effective = 0xffffffff
		c.data[1].permitted = 0xffffffff
		c.data[1].inheritable = 0
	}

	if kind&BOUNDS == BOUNDS {
		c.bounds[0] = 0xffffffff
		c.bounds[1] = 0xffffffff
	}
	if kind&AMBS == AMBS {
		c.ambient[0] = 0xffffffff
		c.ambient[1] = 0xffffffff
	}
}

func (c *capsV3) Clear(kind CapType) {
	if kind&CAPS == CAPS {
		c.data[0].effective = 0
		c.data[0].permitted = 0
		c.data[0].inheritable = 0
		c.data[1].effective = 0
		c.data[1].permitted = 0
		c.data[1].inheritable = 0
	}

	if kind&BOUNDS == BOUNDS {
		c.bounds[0] = 0
		c.bounds[1] = 0
	}
	if kind&AMBS == AMBS {
		c.ambient[0] = 0
		c.ambient[1] = 0
	}
}

func (c *capsV3) StringCap(which CapType) (ret string) {
	return mkStringCap(c, which)
}

func (c *capsV3) String() (ret string) {
	return mkString(c, BOUNDING)
}

func (c *capsV3) Load() (err error) {
	err = capget(&c.hdr, &c.data[0])
	if err != nil {
		return
	}

	path := "/proc/self/status"
	if c.hdr.pid != 0 {
		path = fmt.Sprintf("/proc/%d/status", c.hdr.pid)
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	b := bufio.NewReader(f)
	for {
		line, e := b.ReadString('\n')
		if e != nil {
			if e != io.EOF {
				err = e
			}
			break
		}
		if val, ok := strings.CutPrefix(line, "CapBnd:\t"); ok {
			_, err = fmt.Sscanf(val, "%08x%08x", &c.bounds[1], &c.bounds[0])
			if err != nil {
				break
			}
			continue
		}
		if val, ok := strings.CutPrefix(line, "CapAmb:\t"); ok {
			_, err = fmt.Sscanf(val, "%08x%08x", &c.ambient[1], &c.ambient[0])
			if err != nil {
				break
			}
			continue
		}
	}
	f.Close()

	return
}

func (c *capsV3) Apply(kind CapType) error {
	if c.hdr.pid != 0 {
		return errors.New("unable to modify capabilities of another process")
	}
	last, err := LastCap()
	if err != nil {
		return err
	}
	if kind&BOUNDS == BOUNDS {
		var data [2]capData
		err = capget(&c.hdr, &data[0])
		if err != nil {
			return err
		}
		if (1<<uint(CAP_SETPCAP))&data[0].effective != 0 {
			for i := Cap(0); i <= last; i++ {
				if c.Get(BOUNDING, i) {
					continue
				}
				// Ignore EINVAL since the capability may not be supported in this system.
				err = ignoreEINVAL(dropBound(i))
				if err != nil {
					return err
				}
			}
		}
	}

	if kind&CAPS == CAPS {
		err = capset(&c.hdr, &c.data[0])
		if err != nil {
			return err
		}
	}

	if kind&AMBS == AMBS {
		// Ignore EINVAL as not supported on kernels before 4.3
		err = ignoreEINVAL(resetAmbient())
		if err != nil {
			return err
		}
		for i := Cap(0); i <= last; i++ {
			if !c.Get(AMBIENT, i) {
				continue
			}
			// Ignore EINVAL as not supported on kernels before 4.3
			err = ignoreEINVAL(setAmbient(true, i))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getAmbient(c Cap) (bool, error) {
	res, err := prctlRetInt(pr_CAP_AMBIENT, pr_CAP_AMBIENT_IS_SET, uintptr(c))
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

func setAmbient(raise bool, caps ...Cap) error {
	op := pr_CAP_AMBIENT_RAISE
	if !raise {
		op = pr_CAP_AMBIENT_LOWER
	}
	for _, val := range caps {
		err := prctl(pr_CAP_AMBIENT, op, uintptr(val))
		if err != nil {
			return err
		}
	}
	return nil
}

func resetAmbient() error {
	return prctl(pr_CAP_AMBIENT, pr_CAP_AMBIENT_CLEAR_ALL, 0)
}

func getBound(c Cap) (bool, error) {
	res, err := prctlRetInt(syscall.PR_CAPBSET_READ, uintptr(c), 0)
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

func dropBound(caps ...Cap) error {
	for _, val := range caps {
		err := prctl(syscall.PR_CAPBSET_DROP, uintptr(val), 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func newFile(path string) (c Capabilities, err error) {
	c = &capsFile{path: path}
	return
}

type capsFile struct {
	path string
	data vfscapData
}

func (c *capsFile) Get(which CapType, what Cap) bool {
	var i uint
	if what > 31 {
		if c.data.version == 1 {
			return false
		}
		i = uint(what) >> 5
		what %= 32
	}

	switch which {
	case EFFECTIVE:
		return (1<<uint(what))&c.data.effective[i] != 0
	case PERMITTED:
		return (1<<uint(what))&c.data.data[i].permitted != 0
	case INHERITABLE:
		return (1<<uint(what))&c.data.data[i].inheritable != 0
	}

	return false
}

func (c *capsFile) getData(which CapType, dest []uint32) {
	switch which {
	case EFFECTIVE:
		dest[0] = c.data.effective[0]
		dest[1] = c.data.effective[1]
	case PERMITTED:
		dest[0] = c.data.data[0].permitted
		dest[1] = c.data.data[1].permitted
	case INHERITABLE:
		dest[0] = c.data.data[0].inheritable
		dest[1] = c.data.data[1].inheritable
	}
}

func (c *capsFile) Empty(which CapType) bool {
	var data [2]uint32
	c.getData(which, data[:])
	return data[0] == 0 && data[1] == 0
}

func (c *capsFile) Full(which CapType) bool {
	var data [2]uint32
	c.getData(which, data[:])
	if c.data.version == 0 {
		return (data[0] & 0x7fffffff) == 0x7fffffff
	}
	if (data[0] & 0xffffffff) != 0xffffffff {
		return false
	}
	mask := capUpperMask()
	return (data[1] & mask) == mask
}

func (c *capsFile) Set(which CapType, caps ...Cap) {
	for _, what := range caps {
		var i uint
		if what > 31 {
			if c.data.version == 1 {
				continue
			}
			i = uint(what) >> 5
			what %= 32
		}

		if which&EFFECTIVE != 0 {
			c.data.effective[i] |= 1 << uint(what)
		}
		if which&PERMITTED != 0 {
			c.data.data[i].permitted |= 1 << uint(what)
		}
		if which&INHERITABLE != 0 {
			c.data.data[i].inheritable |= 1 << uint(what)
		}
	}
}

func (c *capsFile) Unset(which CapType, caps ...Cap) {
	for _, what := range caps {
		var i uint
		if what > 31 {
			if c.data.version == 1 {
				continue
			}
			i = uint(what) >> 5
			what %= 32
		}

		if which&EFFECTIVE != 0 {
			c.data.effective[i] &= ^(1 << uint(what))
		}
		if which&PERMITTED != 0 {
			c.data.data[i].permitted &= ^(1 << uint(what))
		}
		if which&INHERITABLE != 0 {
			c.data.data[i].inheritable &= ^(1 << uint(what))
		}
	}
}

func (c *capsFile) Fill(kind CapType) {
	if kind&CAPS == CAPS {
		c.data.effective[0] = 0xffffffff
		c.data.data[0].permitted = 0xffffffff
		c.data.data[0].inheritable = 0
		if c.data.version == 2 {
			c.data.effective[1] = 0xffffffff
			c.data.data[1].permitted = 0xffffffff
			c.data.data[1].inheritable = 0
		}
	}
}

func (c *capsFile) Clear(kind CapType) {
	if kind&CAPS == CAPS {
		c.data.effective[0] = 0
		c.data.data[0].permitted = 0
		c.data.data[0].inheritable = 0
		if c.data.version == 2 {
			c.data.effective[1] = 0
			c.data.data[1].permitted = 0
			c.data.data[1].inheritable = 0
		}
	}
}

func (c *capsFile) StringCap(which CapType) (ret string) {
	return mkStringCap(c, which)
}

func (c *capsFile) String() (ret string) {
	return mkString(c, INHERITABLE)
}

func (c *capsFile) Load() (err error) {
	return getVfsCap(c.path, &c.data)
}

func (c *capsFile) Apply(kind CapType) (err error) {
	if kind&CAPS == CAPS {
		return setVfsCap(c.path, &c.data)
	}
	return
}
