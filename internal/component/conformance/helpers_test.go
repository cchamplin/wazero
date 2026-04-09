// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conformance

import (
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// session1SkipReason is the shared skip message for Session 1 deferred
// tests. Defined here (in the test-only helpers file) so any of the
// remaining conformance test files that still call t.Skip can reference
// it without depending on a particular restored test file.
const session1SkipReason = "later work: async lift/lower (definitions.py async_task machinery)"

// newBuilder returns a fresh ComponentTypesBuilder. Test helper to
// keep call sites short.
func newBuilder() *types.ComponentTypesBuilder {
	return types.NewComponentTypesBuilder()
}

// newLiftContext constructs a LiftContext with the given type bag and
// a fresh top-level ComponentInstance. Suitable for tests that do not
// exercise resource handles (Session 0 default).
func newLiftContext(ct *types.ComponentTypes) *abi.LiftContext {
	return &abi.LiftContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
}

// newLowerContext constructs a LowerContext with the given type bag
// and a fresh top-level ComponentInstance.
func newLowerContext(ct *types.ComponentTypes) *abi.LowerContext {
	return &abi.LowerContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
}
