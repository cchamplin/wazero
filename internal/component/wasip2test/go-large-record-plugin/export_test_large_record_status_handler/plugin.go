package export_test_large_record_status_handler

import (
	"wit_component/test_large_record_host_data"
	"wit_component/test_large_record_types"
)

func GetStatus() test_large_record_types.FullStatus {
	pos := test_large_record_host_data.GetPosition()
	return test_large_record_types.FullStatus{
		Coords: pos,
		Health: 100,
		Alive:  true,
		Score:  9999,
	}
}

func GetPosition() test_large_record_types.Coordinates {
	return test_large_record_host_data.GetPosition()
}
