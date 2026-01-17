// debug_test.go
package wazero

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDebugComponentStructure(t *testing.T) {
	ctx := context.Background()

	rt := NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.AddS32Component)
	require.NoError(t, err)
	defer compiled.Close(ctx)

	cc, ok := compiled.(*component.CompiledComponent)
	require.True(t, ok)

	comp := cc.Internal()
	t.Logf("Component structure:")
	t.Logf("  CoreModules: %d", len(comp.CoreModules))
	t.Logf("  CoreInstances: %d", len(comp.CoreInstances))
	t.Logf("  Aliases: %d", len(comp.Aliases))
	t.Logf("  Canonicals: %d", len(comp.Canonicals))
	t.Logf("  Exports: %d", len(comp.Exports))
	t.Logf("  Imports: %d", len(comp.Imports))

	for i, ci := range comp.CoreInstances {
		t.Logf("  CoreInstance[%d]: Kind=%v, ModuleIdx=%d", i, ci.Kind, ci.ModuleIdx)
	}
	for i, a := range comp.Aliases {
		t.Logf("  Alias[%d]: Kind=%v, CoreSort=%v, InstanceIdx=%d, ExportName=%q", i, a.Kind, a.CoreSort, a.InstanceIdx, a.ExportName)
	}
	for i, c := range comp.Canonicals {
		t.Logf("  Canonical[%d]: Kind=%v, CoreFuncIdx=%d", i, c.Kind, c.CoreFuncIdx)
	}
	for i, e := range comp.Exports {
		t.Logf("  Export[%d]: Name=%q, Kind=%v, Idx=%d", i, e.Name, e.Kind, e.Idx)
	}

	mods := cc.CompiledModules()
	t.Logf("CompiledModules: %d", len(mods))
}
