//go:build linux

package filesystem

import (
	"os"

	"golang.org/x/sys/unix"
)

// fadvise calls posix_fadvise on Linux.
func fadvise(f *os.File, offset, length uint64, advice string) error {
	var adviceFlag int
	switch advice {
	case "normal":
		adviceFlag = unix.FADV_NORMAL
	case "sequential":
		adviceFlag = unix.FADV_SEQUENTIAL
	case "random":
		adviceFlag = unix.FADV_RANDOM
	case "will-need":
		adviceFlag = unix.FADV_WILLNEED
	case "dont-need":
		adviceFlag = unix.FADV_DONTNEED
	case "no-reuse":
		adviceFlag = unix.FADV_NOREUSE
	default:
		return unix.EINVAL
	}
	return unix.Fadvise(int(f.Fd()), int64(offset), int64(length), adviceFlag)
}
