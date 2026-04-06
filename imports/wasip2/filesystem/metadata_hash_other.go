//go:build !linux && !darwin

package filesystem

import "os"

// computeMetadataHash fallback for non-Unix platforms: hashes name + size.
func computeMetadataHash(info os.FileInfo) (uint64, uint64) {
	return computeMetadataHashFallback(info)
}
