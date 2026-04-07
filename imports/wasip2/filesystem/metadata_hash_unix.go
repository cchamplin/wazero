//go:build linux || darwin

package filesystem

import (
	"encoding/binary"
	"hash/fnv"
	"os"
	"syscall"
)

// computeMetadataHash hashes file metadata using wasmtime's algorithm:
// hash(dev, ino) -> lower, lower ^ pi_constant -> upper
func computeMetadataHash(info os.FileInfo) (uint64, uint64) {
	h := fnv.New64a()
	sysStat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return computeMetadataHashFallback(info)
	}
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(sysStat.Dev))
	h.Write(buf[:])
	binary.LittleEndian.PutUint64(buf[:], sysStat.Ino)
	h.Write(buf[:])
	lower := h.Sum64()
	upper := lower ^ 4614256656552045848 // wasmtime's pi constant
	return lower, upper
}
