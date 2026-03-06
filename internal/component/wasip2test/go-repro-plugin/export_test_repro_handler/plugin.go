package export_test_repro_handler

import (
	"wit_component/test_repro_host_ops"
	"wit_component/test_repro_host_rng"
	"wit_component/test_repro_types"
)

// Process calls the host import and returns its value in a record.
func Process() test_repro_types.ProcessResult {
	v := test_repro_host_ops.GetValue()
	return test_repro_types.ProcessResult{
		Value: v,
		Ok:    true,
	}
}

// ProcessRandom calls the host import with a u64 argument.
func ProcessRandom(len uint64) uint64 {
	return test_repro_host_ops.GetRandomLen(len)
}

// ProcessRandomBytes calls host-rng.get-random-bytes (u32 -> list<u8>) to ensure
// the import is present in the component binary alongside wasi:random/random's
// get-random-bytes (u64 -> list<u8>).
func ProcessRandomBytes(count uint32) []uint8 {
	return test_repro_host_rng.GetRandomBytes(count)
}

// EchoString returns the input string as-is.
func EchoString(input string) string {
	return input
}
