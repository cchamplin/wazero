package export_test_variantlog_processor

import (
	"wit_component/test_variantlog_host_logger"
	"wit_component/test_variantlog_types"
)

func ProcessEntry(entry test_variantlog_types.LogEntry) test_variantlog_types.LogResult {
	defaultSev := test_variantlog_host_logger.GetDefaultSeverity()
	return test_variantlog_types.LogResult{
		Count:        1,
		LastSeverity: defaultSev,
	}
}
