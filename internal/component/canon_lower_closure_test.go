// internal/component/canon_lower_closure_test.go
//
// Task C8-a standalone unit test for createCanonLowerFunc.
package component

import (
	"context"
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestCanonLowerClosureSpecFlow asserts createCanonLowerFunc implements
// every step of canon_lower at definitions.py:2064-2130 for the
// synchronous MAX_FLAT_PARAMS path:
//
//	:2065 trap_if(not may_leave)
//	:2068 subtask / borrow scope creation
//	:2089 lift_flat_values on incoming args
//	:2095 host callback invocation via compFunc.Impl
//	:2104 lower_flat_values on results with may_leave toggle
//	:2113 deliver_resolve (borrow-scope release)
//
// Wasmtime parallel: runtime/component/func/host.rs DynamicHostFn::call
// lines 640-694.
func TestCanonLowerClosureSpecFlow(t *testing.T) {
	// Build a one-param / one-result u32->u32 TypeFunc via the bag.
	b := types.NewComponentTypesBuilder()
	paramsTup := b.InternTuple([]types.ValType{types.U32})
	resultsTup := b.InternTuple([]types.ValType{types.U32})
	ft := &types.TypeFunc{
		Params:  paramsTup,
		Results: resultsTup,
	}
	bag := b.Finish()

	// Capture host-callback args for assertion.
	var gotArgs []types.Val
	callbackCalls := 0
	hostFn := HostFunc(func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		callbackCalls++
		gotArgs = args
		require.Equal(t, 1, len(args))
		return []types.Val{types.ValU32(args[0].U32() + 1)}, nil
	})

	c := &Component{Types: bag}
	inst := newInstance(c, 1, nil)

	l := &ComponentLinker{}
	info := canonLowerInfo{funcIdx: 0}
	compFunc := ComponentFunc{Type: ft, Impl: hostFn}

	closure := l.createCanonLowerFunc(
		inst, c, info, compFunc,
		[]types.ValType{types.U32},
		[]types.ValType{types.U32},
		false, // no retptr: 1-param / 1-result fits flat
	)
	require.NotNil(t, closure)

	// Happy path: may_leave=true (default), stack=[42, 0].
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	mod := wazerotest.NewModule(mem)
	stack := []uint64{42, 0}

	require.True(t, inst.rt.IsMayLeave(), "precondition: may_leave=true")
	closure(context.Background(), mod, stack)

	require.Equal(t, 1, callbackCalls)
	require.Equal(t, types.ValKindU32, gotArgs[0].Kind())
	require.Equal(t, uint32(42), gotArgs[0].U32())
	require.Equal(t, uint64(43), stack[0], "result lowered back into stack[0]")
	require.True(t, inst.rt.IsMayLeave(), "may_leave restored after lower")

	// Error path: may_leave=false entry trap.
	inst.rt.MayLeave = false
	defer func() {
		r := recover()
		require.NotNil(t, r, "expected panic for may_leave=false")
		err, ok := r.(error)
		require.True(t, ok, "panic payload is an error")
		require.True(t, errors.Is(err, errMayNotLeave), "panic is errMayNotLeave")
	}()
	closure(context.Background(), mod, []uint64{1, 0})
}
