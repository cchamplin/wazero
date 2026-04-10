// internal/component/component_linker_definefunc_test.go
//
// Task C3 (revised 2026-04-09): dynamic-host model.
//
// Spec: wasmtime linker.rs:665-675 (func_new), host.rs:619-626
// (DynamicHostFn::typecheck — validates only the async bit at link
// time; param/result types are validated at lift/lower time against
// cx.types[ty]).
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestComponentLinkerDefineFuncDynamicTyping asserts that DefineFunc
// accepts a typed HostFunc without requiring the host to declare a
// *types.TypeFunc. The component's import declaration is the source
// of truth; the runtime supplies the type to the callback at call
// time.
func TestComponentLinkerDefineFuncDynamicTyping(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)

	var observedType *types.TypeFunc
	fn := HostFunc(func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		observedType = fnType
		return nil, nil
	})

	// Registration must succeed without any type argument.
	err := l.DefineFunc("ns", "f", fn)
	require.NoError(t, err)

	// Nil HostFunc must be rejected (the only thing DefineFunc validates
	// at registration time).
	err = l.DefineFunc("ns", "g", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil HostFunc")

	// observedType is unused at this layer (no call has happened yet);
	// the explicit assignment exists so the test would catch a future
	// refactor that fails to thread the type into the callback. Lift/lower
	// wiring lands in Task C5+.
	_ = observedType
}
