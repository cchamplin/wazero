//go:generate wit-bindgen-go generate --world plugin --out gen ./docs:calculator@0.1.0.wasm

package main

import (
	"go-div-plugin/gen/docs/calculator/plugin"
)

func init() {
	plugin.Exports.GetPluginName = GetPluginName
	plugin.Exports.Evaluate = Evaluate
}

// GetPluginName returns the name of this plugin.
func GetPluginName() string {
	return "Simple-Go-Div"
}

// Evaluate performs integer division of x by y.
// Returns 0 if y is 0 to avoid division by zero.
func Evaluate(x int32, y int32) int32 {
	if y == 0 {
		return 0
	}
	return x / y
}

func main() {}
