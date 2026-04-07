// internal/component/value_import_test.go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestValueImport(t *testing.T) {
	// Component that imports a value
	c := &Component{
		Imports: []Import{
			{
				Name: "config/name",
				ExternDesc: ImportExternDesc{
					Kind: ImportExternDescValue,
				},
			},
		},
	}

	compiled := &CompiledComponent{
		component: c,
	}

	linker := NewComponentLinker(nil)

	// Define the value
	linker.DefineValue("config", "name", types.ValString("TestApp"))

	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, compiled)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	// Value should be in value index space
	val, err := inst.GetValue(0)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if val.StringVal() != "TestApp" {
		t.Errorf("expected 'TestApp', got '%s'", val.StringVal())
	}
}
