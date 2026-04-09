// Package conformance contains conformance tests for the Component Model implementation.
package conformance

import (
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestPrimitivesIntegers round-trips every int ABI type (s8..u64) through
// LowerFlat + LiftFlat, verifying Val constructors, accessors, and the
// flat ABI path for numeric types. Also exercises the canonical-abi
// narrowing/sign-extension contract by feeding raw core values directly
// into LiftFlat and comparing to the spec-specified narrowed result.
//
// Canonical test: run_tests.py:185-196 test_pairs invocations for U8Type,
// S8Type, U16Type, S16Type, U32Type, S32Type, U64Type, S64Type.
// Spec: definitions.py:1706-1708 (flatten_type numeric mapping) and
// definitions.py:1797-1808 (lift_flat_unsigned / lift_flat_signed —
// assert 0 <= i < (1<<core_width); result is i % (1<<t_width) with sign
// fold at t_width-1).
func TestPrimitivesIntegers(t *testing.T) {
	t.Run("s8", func(t *testing.T) {
		tests := []struct {
			name  string
			value int8
		}{
			{"zero", 0},
			{"positive", 42},
			{"negative", -42},
			{"min", math.MinInt8},
			{"max", math.MaxInt8},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				// Test Val constructor and accessor
				val := types.ValS8(tc.value)
				require.Equal(t, types.ValKindS8, val.Kind())
				require.Equal(t, tc.value, val.S8())

				// Test LowerFlat/LiftFlat roundtrip
				flat, err := abi.LowerFlat(nil, types.S8, val)
				require.NoError(t, err)
				require.Equal(t, 1, len(flat))

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S8, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.S8())
			})
		}
	})

	t.Run("u8", func(t *testing.T) {
		tests := []struct {
			name  string
			value uint8
		}{
			{"zero", 0},
			{"mid", 127},
			{"max", math.MaxUint8},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValU8(tc.value)
				require.Equal(t, types.ValKindU8, val.Kind())
				require.Equal(t, tc.value, val.U8())

				flat, err := abi.LowerFlat(nil, types.U8, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U8, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.U8())
			})
		}
	})

	t.Run("s16", func(t *testing.T) {
		tests := []struct {
			name  string
			value int16
		}{
			{"zero", 0},
			{"positive", 1000},
			{"negative", -1000},
			{"min", math.MinInt16},
			{"max", math.MaxInt16},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValS16(tc.value)
				require.Equal(t, types.ValKindS16, val.Kind())
				require.Equal(t, tc.value, val.S16())

				flat, err := abi.LowerFlat(nil, types.S16, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S16, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.S16())
			})
		}
	})

	t.Run("u16", func(t *testing.T) {
		tests := []struct {
			name  string
			value uint16
		}{
			{"zero", 0},
			{"mid", 32768},
			{"max", math.MaxUint16},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValU16(tc.value)
				require.Equal(t, types.ValKindU16, val.Kind())
				require.Equal(t, tc.value, val.U16())

				flat, err := abi.LowerFlat(nil, types.U16, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U16, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.U16())
			})
		}
	})

	t.Run("s32", func(t *testing.T) {
		tests := []struct {
			name  string
			value int32
		}{
			{"zero", 0},
			{"positive", 100000},
			{"negative", -100000},
			{"min", math.MinInt32},
			{"max", math.MaxInt32},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValS32(tc.value)
				require.Equal(t, types.ValKindS32, val.Kind())
				require.Equal(t, tc.value, val.S32())

				flat, err := abi.LowerFlat(nil, types.S32, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S32, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.S32())
			})
		}
	})

	t.Run("u32", func(t *testing.T) {
		tests := []struct {
			name  string
			value uint32
		}{
			{"zero", 0},
			{"mid", 0x80000000},
			{"max", math.MaxUint32},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValU32(tc.value)
				require.Equal(t, types.ValKindU32, val.Kind())
				require.Equal(t, tc.value, val.U32())

				flat, err := abi.LowerFlat(nil, types.U32, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U32, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.U32())
			})
		}
	})

	t.Run("s64", func(t *testing.T) {
		tests := []struct {
			name  string
			value int64
		}{
			{"zero", 0},
			{"positive", 9000000000000},
			{"negative", -9000000000000},
			{"min", math.MinInt64},
			{"max", math.MaxInt64},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValS64(tc.value)
				require.Equal(t, types.ValKindS64, val.Kind())
				require.Equal(t, tc.value, val.S64())

				flat, err := abi.LowerFlat(nil, types.S64, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S64, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.S64())
			})
		}
	})

	t.Run("u64", func(t *testing.T) {
		tests := []struct {
			name  string
			value uint64
		}{
			{"zero", 0},
			{"mid", 0x8000000000000000},
			{"max", math.MaxUint64},
			{"pattern", 0xDEADBEEF12345678},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValU64(tc.value)
				require.Equal(t, types.ValKindU64, val.Kind())
				require.Equal(t, tc.value, val.U64())

				flat, err := abi.LowerFlat(nil, types.U64, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U64, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.U64())
			})
		}
	})

	// spec_test_pairs feeds raw core values directly into LiftFlat and
	// asserts the narrowed/sign-folded result. Mirrors the canonical-abi
	// test_pairs invocations; see definitions.py:1797-1808
	// lift_flat_unsigned / lift_flat_signed.

	// Added from run_tests.py:185 test_pairs U8Type invocations.
	t.Run("spec_test_pairs_u8", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect uint8
		}{
			{127, 127},
			{128, 128},
			{255, 255},
			{256, 0},
			{4294967295, 255}, // (1<<32)-1
			{4294967168, 128}, // (1<<32)-128
			{4294967167, 127}, // (1<<32)-129
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.U8, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.U8(),
				"lift U8 from core 0x%X expected %d", c.core, c.expect)
		}
	})

	// Added from run_tests.py:187 test_pairs S8Type invocations.
	t.Run("spec_test_pairs_s8", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect int8
		}{
			{127, 127},
			{128, -128},
			{255, -1},
			{256, 0},
			{4294967295, -1},   // (1<<32)-1
			{4294967168, -128}, // (1<<32)-128
			{4294967167, 127},  // (1<<32)-129
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.S8, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.S8(),
				"lift S8 from core 0x%X expected %d", c.core, c.expect)
		}
	})

	// Added from run_tests.py:189 test_pairs U16Type invocations.
	t.Run("spec_test_pairs_u16", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect uint16
		}{
			{32767, 32767},
			{32768, 32768},
			{65535, 65535},
			{65536, 0},
			{4294967295, 65535}, // (1<<32)-1
			{4294934528, 32768}, // (1<<32)-32768
			{4294934527, 32767}, // (1<<32)-32769
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.U16, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.U16(),
				"lift U16 from core 0x%X expected %d", c.core, c.expect)
		}
	})

	// Added from run_tests.py:191 test_pairs S16Type invocations.
	t.Run("spec_test_pairs_s16", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect int16
		}{
			{32767, 32767},
			{32768, -32768},
			{65535, -1},
			{65536, 0},
			{4294967295, -1},     // (1<<32)-1
			{4294934528, -32768}, // (1<<32)-32768
			{4294934527, 32767},  // (1<<32)-32769
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.S16, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.S16(),
				"lift S16 from core 0x%X expected %d", c.core, c.expect)
		}
	})

	// Added from run_tests.py:193 test_pairs U32Type invocations.
	t.Run("spec_test_pairs_u32", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect uint32
		}{
			{(1 << 31) - 1, (1 << 31) - 1},
			{1 << 31, 1 << 31},
			{(1 << 32) - 1, (1 << 32) - 1},
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.U32, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.U32(),
				"lift U32 from core 0x%X expected %d", c.core, c.expect)
		}
	})

	// Added from run_tests.py:194 test_pairs S32Type invocations.
	t.Run("spec_test_pairs_s32", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect int32
		}{
			{(1 << 31) - 1, (1 << 31) - 1},
			{1 << 31, -(1 << 31)},
			{(1 << 32) - 1, -1},
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.S32, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.S32(),
				"lift S32 from core 0x%X expected %d", c.core, c.expect)
		}
	})

	// Added from run_tests.py:195 test_pairs U64Type invocations.
	t.Run("spec_test_pairs_u64", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect uint64
		}{
			{(1 << 63) - 1, (1 << 63) - 1},
			{1 << 63, 1 << 63},
			{^uint64(0), ^uint64(0)}, // (1<<64)-1
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.U64, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.U64(),
				"lift U64 from core 0x%X expected 0x%X", c.core, c.expect)
		}
	})

	// Added from run_tests.py:196 test_pairs S64Type invocations.
	t.Run("spec_test_pairs_s64", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect int64
		}{
			{(1 << 63) - 1, (1 << 63) - 1},
			{1 << 63, math.MinInt64}, // -(1<<63)
			{^uint64(0), -1},         // (1<<64)-1
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.S64, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.S64(),
				"lift S64 from core 0x%X expected %d", c.core, c.expect)
		}
	})
}

// TestPrimitivesFloats round-trips f32 and f64 through LowerFlat + LiftFlat
// and verifies NaN canonicalization for the flat lift path. wazero
// implements the DETERMINISTIC_PROFILE branch of the spec
// (definitions.py:1209 "DETERMINISTIC_PROFILE = False # or True" — wazero
// hard-codes the deterministic mapping in
// internal/component/abi/context.go:33-47 canonicalizeNaN32/64), so any
// NaN on the lift path collapses to the CANONICAL_FLOAT32_NAN /
// CANONICAL_FLOAT64_NAN bit pattern.
//
// Canonical test: run_tests.py:197-198 test_pairs for F32Type/F64Type and
// run_tests.py:231-244 test_nan32 / test_nan64 invocations (canonical NaN,
// quiet/signalling NaNs, negative NaN, +Inf, and a normal finite value).
// Spec: definitions.py:1210-1211 (CANONICAL_FLOAT32_NAN=0x7fc00000 /
// CANONICAL_FLOAT64_NAN=0x7ff8000000000000), definitions.py:1213-1223
// (canonicalize_nan32/64), definitions.py:1783-1784 (lift_flat F32Type /
// F64Type call canonicalize_nan*), and definitions.py:1395-1411
// (maybe_scramble_nan* — lower path canonicalization under the
// deterministic profile).
func TestPrimitivesFloats(t *testing.T) {
	t.Run("f32", func(t *testing.T) {
		t.Run("normal_values", func(t *testing.T) {
			tests := []struct {
				name  string
				value float32
			}{
				{"zero", 0.0},
				{"negative_zero", float32(math.Copysign(0, -1))},
				{"one", 1.0},
				{"negative_one", -1.0},
				{"pi", 3.14159},
				{"small", 1e-10},
				{"large", 1e10},
				{"min_positive", math.SmallestNonzeroFloat32},
				{"max", math.MaxFloat32},
				{"neg_max", -math.MaxFloat32},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					val := types.ValF32(tc.value)
					require.Equal(t, types.ValKindF32, val.Kind())

					// Use bit comparison for exact equality (handles -0 vs +0)
					require.Equal(t, math.Float32bits(tc.value), math.Float32bits(val.F32()))

					flat, err := abi.LowerFlat(nil, types.F32, val)
					require.NoError(t, err)

					iter := abi.NewFlatIter(flat)
					lifted, err := abi.LiftFlat(nil, types.F32, iter)
					require.NoError(t, err)

					// Use bit comparison for roundtrip
					require.Equal(t, math.Float32bits(tc.value), math.Float32bits(lifted.F32()))
				})
			}
		})

		t.Run("infinity", func(t *testing.T) {
			// Positive infinity
			val := types.ValF32(float32(math.Inf(1)))
			require.True(t, math.IsInf(float64(val.F32()), 1))

			flat, err := abi.LowerFlat(nil, types.F32, val)
			require.NoError(t, err)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F32, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(float64(lifted.F32()), 1))

			// Negative infinity
			val = types.ValF32(float32(math.Inf(-1)))
			require.True(t, math.IsInf(float64(val.F32()), -1))

			flat, err = abi.LowerFlat(nil, types.F32, val)
			require.NoError(t, err)
			iter = abi.NewFlatIter(flat)
			lifted, err = abi.LiftFlat(nil, types.F32, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(float64(lifted.F32()), -1))
		})

		t.Run("nan", func(t *testing.T) {
			// Standard NaN
			val := types.ValF32(float32(math.NaN()))
			require.True(t, math.IsNaN(float64(val.F32())))

			flat, err := abi.LowerFlat(nil, types.F32, val)
			require.NoError(t, err)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F32, iter)
			require.NoError(t, err)
			require.True(t, math.IsNaN(float64(lifted.F32())))
		})

		t.Run("nan_payloads", func(t *testing.T) {
			// Test various NaN payloads to verify they roundtrip as NaN
			// Component Model may canonicalize NaN payloads, so we just verify
			// the result is still NaN
			nanPayloads := []uint32{
				0x7FC00000, // Canonical quiet NaN
				0x7FC00001, // Quiet NaN with payload
				0x7F800001, // Signaling NaN
				0xFFC00000, // Negative quiet NaN
			}
			for _, payload := range nanPayloads {
				val := types.ValF32(math.Float32frombits(payload))
				require.True(t, math.IsNaN(float64(val.F32())), "expected NaN for payload 0x%X", payload)

				flat, err := abi.LowerFlat(nil, types.F32, val)
				require.NoError(t, err)
				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.F32, iter)
				require.NoError(t, err)
				require.True(t, math.IsNaN(float64(lifted.F32())), "expected NaN after roundtrip for payload 0x%X", payload)
			}
		})
	})

	t.Run("f64", func(t *testing.T) {
		t.Run("normal_values", func(t *testing.T) {
			tests := []struct {
				name  string
				value float64
			}{
				{"zero", 0.0},
				{"negative_zero", math.Copysign(0, -1)},
				{"one", 1.0},
				{"negative_one", -1.0},
				{"pi", 3.14159265358979323846},
				{"small", 1e-100},
				{"large", 1e100},
				{"min_positive", math.SmallestNonzeroFloat64},
				{"max", math.MaxFloat64},
				{"neg_max", -math.MaxFloat64},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					val := types.ValF64(tc.value)
					require.Equal(t, types.ValKindF64, val.Kind())

					// Use bit comparison for exact equality
					require.Equal(t, math.Float64bits(tc.value), math.Float64bits(val.F64()))

					flat, err := abi.LowerFlat(nil, types.F64, val)
					require.NoError(t, err)

					iter := abi.NewFlatIter(flat)
					lifted, err := abi.LiftFlat(nil, types.F64, iter)
					require.NoError(t, err)

					require.Equal(t, math.Float64bits(tc.value), math.Float64bits(lifted.F64()))
				})
			}
		})

		t.Run("infinity", func(t *testing.T) {
			// Positive infinity
			val := types.ValF64(math.Inf(1))
			require.True(t, math.IsInf(val.F64(), 1))

			flat, err := abi.LowerFlat(nil, types.F64, val)
			require.NoError(t, err)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F64, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(lifted.F64(), 1))

			// Negative infinity
			val = types.ValF64(math.Inf(-1))
			require.True(t, math.IsInf(val.F64(), -1))

			flat, err = abi.LowerFlat(nil, types.F64, val)
			require.NoError(t, err)
			iter = abi.NewFlatIter(flat)
			lifted, err = abi.LiftFlat(nil, types.F64, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(lifted.F64(), -1))
		})

		t.Run("nan", func(t *testing.T) {
			val := types.ValF64(math.NaN())
			require.True(t, math.IsNaN(val.F64()))

			flat, err := abi.LowerFlat(nil, types.F64, val)
			require.NoError(t, err)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F64, iter)
			require.NoError(t, err)
			require.True(t, math.IsNaN(lifted.F64()))
		})

		t.Run("nan_payloads", func(t *testing.T) {
			// Test various NaN payloads
			nanPayloads := []uint64{
				0x7FF8000000000000, // Canonical quiet NaN
				0x7FF8000000000001, // Quiet NaN with payload
				0x7FF0000000000001, // Signaling NaN
				0xFFF8000000000000, // Negative quiet NaN
			}
			for _, payload := range nanPayloads {
				val := types.ValF64(math.Float64frombits(payload))
				require.True(t, math.IsNaN(val.F64()), "expected NaN for payload 0x%X", payload)

				flat, err := abi.LowerFlat(nil, types.F64, val)
				require.NoError(t, err)
				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.F64, iter)
				require.NoError(t, err)
				require.True(t, math.IsNaN(lifted.F64()), "expected NaN after roundtrip for payload 0x%X", payload)
			}
		})
	})

	// spec_test_nan32 / spec_test_nan64 feed raw i32/i64 bit patterns
	// directly into LiftFlat and assert that wazero's DETERMINISTIC_PROFILE
	// implementation collapses every NaN to CANONICAL_FLOAT32_NAN /
	// CANONICAL_FLOAT64_NAN and leaves non-NaN values (Inf, finite)
	// bit-identical. See internal/component/abi/context.go:25-47 for the
	// wazero constants (CanonicalFloat32NaN=0x7fc00000,
	// CanonicalFloat64NaN=0x7ff8000000000000) matching
	// definitions.py:1210-1211.

	// Added from run_tests.py:231-237 test_nan32 invocations.
	t.Run("spec_test_nan32", func(t *testing.T) {
		const canonicalNaN32 uint32 = 0x7fc00000
		cases := []struct {
			inbits  uint32
			outbits uint32
			name    string
		}{
			{0x7fc00000, canonicalNaN32, "canonical_quiet_nan"},
			{0x7fc00001, canonicalNaN32, "quiet_nan_with_payload"},
			{0x7fe00000, canonicalNaN32, "alt_quiet_nan"},
			{0x7fffffff, canonicalNaN32, "all_ones_nan"},
			{0xffffffff, canonicalNaN32, "neg_all_ones_nan"},
			{0x7f800000, 0x7f800000, "positive_infinity_preserved"},
			{0x3fc00000, 0x3fc00000, "one_point_five_preserved"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				iter := abi.NewFlatIter([]uint64{uint64(c.inbits)})
				lifted, err := abi.LiftFlat(nil, types.F32, iter)
				require.NoError(t, err)
				gotBits := math.Float32bits(lifted.F32())
				require.Equal(t, c.outbits, gotBits,
					"lift F32 from 0x%08X: expected bits 0x%08X, got 0x%08X",
					c.inbits, c.outbits, gotBits)
			})
		}
	})

	// Added from run_tests.py:238-244 test_nan64 invocations.
	t.Run("spec_test_nan64", func(t *testing.T) {
		const canonicalNaN64 uint64 = 0x7ff8000000000000
		cases := []struct {
			inbits  uint64
			outbits uint64
			name    string
		}{
			{0x7ff8000000000000, canonicalNaN64, "canonical_quiet_nan"},
			{0x7ff8000000000001, canonicalNaN64, "quiet_nan_with_payload"},
			{0x7ffc000000000000, canonicalNaN64, "alt_quiet_nan"},
			{0x7fffffffffffffff, canonicalNaN64, "all_ones_nan"},
			{0xffffffffffffffff, canonicalNaN64, "neg_all_ones_nan"},
			{0x7ff0000000000000, 0x7ff0000000000000, "positive_infinity_preserved"},
			{0x3ff0000000000000, 0x3ff0000000000000, "one_point_zero_preserved"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				iter := abi.NewFlatIter([]uint64{c.inbits})
				lifted, err := abi.LiftFlat(nil, types.F64, iter)
				require.NoError(t, err)
				gotBits := math.Float64bits(lifted.F64())
				require.Equal(t, c.outbits, gotBits,
					"lift F64 from 0x%016X: expected bits 0x%016X, got 0x%016X",
					c.inbits, c.outbits, gotBits)
			})
		}
	})
}

// TestPrimitivesBools exercises ValBool round-trips and verifies that any
// non-zero i32 core value lifts to True — i.e., wazero's LiftFlat BoolType
// path matches convert_int_to_bool.
//
// Canonical test: run_tests.py:184 test_pairs(BoolType(),
// [(0,False),(1,True),(2,True),(4294967295,True)]).
// Spec: definitions.py:1205-1207 convert_int_to_bool (asserts non-negative,
// returns bool(i)) and definitions.py:1774 (lift_flat BoolType dispatch
// through convert_int_to_bool(vi.next('i32'))).
func TestPrimitivesBools(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		val := types.ValBool(true)
		require.Equal(t, types.ValKindBool, val.Kind())
		require.True(t, val.Bool())

		flat, err := abi.LowerFlat(nil, types.Bool, val)
		require.NoError(t, err)
		require.Equal(t, []uint64{1}, flat)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.Bool, iter)
		require.NoError(t, err)
		require.True(t, lifted.Bool())
	})

	t.Run("false", func(t *testing.T) {
		val := types.ValBool(false)
		require.Equal(t, types.ValKindBool, val.Kind())
		require.False(t, val.Bool())

		flat, err := abi.LowerFlat(nil, types.Bool, val)
		require.NoError(t, err)
		require.Equal(t, []uint64{0}, flat)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.Bool, iter)
		require.NoError(t, err)
		require.False(t, lifted.Bool())
	})

	t.Run("lift_nonzero_as_true", func(t *testing.T) {
		// In the Component Model, any non-zero value lifts as true
		// Test various non-zero values
		nonZeroValues := []uint64{1, 2, 0xFF, 0x100, 0xFFFFFFFF}
		for _, nzv := range nonZeroValues {
			iter := abi.NewFlatIter([]uint64{nzv})
			lifted, err := abi.LiftFlat(nil, types.Bool, iter)
			require.NoError(t, err)
			require.True(t, lifted.Bool(), "expected true for value %d", nzv)
		}
	})

	// Added from run_tests.py:184 test_pairs BoolType cases. Pins the
	// four explicit pairs (0,False),(1,True),(2,True),(4294967295,True)
	// on the direct LiftFlat-from-core path. Overlaps with
	// lift_nonzero_as_true above but anchors the exact canonical pairs.
	t.Run("spec_test_pairs_bool", func(t *testing.T) {
		cases := []struct {
			core   uint64
			expect bool
		}{
			{0, false},
			{1, true},
			{2, true},
			{4294967295, true},
		}
		for _, c := range cases {
			iter := abi.NewFlatIter([]uint64{c.core})
			lifted, err := abi.LiftFlat(nil, types.Bool, iter)
			require.NoError(t, err)
			require.Equal(t, c.expect, lifted.Bool(),
				"lift Bool from core 0x%X expected %v", c.core, c.expect)
		}
	})
}

// TestPrimitivesChars exercises the char type (Unicode scalar value) on
// both the LowerFlat and LiftFlat paths, including the surrogate
// rejection (U+D800..U+DFFF), the above-U+10FFFF rejection, and the
// 0xFFFFFFFF invalid-core-value rejection.
//
// Canonical test: run_tests.py:199-200 test_pairs(CharType(), ...) — two
// invocations covering (0,'\x00'), (65,'A'), (0xD7FF,'\uD7FF'),
// (0xD800,None), (0xDFFF,None), (0xE000,'\uE000'),
// (0x10FFFF,'\U0010FFFF'), (0x110000,None), (0xFFFFFFFF,None).
// Spec: definitions.py:1237-1241 convert_i32_to_char (asserts i>=0,
// traps when i>=0x110000 or 0xD800<=i<=0xDFFF, returns chr(i)) and
// definitions.py:1785 (lift_flat CharType dispatch through
// convert_i32_to_char(cx, vi.next('i32'))).
func TestPrimitivesChars(t *testing.T) {
	t.Run("valid_unicode_scalars", func(t *testing.T) {
		tests := []struct {
			name  string
			value rune
		}{
			{"null", 0},
			{"ascii_a", 'a'},
			{"ascii_z", 'z'},
			{"newline", '\n'},
			{"space", ' '},
			{"tilde", '~'},
			{"latin_extended", '\u00E9'},        // e with accent
			{"greek", '\u03B1'},                 // alpha
			{"cjk", '\u4E2D'},                   // Chinese character
			{"emoji", '\U0001F600'},             // grinning face
			{"emoji_rocket", '\U0001F680'},      // rocket
			{"max_bmp", '\uFFFF'},               // max BMP
			{"min_supplementary", '\U00010000'}, // min supplementary
			{"max_unicode", '\U0010FFFF'},       // max valid Unicode scalar
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValChar(tc.value)
				require.Equal(t, types.ValKindChar, val.Kind())
				require.Equal(t, tc.value, val.Char())

				flat, err := abi.LowerFlat(nil, types.Char, val)
				require.NoError(t, err)
				require.Equal(t, []uint64{uint64(tc.value)}, flat)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.Char, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.Char())
			})
		}
	})

	t.Run("surrogate_rejection", func(t *testing.T) {
		// Unicode surrogates (U+D800 to U+DFFF) are not valid Unicode scalar values
		// The char type should only accept valid Unicode scalar values
		// Note: In Go, rune can hold these values but they're not valid Unicode scalars
		// The Component Model specification requires char to be a Unicode scalar value
		surrogates := []struct {
			name  string
			value rune
		}{
			{"first_high_surrogate", 0xD800},
			{"last_high_surrogate", 0xDBFF},
			{"first_low_surrogate", 0xDC00},
			{"last_low_surrogate", 0xDFFF},
		}

		// The ABI must reject surrogates per the Component Model spec
		for _, tc := range surrogates {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValChar(tc.value)

				// LowerFlat must reject surrogate values
				_, err := abi.LowerFlat(nil, types.Char, val)
				require.Error(t, err, "LowerFlat should reject surrogate U+%04X", tc.value)
				require.Contains(t, err.Error(), "not a valid Unicode scalar value")

				// LiftFlat must reject surrogate values when reading from flat representation
				// Simulate what would happen if invalid data came from wasm
				iter := abi.NewFlatIter([]uint64{uint64(tc.value)})
				_, err = abi.LiftFlat(nil, types.Char, iter)
				require.Error(t, err, "LiftFlat should reject surrogate U+%04X", tc.value)
				require.Contains(t, err.Error(), "not a valid Unicode scalar value")
			})
		}
	})

	t.Run("invalid_char_above_max_unicode", func(t *testing.T) {
		// Values above U+10FFFF are not valid Unicode code points
		invalidValues := []struct {
			name  string
			value rune
		}{
			{"above_max_unicode", 0x110000},
			{"way_above_max", 0x1FFFFF},
		}

		for _, tc := range invalidValues {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValChar(tc.value)

				// LowerFlat must reject values above U+10FFFF
				_, err := abi.LowerFlat(nil, types.Char, val)
				require.Error(t, err, "LowerFlat should reject value U+%04X above max Unicode", tc.value)
				require.Contains(t, err.Error(), "not a valid Unicode scalar value")

				// LiftFlat must reject values above U+10FFFF
				iter := abi.NewFlatIter([]uint64{uint64(tc.value)})
				_, err = abi.LiftFlat(nil, types.Char, iter)
				require.Error(t, err, "LiftFlat should reject value U+%04X above max Unicode", tc.value)
				require.Contains(t, err.Error(), "not a valid Unicode scalar value")
			})
		}
	})

	t.Run("boundary_values", func(t *testing.T) {
		// Test boundary values around BMP and surrogates
		boundaries := []struct {
			name  string
			value rune
			valid bool // Whether it's a valid Unicode scalar value
		}{
			{"before_surrogates", 0xD7FF, true},
			{"after_surrogates", 0xE000, true},
			{"max_bmp", 0xFFFF, true},
			{"first_supplementary", 0x10000, true},
			{"max_unicode", 0x10FFFF, true},
		}
		for _, tc := range boundaries {
			if tc.valid {
				t.Run(tc.name, func(t *testing.T) {
					val := types.ValChar(tc.value)
					require.Equal(t, tc.value, val.Char())

					flat, err := abi.LowerFlat(nil, types.Char, val)
					require.NoError(t, err)

					iter := abi.NewFlatIter(flat)
					lifted, err := abi.LiftFlat(nil, types.Char, iter)
					require.NoError(t, err)
					require.Equal(t, tc.value, lifted.Char())
				})
			}
		}
	})

	// Added from run_tests.py:199-200 test_pairs CharType invocations.
	// Anchors each (core_u32, expected_scalar_or_trap) pair on the
	// direct LiftFlat path. The valid cases (0, 65, 0xD7FF, 0xE000,
	// 0x10FFFF) and some of the reject cases (surrogates, 0x110000)
	// overlap with boundary_values / surrogate_rejection /
	// invalid_char_above_max_unicode above; this sub-test adds the
	// canonical (0xFFFFFFFF, None) case missing elsewhere and pins the
	// full spec pair list in one place.
	t.Run("spec_test_pairs_char", func(t *testing.T) {
		validCases := []struct {
			core   uint32
			expect rune
			name   string
		}{
			{0, '\x00', "null"},
			{65, 'A', "ascii_A"},
			{0xD7FF, '\uD7FF', "last_before_surrogates"},
			{0xE000, '\uE000', "first_after_surrogates"},
			{0x10FFFF, '\U0010FFFF', "max_unicode"},
		}
		for _, c := range validCases {
			t.Run(c.name, func(t *testing.T) {
				iter := abi.NewFlatIter([]uint64{uint64(c.core)})
				lifted, err := abi.LiftFlat(nil, types.Char, iter)
				require.NoError(t, err)
				require.Equal(t, c.expect, lifted.Char(),
					"lift Char from 0x%X", c.core)
			})
		}

		invalidCases := []struct {
			core uint32
			name string
		}{
			{0xD800, "first_surrogate"},
			{0xDFFF, "last_surrogate"},
			{0x110000, "just_above_max"},
			{0xFFFFFFFF, "all_ones"},
		}
		for _, c := range invalidCases {
			t.Run(c.name, func(t *testing.T) {
				iter := abi.NewFlatIter([]uint64{uint64(c.core)})
				_, err := abi.LiftFlat(nil, types.Char, iter)
				require.Error(t, err,
					"LiftFlat should reject 0x%X", c.core)
				require.Contains(t, err.Error(),
					"not a valid Unicode scalar value")
			})
		}
	})
}

// TestPrimitivesTypeProperties verifies that wazero's ValType.ABI helper
// reports the canonical size, alignment and flatten count for each
// primitive type, i.e. the numeric invariants of the spec formulas.
//
// No counterpart (justified): wazero-specific accessor test exercising
// the internal ABI.Size32 / ABI.Align32 / ABI.FlattenCount helper.
// run_tests.py does not exercise elem_size / alignment directly — those
// are static spec formulas. The reference for expected values is
// definitions.py:1706-1711 (flatten_type — BoolType/U8/S8/U16/S16/U32/S32
// → i32; S64/U64 → i64; F32 → f32; F64 → f64; CharType → i32) and
// definitions.py:1706-1711 cross-referenced with the per-type
// elem_size / alignment formulas (see ValType.ABI implementation in
// internal/component/types/types.go). Verify via:
//   grep -n 'def flatten_type\|class U8Type\|class S8Type\|class CharType' \
//     debug-vendored/component-model/design/mvp/canonical-abi/definitions.py
func TestPrimitivesTypeProperties(t *testing.T) {
	tests := []struct {
		name         string
		typ          types.ValType
		size         uint32
		align        uint32
		flattenCount int
	}{
		{"bool", types.Bool, 1, 1, 1},
		{"s8", types.S8, 1, 1, 1},
		{"u8", types.U8, 1, 1, 1},
		{"s16", types.S16, 2, 2, 1},
		{"u16", types.U16, 2, 2, 1},
		{"s32", types.S32, 4, 4, 1},
		{"u32", types.U32, 4, 4, 1},
		{"s64", types.S64, 8, 8, 1},
		{"u64", types.U64, 8, 8, 1},
		{"f32", types.F32, 4, 4, 1},
		{"f64", types.F64, 8, 8, 1},
		{"char", types.Char, 4, 4, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := tc.typ.ABI(nil)
			require.Equal(t, tc.size, info.Size32, "size mismatch")
			require.Equal(t, tc.align, info.Align32, "align mismatch")
			require.Equal(t, tc.flattenCount, int(info.FlattenCount), "flatten count mismatch")
		})
	}
}

// TestPrimitivesSignExtension exercises the negative / minimum value
// cases of the signed integer round-trip path (s8/s16/s32/s64 at -1 and
// their type-min), asserting that LowerFlat stores the correct two's
// complement bit pattern (e.g. s64 -1 → 0xFFFFFFFFFFFFFFFF) and that
// LiftFlat recovers the original negative value.
//
// Canonical test: run_tests.py:187-192,194,196 — test_pairs S8Type,
// S16Type, S32Type, S64Type cases cover the negative / wrap-around
// values exercised here. This test narrows in on the s*_min / -1 pairs
// and additionally pins the s64 lowered bit pattern.
// Spec: definitions.py:1802-1808 lift_flat_signed (mod + sign fold) and
// definitions.py:1891-1894 lower_flat_signed (negative values add
// 1<<core_bits to wrap into unsigned core representation).
func TestPrimitivesSignExtension(t *testing.T) {
	t.Run("s8_sign_extension", func(t *testing.T) {
		// -1 as s8 is 0xFF, which should sign-extend properly
		val := types.ValS8(-1)
		flat, err := abi.LowerFlat(nil, types.S8, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.S8, iter)
		require.NoError(t, err)
		require.Equal(t, int8(-1), lifted.S8())

		// -128 as s8 is 0x80
		val = types.ValS8(-128)
		flat, err = abi.LowerFlat(nil, types.S8, val)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, types.S8, iter)
		require.NoError(t, err)
		require.Equal(t, int8(-128), lifted.S8())
	})

	t.Run("s16_sign_extension", func(t *testing.T) {
		val := types.ValS16(-1)
		flat, err := abi.LowerFlat(nil, types.S16, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.S16, iter)
		require.NoError(t, err)
		require.Equal(t, int16(-1), lifted.S16())

		val = types.ValS16(-32768)
		flat, err = abi.LowerFlat(nil, types.S16, val)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, types.S16, iter)
		require.NoError(t, err)
		require.Equal(t, int16(-32768), lifted.S16())
	})

	t.Run("s32_sign_extension", func(t *testing.T) {
		val := types.ValS32(-1)
		flat, err := abi.LowerFlat(nil, types.S32, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.S32, iter)
		require.NoError(t, err)
		require.Equal(t, int32(-1), lifted.S32())

		val = types.ValS32(math.MinInt32)
		flat, err = abi.LowerFlat(nil, types.S32, val)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, types.S32, iter)
		require.NoError(t, err)
		require.Equal(t, int32(math.MinInt32), lifted.S32())
	})

	t.Run("s64_sign_extension", func(t *testing.T) {
		val := types.ValS64(-1)
		flat, err := abi.LowerFlat(nil, types.S64, val)
		require.NoError(t, err)
		require.Equal(t, []uint64{0xFFFFFFFFFFFFFFFF}, flat)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.S64, iter)
		require.NoError(t, err)
		require.Equal(t, int64(-1), lifted.S64())

		val = types.ValS64(math.MinInt64)
		flat, err = abi.LowerFlat(nil, types.S64, val)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, types.S64, iter)
		require.NoError(t, err)
		require.Equal(t, int64(math.MinInt64), lifted.S64())
	})
}

// TestPrimitivesAllTypesRoundtrip is a comprehensive smoke-test that
// round-trips one or two representative values per primitive type
// through LowerFlat + LiftFlat in a single table. It does not add new
// spec coverage beyond the per-type tests above; its purpose is to
// cover the dispatch wiring of every TypeKind branch in a single pass.
//
// No counterpart (justified): wazero-specific smoke test that
// aggregates per-type round-trips so a new primitive TypeKind cannot be
// added without at least one dispatch entry. The individual spec cases
// it drives are already cited on the per-type tests above
// (TestPrimitivesIntegers, TestPrimitivesFloats, TestPrimitivesBools,
// TestPrimitivesChars); see run_tests.py:184-200 test_pairs.
func TestPrimitivesAllTypesRoundtrip(t *testing.T) {
	type testCase struct {
		name string
		typ  types.ValType
		val  types.Val
		// check is a function that verifies the lifted value matches the original
		check func(t *testing.T, original, lifted types.Val)
	}

	tests := []testCase{
		// Bool
		{"bool_true", types.Bool, types.ValBool(true),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Bool(), lifted.Bool())
			}},
		{"bool_false", types.Bool, types.ValBool(false),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Bool(), lifted.Bool())
			}},

		// Signed integers
		{"s8_min", types.S8, types.ValS8(math.MinInt8),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S8(), lifted.S8())
			}},
		{"s8_max", types.S8, types.ValS8(math.MaxInt8),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S8(), lifted.S8())
			}},
		{"s16_min", types.S16, types.ValS16(math.MinInt16),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S16(), lifted.S16())
			}},
		{"s16_max", types.S16, types.ValS16(math.MaxInt16),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S16(), lifted.S16())
			}},
		{"s32_min", types.S32, types.ValS32(math.MinInt32),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S32(), lifted.S32())
			}},
		{"s32_max", types.S32, types.ValS32(math.MaxInt32),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S32(), lifted.S32())
			}},
		{"s64_min", types.S64, types.ValS64(math.MinInt64),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S64(), lifted.S64())
			}},
		{"s64_max", types.S64, types.ValS64(math.MaxInt64),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S64(), lifted.S64())
			}},

		// Unsigned integers
		{"u8_zero", types.U8, types.ValU8(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U8(), lifted.U8())
			}},
		{"u8_max", types.U8, types.ValU8(math.MaxUint8),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U8(), lifted.U8())
			}},
		{"u16_zero", types.U16, types.ValU16(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U16(), lifted.U16())
			}},
		{"u16_max", types.U16, types.ValU16(math.MaxUint16),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U16(), lifted.U16())
			}},
		{"u32_zero", types.U32, types.ValU32(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U32(), lifted.U32())
			}},
		{"u32_max", types.U32, types.ValU32(math.MaxUint32),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U32(), lifted.U32())
			}},
		{"u64_zero", types.U64, types.ValU64(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U64(), lifted.U64())
			}},
		{"u64_max", types.U64, types.ValU64(math.MaxUint64),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U64(), lifted.U64())
			}},

		// Floats
		{"f32_zero", types.F32, types.ValF32(0.0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, math.Float32bits(orig.F32()), math.Float32bits(lifted.F32()))
			}},
		{"f32_max", types.F32, types.ValF32(math.MaxFloat32),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, math.Float32bits(orig.F32()), math.Float32bits(lifted.F32()))
			}},
		{"f32_inf", types.F32, types.ValF32(float32(math.Inf(1))),
			func(t *testing.T, orig, lifted types.Val) {
				require.True(t, math.IsInf(float64(lifted.F32()), 1))
			}},
		{"f32_nan", types.F32, types.ValF32(float32(math.NaN())),
			func(t *testing.T, orig, lifted types.Val) {
				require.True(t, math.IsNaN(float64(lifted.F32())))
			}},
		{"f64_zero", types.F64, types.ValF64(0.0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, math.Float64bits(orig.F64()), math.Float64bits(lifted.F64()))
			}},
		{"f64_max", types.F64, types.ValF64(math.MaxFloat64),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, math.Float64bits(orig.F64()), math.Float64bits(lifted.F64()))
			}},
		{"f64_inf", types.F64, types.ValF64(math.Inf(1)),
			func(t *testing.T, orig, lifted types.Val) {
				require.True(t, math.IsInf(lifted.F64(), 1))
			}},
		{"f64_nan", types.F64, types.ValF64(math.NaN()),
			func(t *testing.T, orig, lifted types.Val) {
				require.True(t, math.IsNaN(lifted.F64()))
			}},

		// Char
		{"char_null", types.Char, types.ValChar(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Char(), lifted.Char())
			}},
		{"char_ascii", types.Char, types.ValChar('A'),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Char(), lifted.Char())
			}},
		{"char_emoji", types.Char, types.ValChar('\U0001F600'),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Char(), lifted.Char())
			}},
		{"char_max", types.Char, types.ValChar('\U0010FFFF'),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Char(), lifted.Char())
			}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Lower to flat representation
			flat, err := abi.LowerFlat(nil, tc.typ, tc.val)
			require.NoError(t, err)

			// Lift back from flat representation
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, tc.typ, iter)
			require.NoError(t, err)

			// Verify the lifted value matches the original
			tc.check(t, tc.val, lifted)
		})
	}
}
