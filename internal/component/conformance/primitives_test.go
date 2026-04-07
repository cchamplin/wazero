// Package conformance contains conformance tests for the Component Model implementation.
package conformance

import (
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestPrimitivesIntegers tests all integer type Val constructors, accessors, and roundtrips.
// Ported from wasmtime func.rs integers tests.
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
				flat, err := abi.LowerFlat(nil, types.S8{}, val)
				require.NoError(t, err)
				require.Equal(t, 1, len(flat))

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S8{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.U8{}, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U8{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.S16{}, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S16{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.U16{}, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U16{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.S32{}, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S32{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.U32{}, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U32{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.S64{}, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S64{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.U64{}, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U64{}, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.U64())
			})
		}
	})
}

// TestPrimitivesFloats tests float32 and float64 Val constructors, accessors, and roundtrips.
// Ported from wasmtime func.rs floats tests.
// Includes special handling for NaN canonicalization.
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

					flat, err := abi.LowerFlat(nil, types.F32{}, val)
					require.NoError(t, err)

					iter := abi.NewFlatIter(flat)
					lifted, err := abi.LiftFlat(nil, types.F32{}, iter)
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

			flat, err := abi.LowerFlat(nil, types.F32{}, val)
			require.NoError(t, err)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F32{}, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(float64(lifted.F32()), 1))

			// Negative infinity
			val = types.ValF32(float32(math.Inf(-1)))
			require.True(t, math.IsInf(float64(val.F32()), -1))

			flat, err = abi.LowerFlat(nil, types.F32{}, val)
			require.NoError(t, err)
			iter = abi.NewFlatIter(flat)
			lifted, err = abi.LiftFlat(nil, types.F32{}, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(float64(lifted.F32()), -1))
		})

		t.Run("nan", func(t *testing.T) {
			// Standard NaN
			val := types.ValF32(float32(math.NaN()))
			require.True(t, math.IsNaN(float64(val.F32())))

			flat, err := abi.LowerFlat(nil, types.F32{}, val)
			require.NoError(t, err)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F32{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.F32{}, val)
				require.NoError(t, err)
				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.F32{}, iter)
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

					flat, err := abi.LowerFlat(nil, types.F64{}, val)
					require.NoError(t, err)

					iter := abi.NewFlatIter(flat)
					lifted, err := abi.LiftFlat(nil, types.F64{}, iter)
					require.NoError(t, err)

					require.Equal(t, math.Float64bits(tc.value), math.Float64bits(lifted.F64()))
				})
			}
		})

		t.Run("infinity", func(t *testing.T) {
			// Positive infinity
			val := types.ValF64(math.Inf(1))
			require.True(t, math.IsInf(val.F64(), 1))

			flat, err := abi.LowerFlat(nil, types.F64{}, val)
			require.NoError(t, err)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F64{}, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(lifted.F64(), 1))

			// Negative infinity
			val = types.ValF64(math.Inf(-1))
			require.True(t, math.IsInf(val.F64(), -1))

			flat, err = abi.LowerFlat(nil, types.F64{}, val)
			require.NoError(t, err)
			iter = abi.NewFlatIter(flat)
			lifted, err = abi.LiftFlat(nil, types.F64{}, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(lifted.F64(), -1))
		})

		t.Run("nan", func(t *testing.T) {
			val := types.ValF64(math.NaN())
			require.True(t, math.IsNaN(val.F64()))

			flat, err := abi.LowerFlat(nil, types.F64{}, val)
			require.NoError(t, err)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F64{}, iter)
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

				flat, err := abi.LowerFlat(nil, types.F64{}, val)
				require.NoError(t, err)
				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.F64{}, iter)
				require.NoError(t, err)
				require.True(t, math.IsNaN(lifted.F64()), "expected NaN after roundtrip for payload 0x%X", payload)
			}
		})
	})
}

// TestPrimitivesBools tests bool Val constructor, accessor, and roundtrip.
// Ported from wasmtime func.rs bools tests.
func TestPrimitivesBools(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		val := types.ValBool(true)
		require.Equal(t, types.ValKindBool, val.Kind())
		require.True(t, val.Bool())

		flat, err := abi.LowerFlat(nil, types.Bool{}, val)
		require.NoError(t, err)
		require.Equal(t, []uint64{1}, flat)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.Bool{}, iter)
		require.NoError(t, err)
		require.True(t, lifted.Bool())
	})

	t.Run("false", func(t *testing.T) {
		val := types.ValBool(false)
		require.Equal(t, types.ValKindBool, val.Kind())
		require.False(t, val.Bool())

		flat, err := abi.LowerFlat(nil, types.Bool{}, val)
		require.NoError(t, err)
		require.Equal(t, []uint64{0}, flat)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.Bool{}, iter)
		require.NoError(t, err)
		require.False(t, lifted.Bool())
	})

	t.Run("lift_nonzero_as_true", func(t *testing.T) {
		// In the Component Model, any non-zero value lifts as true
		// Test various non-zero values
		nonZeroValues := []uint64{1, 2, 0xFF, 0x100, 0xFFFFFFFF}
		for _, nzv := range nonZeroValues {
			iter := abi.NewFlatIter([]uint64{nzv})
			lifted, err := abi.LiftFlat(nil, types.Bool{}, iter)
			require.NoError(t, err)
			require.True(t, lifted.Bool(), "expected true for value %d", nzv)
		}
	})
}

// TestPrimitivesChars tests char (Unicode scalar value) Val constructor, accessor, and roundtrip.
// Ported from wasmtime func.rs chars tests.
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

				flat, err := abi.LowerFlat(nil, types.Char{}, val)
				require.NoError(t, err)
				require.Equal(t, []uint64{uint64(tc.value)}, flat)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.Char{}, iter)
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
				_, err := abi.LowerFlat(nil, types.Char{}, val)
				require.Error(t, err, "LowerFlat should reject surrogate U+%04X", tc.value)
				require.Contains(t, err.Error(), "not a valid Unicode scalar value")

				// LiftFlat must reject surrogate values when reading from flat representation
				// Simulate what would happen if invalid data came from wasm
				iter := abi.NewFlatIter([]uint64{uint64(tc.value)})
				_, err = abi.LiftFlat(nil, types.Char{}, iter)
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
				_, err := abi.LowerFlat(nil, types.Char{}, val)
				require.Error(t, err, "LowerFlat should reject value U+%04X above max Unicode", tc.value)
				require.Contains(t, err.Error(), "not a valid Unicode scalar value")

				// LiftFlat must reject values above U+10FFFF
				iter := abi.NewFlatIter([]uint64{uint64(tc.value)})
				_, err = abi.LiftFlat(nil, types.Char{}, iter)
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

					flat, err := abi.LowerFlat(nil, types.Char{}, val)
					require.NoError(t, err)

					iter := abi.NewFlatIter(flat)
					lifted, err := abi.LiftFlat(nil, types.Char{}, iter)
					require.NoError(t, err)
					require.Equal(t, tc.value, lifted.Char())
				})
			}
		}
	})
}

// TestPrimitivesTypeProperties tests that primitive types have correct
// size, alignment, and flatten count properties.
func TestPrimitivesTypeProperties(t *testing.T) {
	tests := []struct {
		name         string
		typ          types.ValType
		size         uint32
		align        uint32
		flattenCount int
	}{
		{"bool", types.Bool{}, 1, 1, 1},
		{"s8", types.S8{}, 1, 1, 1},
		{"u8", types.U8{}, 1, 1, 1},
		{"s16", types.S16{}, 2, 2, 1},
		{"u16", types.U16{}, 2, 2, 1},
		{"s32", types.S32{}, 4, 4, 1},
		{"u32", types.U32{}, 4, 4, 1},
		{"s64", types.S64{}, 8, 8, 1},
		{"u64", types.U64{}, 8, 8, 1},
		{"f32", types.F32{}, 4, 4, 1},
		{"f64", types.F64{}, 8, 8, 1},
		{"char", types.Char{}, 4, 4, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.size, tc.typ.Size(), "size mismatch")
			require.Equal(t, tc.align, tc.typ.Align(), "align mismatch")
			require.Equal(t, tc.flattenCount, tc.typ.FlattenCount(), "flatten count mismatch")
		})
	}
}

// TestPrimitivesSignExtension tests that signed integer types properly
// handle sign extension during lift/lower operations.
func TestPrimitivesSignExtension(t *testing.T) {
	t.Run("s8_sign_extension", func(t *testing.T) {
		// -1 as s8 is 0xFF, which should sign-extend properly
		val := types.ValS8(-1)
		flat, err := abi.LowerFlat(nil, types.S8{}, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.S8{}, iter)
		require.NoError(t, err)
		require.Equal(t, int8(-1), lifted.S8())

		// -128 as s8 is 0x80
		val = types.ValS8(-128)
		flat, err = abi.LowerFlat(nil, types.S8{}, val)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, types.S8{}, iter)
		require.NoError(t, err)
		require.Equal(t, int8(-128), lifted.S8())
	})

	t.Run("s16_sign_extension", func(t *testing.T) {
		val := types.ValS16(-1)
		flat, err := abi.LowerFlat(nil, types.S16{}, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.S16{}, iter)
		require.NoError(t, err)
		require.Equal(t, int16(-1), lifted.S16())

		val = types.ValS16(-32768)
		flat, err = abi.LowerFlat(nil, types.S16{}, val)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, types.S16{}, iter)
		require.NoError(t, err)
		require.Equal(t, int16(-32768), lifted.S16())
	})

	t.Run("s32_sign_extension", func(t *testing.T) {
		val := types.ValS32(-1)
		flat, err := abi.LowerFlat(nil, types.S32{}, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.S32{}, iter)
		require.NoError(t, err)
		require.Equal(t, int32(-1), lifted.S32())

		val = types.ValS32(math.MinInt32)
		flat, err = abi.LowerFlat(nil, types.S32{}, val)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, types.S32{}, iter)
		require.NoError(t, err)
		require.Equal(t, int32(math.MinInt32), lifted.S32())
	})

	t.Run("s64_sign_extension", func(t *testing.T) {
		val := types.ValS64(-1)
		flat, err := abi.LowerFlat(nil, types.S64{}, val)
		require.NoError(t, err)
		require.Equal(t, []uint64{0xFFFFFFFFFFFFFFFF}, flat)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, types.S64{}, iter)
		require.NoError(t, err)
		require.Equal(t, int64(-1), lifted.S64())

		val = types.ValS64(math.MinInt64)
		flat, err = abi.LowerFlat(nil, types.S64{}, val)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, types.S64{}, iter)
		require.NoError(t, err)
		require.Equal(t, int64(math.MinInt64), lifted.S64())
	})
}

// TestPrimitivesAllTypesRoundtrip is a comprehensive table-driven test
// that verifies roundtrip behavior for all primitive types.
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
		{"bool_true", types.Bool{}, types.ValBool(true),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Bool(), lifted.Bool())
			}},
		{"bool_false", types.Bool{}, types.ValBool(false),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Bool(), lifted.Bool())
			}},

		// Signed integers
		{"s8_min", types.S8{}, types.ValS8(math.MinInt8),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S8(), lifted.S8())
			}},
		{"s8_max", types.S8{}, types.ValS8(math.MaxInt8),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S8(), lifted.S8())
			}},
		{"s16_min", types.S16{}, types.ValS16(math.MinInt16),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S16(), lifted.S16())
			}},
		{"s16_max", types.S16{}, types.ValS16(math.MaxInt16),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S16(), lifted.S16())
			}},
		{"s32_min", types.S32{}, types.ValS32(math.MinInt32),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S32(), lifted.S32())
			}},
		{"s32_max", types.S32{}, types.ValS32(math.MaxInt32),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S32(), lifted.S32())
			}},
		{"s64_min", types.S64{}, types.ValS64(math.MinInt64),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S64(), lifted.S64())
			}},
		{"s64_max", types.S64{}, types.ValS64(math.MaxInt64),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.S64(), lifted.S64())
			}},

		// Unsigned integers
		{"u8_zero", types.U8{}, types.ValU8(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U8(), lifted.U8())
			}},
		{"u8_max", types.U8{}, types.ValU8(math.MaxUint8),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U8(), lifted.U8())
			}},
		{"u16_zero", types.U16{}, types.ValU16(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U16(), lifted.U16())
			}},
		{"u16_max", types.U16{}, types.ValU16(math.MaxUint16),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U16(), lifted.U16())
			}},
		{"u32_zero", types.U32{}, types.ValU32(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U32(), lifted.U32())
			}},
		{"u32_max", types.U32{}, types.ValU32(math.MaxUint32),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U32(), lifted.U32())
			}},
		{"u64_zero", types.U64{}, types.ValU64(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U64(), lifted.U64())
			}},
		{"u64_max", types.U64{}, types.ValU64(math.MaxUint64),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.U64(), lifted.U64())
			}},

		// Floats
		{"f32_zero", types.F32{}, types.ValF32(0.0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, math.Float32bits(orig.F32()), math.Float32bits(lifted.F32()))
			}},
		{"f32_max", types.F32{}, types.ValF32(math.MaxFloat32),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, math.Float32bits(orig.F32()), math.Float32bits(lifted.F32()))
			}},
		{"f32_inf", types.F32{}, types.ValF32(float32(math.Inf(1))),
			func(t *testing.T, orig, lifted types.Val) {
				require.True(t, math.IsInf(float64(lifted.F32()), 1))
			}},
		{"f32_nan", types.F32{}, types.ValF32(float32(math.NaN())),
			func(t *testing.T, orig, lifted types.Val) {
				require.True(t, math.IsNaN(float64(lifted.F32())))
			}},
		{"f64_zero", types.F64{}, types.ValF64(0.0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, math.Float64bits(orig.F64()), math.Float64bits(lifted.F64()))
			}},
		{"f64_max", types.F64{}, types.ValF64(math.MaxFloat64),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, math.Float64bits(orig.F64()), math.Float64bits(lifted.F64()))
			}},
		{"f64_inf", types.F64{}, types.ValF64(math.Inf(1)),
			func(t *testing.T, orig, lifted types.Val) {
				require.True(t, math.IsInf(lifted.F64(), 1))
			}},
		{"f64_nan", types.F64{}, types.ValF64(math.NaN()),
			func(t *testing.T, orig, lifted types.Val) {
				require.True(t, math.IsNaN(lifted.F64()))
			}},

		// Char
		{"char_null", types.Char{}, types.ValChar(0),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Char(), lifted.Char())
			}},
		{"char_ascii", types.Char{}, types.ValChar('A'),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Char(), lifted.Char())
			}},
		{"char_emoji", types.Char{}, types.ValChar('\U0001F600'),
			func(t *testing.T, orig, lifted types.Val) {
				require.Equal(t, orig.Char(), lifted.Char())
			}},
		{"char_max", types.Char{}, types.ValChar('\U0010FFFF'),
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
