// internal/component/binary/canonical.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Canonical opcodes
const (
	CanonOpLift         byte = 0x00
	CanonOpLower        byte = 0x01
	CanonOpResourceNew  byte = 0x02
	CanonOpResourceDrop byte = 0x03
	CanonOpResourceRep  byte = 0x04
)

// Canonical option opcodes
const (
	CanonOptStringUTF8        byte = 0x00
	CanonOptStringUTF16       byte = 0x01
	CanonOptStringLatin1UTF16 byte = 0x02
	CanonOptMemory            byte = 0x03
	CanonOptRealloc           byte = 0x04
	CanonOptPostReturn        byte = 0x05
)

// decodeCanonical reads a single canonical definition.
func decodeCanonical(r *bytes.Reader) (component.CanonicalDef, error) {
	def := component.CanonicalDef{}

	opcode, err := r.ReadByte()
	if err != nil {
		return def, err
	}

	switch opcode {
	case CanonOpLift:
		def.Kind = component.CanonKindLift

		// Read core sort (always 0x00 for funcs)
		sort, err := r.ReadByte()
		if err != nil {
			return def, fmt.Errorf("read core sort: %w", err)
		}
		if sort != 0x00 {
			return def, fmt.Errorf("expected core sort 0x00, got 0x%02x", sort)
		}

		// Read core function index
		def.CoreFuncIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read core funcidx: %w", err)
		}

		// Read options
		if err := decodeCanonicalOptions(r, &def.Options); err != nil {
			return def, fmt.Errorf("read options: %w", err)
		}

		// Read component function type index
		def.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read typeidx: %w", err)
		}

	case CanonOpLower:
		def.Kind = component.CanonKindLower

		// Read always-zero byte
		zero, err := r.ReadByte()
		if err != nil {
			return def, fmt.Errorf("read lower reserved byte: %w", err)
		}
		if zero != 0x00 {
			return def, fmt.Errorf("expected 0x00 after lower, got 0x%02x", zero)
		}

		// Read component function index
		def.FuncIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read funcidx: %w", err)
		}

		// Read options
		if err := decodeCanonicalOptions(r, &def.Options); err != nil {
			return def, fmt.Errorf("read options: %w", err)
		}

	case CanonOpResourceNew:
		def.Kind = component.CanonKindResourceNew
		def.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read typeidx: %w", err)
		}

	case CanonOpResourceDrop:
		def.Kind = component.CanonKindResourceDrop
		def.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read typeidx: %w", err)
		}

	case CanonOpResourceRep:
		def.Kind = component.CanonKindResourceRep
		def.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read typeidx: %w", err)
		}

	default:
		return def, fmt.Errorf("unknown canonical opcode: 0x%02x", opcode)
	}

	return def, nil
}

// decodeCanonicalOptions reads canonical options vector.
func decodeCanonicalOptions(r *bytes.Reader, opts *component.CanonicalOptions) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return err
	}

	for i := uint32(0); i < count; i++ {
		optCode, err := r.ReadByte()
		if err != nil {
			return err
		}

		switch optCode {
		case CanonOptStringUTF8:
			opts.StringEncoding = component.StringEncodingUTF8
		case CanonOptStringUTF16:
			opts.StringEncoding = component.StringEncodingUTF16
		case CanonOptStringLatin1UTF16:
			opts.StringEncoding = component.StringEncodingLatin1UTF16
		case CanonOptMemory:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return err
			}
			opts.MemoryIdx = &idx
		case CanonOptRealloc:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return err
			}
			opts.ReallocIdx = &idx
		case CanonOptPostReturn:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return err
			}
			opts.PostReturnIdx = &idx
		default:
			return fmt.Errorf("unknown canonical option: 0x%02x", optCode)
		}
	}

	return nil
}
