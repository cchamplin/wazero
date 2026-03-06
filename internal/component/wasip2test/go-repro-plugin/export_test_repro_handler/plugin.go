package export_test_repro_handler

import (
	"wit_component/test_repro_host_ops"
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
