// Package conformance contains conformance tests for the Component Model implementation.
//
// These tests are ported from wasmtime's test suite (tests/all/component_model/)
// to verify that our implementation correctly handles all Component Model types
// and operations according to the specification.
//
// Test categories:
//   - primitives_test.go: Integer, float, bool, and char type roundtrips
//   - (future) strings_test.go: String encoding/decoding tests
//   - (future) records_test.go: Record type tests
//   - (future) variants_test.go: Variant type tests
//   - (future) lists_test.go: List type tests
//   - (future) resources_test.go: Resource handle tests
package conformance
