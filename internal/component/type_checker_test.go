// internal/component/type_checker_test.go
package component

import (
	"testing"
)

func TestNewTypeChecker(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
		},
	}

	tc := NewTypeChecker(c)

	if tc == nil {
		t.Fatal("NewTypeChecker returned nil")
	}
	if tc.component != c {
		t.Error("component not set correctly")
	}
	if tc.importedResources == nil {
		t.Error("importedResources map not initialized")
	}
}
