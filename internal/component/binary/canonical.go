// internal/component/binary/canonical.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Canonical opcodes
const (
	CanonOpLift         byte = 0x00
	CanonOpLower        byte = 0x01
	CanonOpResourceNew  byte = 0x02
	CanonOpResourceDrop byte = 0x03
	CanonOpResourceRep  byte = 0x04

	// Async/threading canonical opcodes (produce core func)
	CanonOpTaskCancel           byte = 0x05
	CanonOpSubtaskCancel        byte = 0x06
	CanonOpBackpressureSet      byte = 0x08
	CanonOpTaskReturn           byte = 0x09
	CanonOpContextGetI32        byte = 0x0a
	CanonOpContextSetI32        byte = 0x0b
	CanonOpThreadYield          byte = 0x0c
	CanonOpSubtaskDrop          byte = 0x0d
	CanonOpStreamNew            byte = 0x0e
	CanonOpStreamRead           byte = 0x0f
	CanonOpStreamWrite          byte = 0x10
	CanonOpStreamCancelRead     byte = 0x11
	CanonOpStreamCancelWrite    byte = 0x12
	CanonOpStreamDropReadable   byte = 0x13
	CanonOpStreamDropWritable   byte = 0x14
	CanonOpFutureNew            byte = 0x15
	CanonOpFutureRead           byte = 0x16
	CanonOpFutureWrite          byte = 0x17
	CanonOpFutureCancelRead     byte = 0x18
	CanonOpFutureCancelWrite    byte = 0x19
	CanonOpFutureDropReadable   byte = 0x1a
	CanonOpFutureDropWritable   byte = 0x1b
	CanonOpErrorContextNew      byte = 0x1c
	CanonOpErrorContextDebugMsg byte = 0x1d
	CanonOpErrorContextDrop     byte = 0x1e
	CanonOpWaitableSetNew       byte = 0x1f
	CanonOpWaitableSetWait      byte = 0x20
	CanonOpWaitableSetPoll      byte = 0x21
	CanonOpWaitableSetDrop      byte = 0x22
	CanonOpWaitableJoin         byte = 0x23
	CanonOpBackpressureInc      byte = 0x24
	CanonOpBackpressureDec      byte = 0x25
	CanonOpThreadIndex          byte = 0x26
	CanonOpThreadNewIndirect    byte = 0x27
	CanonOpThreadSwitchTo       byte = 0x28
	CanonOpThreadSuspend        byte = 0x29
	CanonOpThreadResumeLater    byte = 0x2a
	CanonOpThreadYieldTo        byte = 0x2b
	CanonOpThreadSpawnRef       byte = 0x40
	CanonOpThreadSpawnIndirect  byte = 0x41
	CanonOpThreadAvailParallel  byte = 0x42
)

// Canonical option opcodes
const (
	CanonOptStringUTF8        byte = 0x00
	CanonOptStringUTF16       byte = 0x01
	CanonOptStringLatin1UTF16 byte = 0x02
	CanonOptMemory            byte = 0x03
	CanonOptRealloc           byte = 0x04
	CanonOptPostReturn        byte = 0x05
	CanonOptAsync             byte = 0x06
	CanonOptCallback          byte = 0x07
	CanonOptCoreType          byte = 0x08
	CanonOptGC                byte = 0x09
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
		// Handle async/threading canonical opcodes.
		// All of these produce a (core func).
		if err := skipAsyncCanonicalBody(r, opcode); err != nil {
			return def, err
		}
		def.Kind = component.CanonKindAsync
		return def, nil
	}

	return def, nil
}

// skipAsyncCanonicalBody consumes the binary payload of an async/threading
// canonical opcode so the reader stays aligned. Every async/threading
// canonical builtin produces a (core func).
func skipAsyncCanonicalBody(r *bytes.Reader, opcode byte) error {
	switch opcode {
	// No operands
	case CanonOpBackpressureSet, // 0x08
		CanonOpTaskCancel,      // 0x05
		CanonOpSubtaskDrop,     // 0x0d
		CanonOpErrorContextDrop, // 0x1e
		CanonOpWaitableSetNew,  // 0x1f
		CanonOpWaitableSetDrop, // 0x22
		CanonOpWaitableJoin,    // 0x23
		CanonOpBackpressureInc, // 0x24
		CanonOpBackpressureDec, // 0x25
		CanonOpThreadIndex,     // 0x26
		CanonOpThreadResumeLater: // 0x2a
		return nil

	// resultlist opts
	case CanonOpTaskReturn: // 0x09
		if err := skipResultList(r); err != nil {
			return fmt.Errorf("skip task.return result list: %w", err)
		}
		if err := skipCanonOpts(r); err != nil {
			return fmt.Errorf("skip task.return opts: %w", err)
		}
		return nil

	// 0x7f u32
	case CanonOpContextGetI32, CanonOpContextSetI32: // 0x0a, 0x0b
		if _, err := r.ReadByte(); err != nil { // 0x7f
			return fmt.Errorf("skip context op valtype: %w", err)
		}
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip context op index: %w", err)
		}
		return nil

	// cancel?
	case CanonOpThreadYield,    // 0x0c
		CanonOpThreadSwitchTo, // 0x28
		CanonOpThreadSuspend,  // 0x29
		CanonOpThreadYieldTo:  // 0x2b
		if _, err := r.ReadByte(); err != nil { // cancel flag
			return fmt.Errorf("skip cancel flag: %w", err)
		}
		return nil

	// async?
	case CanonOpSubtaskCancel: // 0x06
		if _, err := r.ReadByte(); err != nil { // async flag
			return fmt.Errorf("skip async flag: %w", err)
		}
		return nil

	// typeidx
	case CanonOpStreamNew,          // 0x0e
		CanonOpStreamDropReadable, // 0x13
		CanonOpStreamDropWritable, // 0x14
		CanonOpFutureNew,          // 0x15
		CanonOpFutureDropReadable, // 0x1a
		CanonOpFutureDropWritable: // 0x1b
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip typeidx: %w", err)
		}
		return nil

	// typeidx opts
	case CanonOpStreamRead,  // 0x0f
		CanonOpStreamWrite, // 0x10
		CanonOpFutureRead,  // 0x16
		CanonOpFutureWrite: // 0x17
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip typeidx: %w", err)
		}
		if err := skipCanonOpts(r); err != nil {
			return fmt.Errorf("skip opts: %w", err)
		}
		return nil

	// typeidx async?
	case CanonOpStreamCancelRead,  // 0x11
		CanonOpStreamCancelWrite, // 0x12
		CanonOpFutureCancelRead,  // 0x18
		CanonOpFutureCancelWrite: // 0x19
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip typeidx: %w", err)
		}
		if _, err := r.ReadByte(); err != nil { // async flag
			return fmt.Errorf("skip async flag: %w", err)
		}
		return nil

	// opts
	case CanonOpErrorContextNew,      // 0x1c
		CanonOpErrorContextDebugMsg: // 0x1d
		if err := skipCanonOpts(r); err != nil {
			return fmt.Errorf("skip opts: %w", err)
		}
		return nil

	// cancel? memidx
	case CanonOpWaitableSetWait, // 0x20
		CanonOpWaitableSetPoll: // 0x21
		if _, err := r.ReadByte(); err != nil { // cancel flag
			return fmt.Errorf("skip cancel flag: %w", err)
		}
		if _, _, err := leb128.DecodeUint32(r); err != nil { // memidx
			return fmt.Errorf("skip memidx: %w", err)
		}
		return nil

	// typeidx tableidx
	case CanonOpThreadNewIndirect: // 0x27
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip typeidx: %w", err)
		}
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip tableidx: %w", err)
		}
		return nil

	// shared? typeidx
	case CanonOpThreadSpawnRef: // 0x40
		if _, err := r.ReadByte(); err != nil { // shared flag
			return fmt.Errorf("skip shared flag: %w", err)
		}
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip typeidx: %w", err)
		}
		return nil

	// shared? typeidx tableidx
	case CanonOpThreadSpawnIndirect: // 0x41
		if _, err := r.ReadByte(); err != nil { // shared flag
			return fmt.Errorf("skip shared flag: %w", err)
		}
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip typeidx: %w", err)
		}
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("skip tableidx: %w", err)
		}
		return nil

	// shared?
	case CanonOpThreadAvailParallel: // 0x42
		if _, err := r.ReadByte(); err != nil { // shared flag
			return fmt.Errorf("skip shared flag: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unknown canonical opcode: 0x%02x", opcode)
	}
}

// skipResultList skips a result type list in the binary stream.
// Format: vec(valtype)
func skipResultList(r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		if err := skipValType(r); err != nil {
			return err
		}
	}
	return nil
}

// skipCanonOpts skips an opts vector.
func skipCanonOpts(r *bytes.Reader) error {
	var opts component.CanonicalOptions
	return decodeCanonicalOptions(r, &opts)
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
			opts.StringEncoding = types.StringEncodingUTF8
		case CanonOptStringUTF16:
			opts.StringEncoding = types.StringEncodingUTF16
		case CanonOptStringLatin1UTF16:
			opts.StringEncoding = types.StringEncodingLatin1UTF16
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
		case CanonOptAsync:
			opts.Async = true
		case CanonOptCallback:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("read callback index: %w", err)
			}
			opts.CallbackIdx = &idx
		case CanonOptCoreType:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("read core-type index: %w", err)
			}
			opts.CoreTypeIdx = &idx
		case CanonOptGC:
			opts.GC = true
		default:
			return fmt.Errorf("unknown canonical option: 0x%02x", optCode)
		}
	}

	return nil
}
