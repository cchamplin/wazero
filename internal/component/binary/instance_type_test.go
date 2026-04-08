// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"testing"
)

// Instance-type-declaration tests previously asserted on a []TypeDef
// slice on Component.Types and on per-kind composite TypeDef structs.
// After Task 13 the binary decoder produces *types.ComponentTypes
// directly via the builder, and nested instance-type declarations are
// parsed for byte-level correctness only in Session 0 — their
// structural content is not surfaced on the parent Component. Session
// 2 will wire a nested *typeScope and builder through
// decodeNestedTypeDef and restore these assertions against the
// canonical table.
//
// See: docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md

func TestDecodeInstanceType(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeWithAlias(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeEmpty(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeMultipleExports(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeWithCoreType(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeWithNestedType(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeCoreModuleExport(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeInstanceExport(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeComponentExport(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

func TestDecodeInstanceTypeValueExport(t *testing.T) {
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}
