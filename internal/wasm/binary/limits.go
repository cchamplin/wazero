package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeLimitsType returns the `limitsType` (min, max) decoded with the WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#limits%E2%91%A6
//
// Extended in threads proposal: https://webassembly.github.io/threads/core/binary/types.html#limits
func decodeLimitsType(r *bytes.Reader) (min uint32, max *uint32, shared bool, err error) {
	var flag byte
	if flag, err = r.ReadByte(); err != nil {
		err = fmt.Errorf("read leading byte: %v", err)
		return
	}

	switch flag {
	case 0x00, 0x02:
		min, _, err = leb128.DecodeUint32(r)
		if err != nil {
			err = fmt.Errorf("read min of limit: %v", err)
		}
	case 0x01, 0x03:
		min, _, err = leb128.DecodeUint32(r)
		if err != nil {
			err = fmt.Errorf("read min of limit: %v", err)
			return
		}
		var m uint32
		if m, _, err = leb128.DecodeUint32(r); err != nil {
			err = fmt.Errorf("read max of limit: %v", err)
		} else {
			max = &m
		}
	case 0x04: // memory64, min only (u64 LEB128, truncated to u32)
		var min64 int64
		min64, _, err = leb128.DecodeInt64(r)
		if err != nil {
			err = fmt.Errorf("read min of limit (i64): %v", err)
			return
		}
		min = uint32(min64)
	case 0x05: // memory64, min+max (u64 LEB128, truncated to u32)
		var min64 int64
		min64, _, err = leb128.DecodeInt64(r)
		if err != nil {
			err = fmt.Errorf("read min of limit (i64): %v", err)
			return
		}
		min = uint32(min64)
		var max64 int64
		if max64, _, err = leb128.DecodeInt64(r); err != nil {
			err = fmt.Errorf("read max of limit (i64): %v", err)
		} else {
			m := uint32(max64)
			max = &m
		}
	default:
		err = fmt.Errorf("%v for limits: %#x not in (0x00, 0x01, 0x02, 0x03)", ErrInvalidByte, flag)
	}

	shared = flag == 0x02 || flag == 0x03

	return
}
