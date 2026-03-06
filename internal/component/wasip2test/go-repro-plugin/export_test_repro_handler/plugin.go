package export_test_repro_handler

import (
	"wit_component/test_repro_host_ops"
)

// Process calls the host import and returns its value.
func Process() uint32 {
	return test_repro_host_ops.GetValue()
}
