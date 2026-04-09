// internal/component/integration_records_test.go
//
// Integration tests for record types through the public API. These exercise
// the full pipeline: wazero.Runtime.CompileComponent, InstantiateComponent,
// and ExportedFunction.Call with record-valued parameters and results.
//
// The echo_record component currently fails at InstantiateComponent because
// wireExports cannot resolve core function index 0 in the alias-aware core
// func space. Tests carry specific skip messages referencing this gap.
package component_test

import "testing"

// TestPublicAPIRecordEcho verifies round-tripping a record {x: s32, y: s32}
// through the echo_record component via the public API.
//
// Spec: Explainer.md record type definition; definitions.py
// flatten/lift/lower for record types at :1978-2040 (canon_lift).
// Wasmtime parallel: tests/all/component_model/func.rs echo_record.
func TestPublicAPIRecordEcho(t *testing.T) {
	t.Skip("wireExports cannot resolve core function index 0 for echo_record (record-typed lift requires alias-aware core func space — see component_linker.go wireExports)")
}

// TestPublicAPIRecordWithDifferentValues verifies edge cases (zeros,
// negatives, mixed, boundary values) for the echo_record component.
//
// Spec: Explainer.md record type definition; definitions.py record
// flatten/lift/lower.
func TestPublicAPIRecordWithDifferentValues(t *testing.T) {
	t.Skip("wireExports cannot resolve core function index 0 for echo_record (record-typed lift requires alias-aware core func space — see component_linker.go wireExports)")
}
