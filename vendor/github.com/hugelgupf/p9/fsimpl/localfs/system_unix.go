//go:build !windows

package localfs

import (
	"os"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/hugelgupf/p9/p9"
	"golang.org/x/sys/unix"
)

func umask(mask int) int {
	return syscall.Umask(mask)
}

// Inode numbers are only unique within a given device.  We need to compress
// device (u64) and inode number (u64) to a 64 bit qid path.
//
// We could assign a path for each new file we see and save the {dev,ino} -> qid
// lookup in a map.  Big downside: wasted memory.
//
// To avoid making a map entry for every file we ever see, realize that a lot of
// bits are 0 in the common case.  Let's pick bits that are likely set, and make
// a deterministic function to compute the qid.path for them.  For the cases
// that have any unlikely bits, stuff them in the map.  And differentiate
// between these two scenarios with the leading bit of qid.path.
//
// dev_t:
// 63                             32                              0
// |------------------------------|-------------------------------|
// \-------nothing really---------|\----MAJOR---AND----MINOR------|
//
// - The top 32 bits are usually 0.  The kernel's dev_t is 32 bits.
// - Major and minor aren't nicely laid out.  We'll just extract them from
// dev_t using unix helpers.
// - The MAJOR number space is used a lot.  I regularly see at least 9 of 12 bits
// used.  Let's not chop off any.
// - For MINOR, let's chop off some upper bits.
//
// If we want to be clever, we could look at all the devices currently in the
// system and dynamically generate how many bits of major/minor we use.  It just
// needs to be determined before we start serving.
//
// And for inode numbers, let's use as many lower bits as we can.  We can have
// up to 63 likely bits.  If only these bits are set, we can use the common case
// function.  If any of the unlikely bits are set, we'll have to use the map.
//
// Let's build the likely qid.path like so, with the partitions decided by
// the vars below, with bit63 = 0 for the Likely case.
//
// e.g. for the vars below, we'd have:
//
// 63 = 0
// |           51          39                                     0
// |-----------|-----------|--------------------------------------|
// |\---major--|\---minor--|\----------------inode----------------|

const devUpperBits = 32
const devUpperOffset = 32
const devMajorBits = 12
const devMinorBits = 20

// These can be tweaked by us at startup time.
var devMajorLikelyBits = devMajorBits - 0
var devMinorLikelyBits = devMinorBits - 8
var inodeLikelyBits = 39 // as much as we can get, up to 63 total

func init() {
	if devMajorLikelyBits+devMinorLikelyBits+
		inodeLikelyBits != 63 {
		panic("Bad dev-ino bit setup!")
	}
	nextQid.Store(uint64(1) << 63)
}

// Returns a u64 with the lower N bits set.
func nOnes(n int) uint64 {
	return (uint64(1) << n) - 1
}

// Returns the qid.path encoding for dev,ino and OK if it is likely.  To be
// "likely", dev and ino must only have the likely bits set.
func encodeLikely(dev, ino uint64) (uint64, bool) {
	inoLikely := nOnes(inodeLikelyBits)
	if (ino & ^inoLikely) != 0 {
		return 0, false
	}
	upperUnlikely := nOnes(devUpperBits) << devUpperOffset
	if (dev & upperUnlikely) != 0 {
		return 0, false
	}
	// In lieu of playing with bits, it suffices to make sure major and
	// minor fit within the bits that we'll encode in the "likely" case.
	major := uint64(unix.Major(dev))
	if major > nOnes(devMajorLikelyBits) {
		return 0, false
	}
	minor := uint64(unix.Minor(dev))
	if minor > nOnes(devMinorLikelyBits) {
		return 0, false
	}

	q := ino & inoLikely
	q |= minor << (inodeLikelyBits)
	q |= major << (inodeLikelyBits + devMinorLikelyBits)

	return q, true
}

type devino struct {
	dev uint64
	ino uint64
}

var qids sync.Map
var nextQid atomic.Uint64 // set to 1 << 63 in init()

func localToQid(_ string, fi os.FileInfo) (uint64, error) {
	stat := fi.Sys().(*syscall.Stat_t)
	if q, ok := encodeLikely(uint64(stat.Dev), stat.Ino); ok {
		return q, nil
	}
	di := &devino{uint64(stat.Dev), stat.Ino}
	if q, ok := qids.Load(di); ok {
		return q.(uint64), nil
	}
	// Could race and have two nextQids, but only one will win.  The other
	// will be ignored.  This is fine.
	q, _ := qids.LoadOrStore(di, nextQid.Add(1))
	return q.(uint64), nil
}

// lock implements p9.File.Lock.
func (l *Local) lock(pid int, locktype p9.LockType, flags p9.LockFlags, start, length uint64, client string) (p9.LockStatus, error) {
	switch locktype {
	case p9.ReadLock, p9.WriteLock:
		if err := unix.Flock(int(l.file.Fd()), unix.LOCK_EX); err != nil {
			return p9.LockStatusError, nil
		}

	case p9.Unlock:
		if err := unix.Flock(int(l.file.Fd()), unix.LOCK_EX); err != nil {
			return p9.LockStatusError, nil
		}

	default:
		return p9.LockStatusOK, unix.ENOSYS
	}

	return p9.LockStatusOK, nil
}
