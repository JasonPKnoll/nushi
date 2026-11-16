//go:build unix

package regularfile

import (
	"io/fs"
	"syscall"
)

// Owner is the hard links of the file fi is a stat of, and its owner's
// uid; ok is false for a stat without them.
func Owner(fi fs.FileInfo) (links uint64, uid uint32, ok bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return widen(st.Nlink), st.Uid, true
}

// widen widens a link count, whose type is the platform's (uint16 on
// darwin, uint32 on linux/arm64, uint64 on linux/amd64, where a plain
// conversion reads to unconvert as one too many).
func widen[T uint16 | uint32 | uint64](n T) uint64 { return uint64(n) }
