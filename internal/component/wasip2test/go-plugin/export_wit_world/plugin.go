package export_wit_world

// GetPluginName implements the "get-plugin-name" export from the WIT world.
// WIT: export get-plugin-name: func() -> string;
func GetPluginName() string {
	return "Simple-Go-Multi"
}

// Evaluate implements the "evaluate" export from the WIT world.
// WIT: export evaluate: func(x: s32, y: s32) -> s32;
func Evaluate(x int32, y int32) int32 {
	// Simple implementation: multi the two numbers
	return x * y
}
