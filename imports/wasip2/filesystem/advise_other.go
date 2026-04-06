//go:build !linux

package filesystem

import "os"

// fadvise is a no-op on non-Linux platforms.
// posix_fadvise is an optimization hint — the spec allows no-op.
func fadvise(f *os.File, offset, length uint64, advice string) error {
	return nil
}
