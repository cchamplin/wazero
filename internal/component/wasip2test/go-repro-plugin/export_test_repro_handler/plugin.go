package export_test_repro_handler

import (
	"strconv"

	"github.com/bytecodealliance/wit-bindgen/wit_types"
	"wit_component/test_repro_host_ops"
	"wit_component/test_repro_host_rng"
	"wit_component/test_repro_types"
)


// Process calls the host import and returns its value in a record.
func Process() test_repro_types.ProcessResult {
	v := test_repro_host_ops.GetValue()
	return test_repro_types.ProcessResult{
		Value: v,
		Ok:    true,
	}
}

// ProcessRandom calls the host import with a u64 argument.
func ProcessRandom(len uint64) uint64 {
	return test_repro_host_ops.GetRandomLen(len)
}

// ProcessRandomBytes calls host-rng.get-random-bytes (u32 -> list<u8>) to ensure
// the import is present in the component binary alongside wasi:random/random's
// get-random-bytes (u64 -> list<u8>).
func ProcessRandomBytes(count uint32) []uint8 {
	return test_repro_host_rng.GetRandomBytes(count)
}

// CallSendEnum calls the host import send-enum with an enum + string.
func CallSendEnum(c test_repro_types.Color, msg string) uint32 {
	return test_repro_host_ops.SendEnum(c, msg)
}

// CallSendEvent calls the host import send-event with a record containing option.
func CallSendEvent(event test_repro_types.EventData) uint32 {
	return test_repro_host_ops.SendEvent(event)
}

// EchoString returns the input string as-is.
func EchoString(input string) string {
	return input
}

// EchoBool returns the input bool as-is.
func EchoBool(input bool) bool {
	return input
}

// EchoS8 returns the input int8 as-is.
func EchoS8(input int8) int8 {
	return input
}

// EchoU8 returns the input uint8 as-is.
func EchoU8(input uint8) uint8 {
	return input
}

// EchoS16 returns the input int16 as-is.
func EchoS16(input int16) int16 {
	return input
}

// EchoU16 returns the input uint16 as-is.
func EchoU16(input uint16) uint16 {
	return input
}

// EchoF32 returns the input float32 as-is.
func EchoF32(input float32) float32 {
	return input
}

// EchoF64 returns the input float64 as-is.
func EchoF64(input float64) float64 {
	return input
}

// EchoChar returns the input rune as-is.
func EchoChar(input rune) rune {
	return input
}

// EchoEnum returns the input Color enum as-is.
func EchoEnum(input test_repro_types.Color) test_repro_types.Color {
	return input
}

// EchoFlags returns the input Permissions flags as-is.
func EchoFlags(input test_repro_types.Permissions) test_repro_types.Permissions {
	return input
}

// EchoVariant returns the input Shape variant as-is.
func EchoVariant(input test_repro_types.Shape) test_repro_types.Shape {
	return input
}

// MakeOk returns an Ok result with the given value.
func MakeOk(value uint32) wit_types.Result[uint32, string] {
	return wit_types.Ok[uint32, string](value)
}

// MakeErr returns an Err result with the given message.
func MakeErr(message string) wit_types.Result[uint32, string] {
	return wit_types.Err[uint32, string](message)
}

// AddThree returns the sum of three uint32 values.
func AddThree(a, b, c uint32) uint32 {
	return a + b + c
}

// ConcatStrings returns the concatenation of two strings.
func ConcatStrings(a, b string) string {
	return a + b
}

// MixedParams returns "name:count" if flag is true, otherwise just name.
func MixedParams(name string, count uint32, flag bool) string {
	if flag {
		return name + ":" + strconv.FormatUint(uint64(count), 10)
	}
	return name
}

// CallGetColor calls the host import get-color and returns the result.
func CallGetColor() test_repro_types.Color {
	return test_repro_host_ops.GetColor()
}

// CallCheckOption calls the host import check-option with the given option.
func CallCheckOption(val wit_types.Option[uint32]) uint32 {
	return test_repro_host_ops.CheckOption(val)
}

// CallGetEvent calls the host import get-event and returns the result.
func CallGetEvent() test_repro_types.EventData {
	return test_repro_host_ops.GetEvent()
}

// CallCheckOptBytes calls the host import check-opt-bytes with the given option.
func CallCheckOptBytes(data wit_types.Option[[]uint8]) uint32 {
	return test_repro_host_ops.CheckOptBytes(data)
}

// CallSendTaggedShape calls the host import send-tagged-shape with a record containing a variant.
func CallSendTaggedShape(ts test_repro_types.TaggedShape) uint32 {
	return test_repro_host_ops.SendTaggedShape(ts)
}

// CallSendEvents calls the host import send-events with a list of records.
func CallSendEvents(events []test_repro_types.EventData) uint32 {
	return test_repro_host_ops.SendEvents(events)
}
