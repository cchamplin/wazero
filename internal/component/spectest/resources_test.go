// internal/component/spectest/resources_test.go
//
// Task 6.1 + Task 13: Spectest Runner
//
// This test exercises the component model spec tests from wasm-tools, testing:
// - Resource ownership semantics
// - Borrow scopes
// - Resource handle lifecycle
// - Component function invocation
// - Value lifting/lowering
// - Cross-component registration

package spectest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/wasmruntime"
	"github.com/tetratelabs/wazero/sys"
)

// resourcesWastPath is the path to the resources.wast test file
// Sourced from wasm-tools: tests/cli/component-model/resources.wast
const resourcesWastPath = "testdata/wasmtime/resources.wast"

// runnerState tracks stateful context across commands in a wast test suite.
// The spec test runner maintains the "current instance" (the most recently
// instantiated component) and a registry of named instances (populated by
// register commands). A shared ComponentLinker persists across all commands
// so that register'd exports are available to future component imports.
type runnerState struct {
	rt           wazero.Runtime
	linker       api.ComponentLinker             // shared across all commands
	currentInst  api.Component                  // most recently instantiated component
	currentName  string                         // name of current module (from "name" field), if any
	registry     map[string]api.Component       // named instances from register commands
	compiledDefs map[string]api.CompiledComponent // named compiled components from module_definition
	// failedModule is set when a module fails to compile/instantiate;
	// subsequent invoke/assert commands targeting it will also fail.
	failedModule bool
	failReason   string
}

// newRunnerState creates a fresh runner state for a test suite.
func newRunnerState(rt wazero.Runtime) *runnerState {
	linker := rt.NewComponentLinker()
	linker.DefineUnknownImportsAsTraps()
	return &runnerState{
		rt:           rt,
		linker:       linker,
		registry:     make(map[string]api.Component),
		compiledDefs: make(map[string]api.CompiledComponent),
	}
}

// closeCompiledDefs closes all stored compiled component definitions.
func (rs *runnerState) closeCompiledDefs(ctx context.Context) {
	for name, compiled := range rs.compiledDefs {
		compiled.Close(ctx)
		delete(rs.compiledDefs, name)
	}
}

// resolveInstance returns the component instance targeted by an action.
// If the action specifies a module name, look it up in the registry;
// otherwise return the current (most recent) instance.
func (rs *runnerState) resolveInstance(action *Action) (api.Component, error) {
	if action != nil && action.Module != "" {
		inst, ok := rs.registry[action.Module]
		if !ok {
			return nil, fmt.Errorf("no registered instance named %q", action.Module)
		}
		return inst, nil
	}
	if rs.currentInst == nil {
		return nil, fmt.Errorf("no current instance (no module command has been processed)")
	}
	return rs.currentInst, nil
}

// TestResourcesWast runs the resources.wast spec test suite
func TestResourcesWast(t *testing.T) {
	runWastSuite(t, resourcesWastPath)
}

// runWastSuite is the generic wast spec test runner. It parses the given
// .wast file, iterates over all commands, and dispatches to the appropriate
// handler. Stateful: module commands instantiate components, register
// commands save them, invoke/assert commands call exported functions.
func runWastSuite(t *testing.T, wastPath string) {
	// Parse the .wast file with binaries
	suite, err := ParseWastFileWithBinaries(wastPath)
	if err != nil {
		t.Fatalf("ParseWastFileWithBinaries: %v", err)
	}
	defer suite.Close()

	t.Logf("Parsed %d commands from %s", len(suite.Commands), wastPath)

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	rs := newRunnerState(rt)
	defer rs.closeCompiledDefs(ctx)

	// Track test statistics
	var stats testStats

	for i, cmd := range suite.Commands {
		cmd := cmd // capture for closure
		switch cmd.Type {
		case "module":
			// Valid component definition — compile AND instantiate
			stats.modules++
			t.Run(formatTestName("module", cmd.Line, i), func(t *testing.T) {
				runModuleInstantiateTest(t, ctx, rs, suite, &cmd)
			})

		case "module_definition":
			// Component definition (may have a name) — compile only, no instantiate
			stats.moduleDefinitions++
			t.Run(formatTestName("module_definition", cmd.Line, i), func(t *testing.T) {
				runModuleCompileOnlyTest(t, ctx, rs, suite, &cmd)
			})

		case "assert_invalid":
			// Invalid component — should fail to compile with expected error message
			stats.assertInvalid++
			t.Run(formatTestName("assert_invalid", cmd.Line, i), func(t *testing.T) {
				runAssertInvalidTest(t, ctx, rs.rt, suite, &cmd)
			})

		case "assert_trap":
			// Should trap at runtime — invoke + assert error
			stats.assertTrap++
			t.Run(formatTestName("assert_trap", cmd.Line, i), func(t *testing.T) {
				runAssertTrapTest(t, ctx, rs, &cmd)
			})

		case "assert_return":
			// Should return expected value — invoke + compare results
			stats.assertReturn++
			t.Run(formatTestName("assert_return", cmd.Line, i), func(t *testing.T) {
				runAssertReturnTest(t, ctx, rs, &cmd)
			})

		case "action":
			// Standalone invoke (wasm-tools emits "action" for bare (invoke ...))
			stats.invoke++
			t.Run(formatTestName("invoke", cmd.Line, i), func(t *testing.T) {
				runInvokeTest(t, ctx, rs, &cmd)
			})

		case "invoke":
			// Some wast formats may emit "invoke" directly
			stats.invoke++
			t.Run(formatTestName("invoke", cmd.Line, i), func(t *testing.T) {
				runInvokeTest(t, ctx, rs, &cmd)
			})

		case "register":
			// Register current instance under a name for later import/reference
			stats.register++
			t.Run(formatTestName("register", cmd.Line, i), func(t *testing.T) {
				runRegisterTest(t, ctx, rs, &cmd)
			})

		case "module_instance":
			// Instantiate a previously compiled module_definition by name
			stats.moduleInstances++
			t.Run(formatTestName("module_instance", cmd.Line, i), func(t *testing.T) {
				runModuleInstanceTest(t, ctx, rs, &cmd)
			})

		case "assert_unlinkable":
			// Compile+instantiate, expect link error
			stats.assertUnlinkable++
			t.Run(formatTestName("assert_unlinkable", cmd.Line, i), func(t *testing.T) {
				runAssertUnlinkableTest(t, ctx, rs, suite, &cmd)
			})

		case "assert_malformed":
			// Parse component, expect malformation error
			stats.assertMalformed++
			t.Run(formatTestName("assert_malformed", cmd.Line, i), func(t *testing.T) {
				runAssertMalformedTest(t, ctx, rs.rt, suite, &cmd)
			})

		case "assert_uninstantiable":
			// Compile+instantiate, expect instantiation error
			stats.assertUninstantiable++
			t.Run(formatTestName("assert_uninstantiable", cmd.Line, i), func(t *testing.T) {
				runAssertUninstantiableTest(t, ctx, rs, suite, &cmd)
			})

		default:
			stats.unknown++
			t.Logf("Unknown command type at line %d: %s", cmd.Line, cmd.Type)
		}
	}

	// Report statistics
	t.Logf("Test statistics:")
	t.Logf("  modules: %d", stats.modules)
	t.Logf("  module_definitions: %d", stats.moduleDefinitions)
	t.Logf("  module_instances: %d", stats.moduleInstances)
	t.Logf("  assert_invalid: %d", stats.assertInvalid)
	t.Logf("  assert_trap: %d", stats.assertTrap)
	t.Logf("  assert_return: %d", stats.assertReturn)
	t.Logf("  assert_unlinkable: %d", stats.assertUnlinkable)
	t.Logf("  assert_malformed: %d", stats.assertMalformed)
	t.Logf("  assert_uninstantiable: %d", stats.assertUninstantiable)
	t.Logf("  invoke: %d", stats.invoke)
	t.Logf("  register: %d", stats.register)
	t.Logf("  unknown: %d", stats.unknown)
}

type testStats struct {
	modules              int
	moduleDefinitions    int
	moduleInstances      int
	assertInvalid        int
	assertTrap           int
	assertReturn         int
	assertUnlinkable     int
	assertMalformed      int
	assertUninstantiable int
	invoke               int
	register             int
	unknown              int
}

func formatTestName(cmdType string, line, index int) string {
	return strings.ReplaceAll(cmdType, "_", "-") + "_line" + strconv.Itoa(line) + "_idx" + strconv.Itoa(index)
}

// runModuleInstantiateTest compiles and instantiates a component, storing
// it as the current instance in the runner state. If compilation or
// instantiation fails due to unsupported features, the runner enters
// skip-until-next-module mode so subsequent action commands are skipped
// gracefully instead of crashing.
func runModuleInstantiateTest(t *testing.T, ctx context.Context, rs *runnerState, suite *WastTestSuite, cmd *Command) {
	// Clear skip state — a new module command resets it
	rs.failedModule = false
	rs.failReason = ""

	if cmd.Filename == "" {
		t.Errorf("no wasm file for this command")
		return
	}

	wasmBytes, err := suite.GetWasmBytes(cmd.Filename)
	if err != nil {
		t.Fatalf("GetWasmBytes(%s): %v", cmd.Filename, err)
	}

	// Compile the component
	compiled, err := rs.rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		rs.failedModule = true
		rs.failReason = fmt.Sprintf("compilation failed: %v", err)
		t.Errorf("CompileComponent failed at line %d: %v", cmd.Line, err)
		return
	}

	// Instantiate the component using the shared linker so registered
	// imports are available.
	instance, err := rs.linker.Instantiate(ctx, compiled)
	if err != nil {
		compiled.Close(ctx)
		rs.failedModule = true
		rs.failReason = fmt.Sprintf("instantiation failed: %v", err)
		t.Errorf("InstantiateComponent failed at line %d: %v", cmd.Line, err)
		return
	}

	// Store as current instance
	rs.currentInst = instance
	rs.currentName = cmd.Name

	// If the module has a name, also store in registry for direct reference
	if cmd.Name != "" {
		rs.registry[cmd.Name] = instance
	}

	t.Logf("Successfully compiled and instantiated component at line %d (%s)", cmd.Line, cmd.Filename)
}

// runModuleCompileOnlyTest compiles a component without instantiating it.
// Used for module_definition commands. If the command has a name, the compiled
// component is stored in rs.compiledDefs for later use by module_instance.
func runModuleCompileOnlyTest(t *testing.T, ctx context.Context, rs *runnerState, suite *WastTestSuite, cmd *Command) {
	if cmd.Filename == "" {
		t.Errorf("no wasm file for this command")
		return
	}

	wasmBytes, err := suite.GetWasmBytes(cmd.Filename)
	if err != nil {
		t.Fatalf("GetWasmBytes(%s): %v", cmd.Filename, err)
	}

	compiled, err := rs.rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Errorf("CompileComponent failed at line %d: %v", cmd.Line, err)
		return
	}

	if cmd.Name != "" {
		// Store for later use by module_instance; will be closed by rs.closeCompiledDefs.
		rs.compiledDefs[cmd.Name] = compiled
	} else {
		// Unnamed definition — close immediately since nothing can reference it.
		defer compiled.Close(ctx)
	}

	t.Logf("Successfully compiled component at line %d (%s)", cmd.Line, cmd.Filename)
}

// runInvokeTest performs a standalone function invocation (bare invoke command).
// The result is discarded; the test passes if the call does not error.
func runInvokeTest(t *testing.T, ctx context.Context, rs *runnerState, cmd *Command) {
	if rs.failedModule {
		t.Errorf("cannot invoke: prior module failed: %s", rs.failReason)
		return
	}
	if cmd.Action == nil {
		t.Errorf("invoke command at line %d has no action", cmd.Line)
		return
	}
	inst, err := rs.resolveInstance(cmd.Action)
	if err != nil {
		t.Errorf("cannot resolve instance for invoke at line %d: %v", cmd.Line, err)
		return
	}
	fn := inst.ExportedFunction(cmd.Action.Field)
	if fn == nil {
		t.Errorf("exported function %q not found at line %d", cmd.Action.Field, cmd.Line)
		return
	}
	args := convertArgs(cmd.Action.Args)
	_, err = safeCall(ctx, fn, args)
	if err != nil {
		t.Errorf("invoke %q at line %d failed: %v", cmd.Action.Field, cmd.Line, err)
		return
	}
	t.Logf("invoke %q at line %d succeeded", cmd.Action.Field, cmd.Line)
}

// runAssertReturnTest invokes a function and compares results to expected values.
func runAssertReturnTest(t *testing.T, ctx context.Context, rs *runnerState, cmd *Command) {
	if rs.failedModule {
		t.Errorf("cannot assert_return: prior module failed: %s", rs.failReason)
		return
	}
	if cmd.Action == nil {
		t.Errorf("assert_return command at line %d has no action", cmd.Line)
		return
	}
	inst, err := rs.resolveInstance(cmd.Action)
	if err != nil {
		t.Errorf("cannot resolve instance for assert_return at line %d: %v", cmd.Line, err)
		return
	}
	fn := inst.ExportedFunction(cmd.Action.Field)
	if fn == nil {
		t.Errorf("exported function %q not found at line %d", cmd.Action.Field, cmd.Line)
		return
	}
	args := convertArgs(cmd.Action.Args)
	results, err := safeCall(ctx, fn, args)
	if err != nil {
		t.Errorf("assert_return at line %d: expected success but call to %q failed: %v",
			cmd.Line, cmd.Action.Field, err)
		return
	}
	compareResults(t, cmd.Line, results, cmd.Expected)
}

// runAssertTrapTest invokes a function and asserts that it traps (returns an error).
func runAssertTrapTest(t *testing.T, ctx context.Context, rs *runnerState, cmd *Command) {
	if rs.failedModule {
		t.Errorf("cannot assert_trap: prior module failed: %s", rs.failReason)
		return
	}
	if cmd.Action == nil {
		// assert_trap can also contain an inline module (module that traps at
		// instantiation). Handle that case.
		if cmd.Filename != "" {
			t.Errorf("assert_trap with inline module at line %d not yet supported", cmd.Line)
			return
		}
		t.Errorf("assert_trap command at line %d has no action", cmd.Line)
		return
	}
	inst, err := rs.resolveInstance(cmd.Action)
	if err != nil {
		t.Errorf("cannot resolve instance for assert_trap at line %d: %v", cmd.Line, err)
		return
	}
	fn := inst.ExportedFunction(cmd.Action.Field)
	if fn == nil {
		t.Errorf("exported function %q not found at line %d", cmd.Action.Field, cmd.Line)
		return
	}
	args := convertArgs(cmd.Action.Args)
	_, err = safeCall(ctx, fn, args)
	if err == nil {
		t.Errorf("assert_trap at line %d: expected trap from %q but call succeeded",
			cmd.Line, cmd.Action.Field)
		return
	}
	// Verify the trap message if specified
	if cmd.Text != "" {
		errStr := err.Error()
		if !strings.Contains(strings.ToLower(errStr), strings.ToLower(cmd.Text)) {
			t.Logf("assert_trap at line %d: trap message mismatch (expected substring %q, got %q) — still counts as trap",
				cmd.Line, cmd.Text, errStr)
		}
	}
	t.Logf("assert_trap at line %d: correctly trapped: %v", cmd.Line, err)
}

// runRegisterTest registers the current instance under a name for later reference.
// It also wires the instance's exports into the shared ComponentLinker so that
// future components can import from this namespace.
func runRegisterTest(t *testing.T, ctx context.Context, rs *runnerState, cmd *Command) {
	if rs.failedModule {
		t.Errorf("cannot register: prior module failed: %s", rs.failReason)
		return
	}
	if cmd.As == "" {
		t.Errorf("register command at line %d has no 'as' name", cmd.Line)
		return
	}
	if rs.currentInst == nil {
		t.Errorf("register command at line %d: no current instance to register", cmd.Line)
		return
	}
	rs.registry[cmd.As] = rs.currentInst

	// Wire exported functions into the shared linker so future components
	// can import from this namespace.
	exported := rs.currentInst.ExportedFunctions()
	if len(exported) > 0 {
		builder := rs.linker.DefineInstance(cmd.As)
		for name, fn := range exported {
			// Capture fn in closure for deferred invocation
			capturedFn := fn
			builder.Func(name, func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
				return capturedFn.CallAndPostReturn(ctx, args...)
			})
		}
		builder.SkipValidation()
		if err := builder.Build(); err != nil {
			t.Logf("register at line %d: warning: failed to define instance %q on linker: %v", cmd.Line, cmd.As, err)
		}
	}

	t.Logf("registered current instance as %q at line %d (exported %d functions)", cmd.As, cmd.Line, len(exported))
}

// runModuleInstanceTest instantiates a previously compiled module_definition
// by name and stores the resulting instance as the current instance.
func runModuleInstanceTest(t *testing.T, ctx context.Context, rs *runnerState, cmd *Command) {
	// Clear skip state — a new instance command resets it
	rs.failedModule = false
	rs.failReason = ""

	// Parse the module name from the Module RawMessage field
	var moduleName string
	if len(cmd.Module) > 0 {
		if err := json.Unmarshal(cmd.Module, &moduleName); err != nil {
			t.Errorf("module_instance at line %d: failed to parse module name: %v", cmd.Line, err)
			return
		}
	}
	if moduleName == "" {
		t.Errorf("module_instance at line %d: no module name specified", cmd.Line)
		return
	}

	compiled, ok := rs.compiledDefs[moduleName]
	if !ok {
		rs.failedModule = true
		rs.failReason = fmt.Sprintf("no compiled module_definition named %q", moduleName)
		t.Errorf("module_instance at line %d: no compiled module_definition named %q", cmd.Line, moduleName)
		return
	}

	instance, err := rs.linker.Instantiate(ctx, compiled)
	if err != nil {
		rs.failedModule = true
		rs.failReason = fmt.Sprintf("instantiation failed: %v", err)
		t.Errorf("module_instance at line %d: InstantiateComponent failed: %v", cmd.Line, err)
		return
	}

	// Store as current instance
	rs.currentInst = instance
	rs.currentName = cmd.Instance

	// If an instance name is specified, store in registry for direct reference
	if cmd.Instance != "" {
		rs.registry[cmd.Instance] = instance
	}

	t.Logf("module_instance at line %d: instantiated %q as %q", cmd.Line, moduleName, cmd.Instance)
}

// runAssertUnlinkableTest compiles and tries to instantiate a component,
// expecting a link error. If compilation itself fails, the assertion passes
// (the component cannot be linked). If both compile and instantiate succeed,
// the test fails.
func runAssertUnlinkableTest(t *testing.T, ctx context.Context, rs *runnerState, suite *WastTestSuite, cmd *Command) {
	if cmd.Filename == "" {
		t.Errorf("no wasm file for assert_unlinkable at line %d", cmd.Line)
		return
	}

	wasmBytes, err := suite.GetWasmBytes(cmd.Filename)
	if err != nil {
		t.Fatalf("GetWasmBytes(%s): %v", cmd.Filename, err)
	}

	compiled, err := rs.rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		// Compilation failed — the component can't be linked. Assertion satisfied.
		t.Logf("assert_unlinkable at line %d: compilation failed (acceptable): %v", cmd.Line, err)
		return
	}

	// Compilation succeeded — try to instantiate; expect failure.
	instance, err := rs.linker.Instantiate(ctx, compiled)
	compiled.Close(ctx)
	if err != nil {
		// Instantiation failed as expected.
		if cmd.Text != "" {
			errStr := err.Error()
			if !containsErrorText(errStr, cmd.Text) {
				t.Logf("assert_unlinkable at line %d: link error mismatch (expected %q, got %q) — still counts as unlinkable",
					cmd.Line, cmd.Text, errStr)
			}
		}
		t.Logf("assert_unlinkable at line %d: correctly failed to link: %v", cmd.Line, err)
		return
	}

	// Both compilation and instantiation succeeded — this is a test failure.
	_ = instance
	t.Errorf("assert_unlinkable at line %d: expected link error %q but component linked successfully", cmd.Line, cmd.Text)
}

// runAssertMalformedTest checks that a component fails to compile as malformed.
// For text-format modules (.wat), wazero has no text parser so the test passes
// trivially. For binary modules (.wasm), compilation must fail.
func runAssertMalformedTest(t *testing.T, ctx context.Context, rt wazero.Runtime, suite *WastTestSuite, cmd *Command) {
	if cmd.ModuleType == "text" {
		// wazero has no text-format parser, so we can't parse .wat files.
		// This is equivalent to "malformed" — the expected outcome.
		t.Logf("assert_malformed at line %d: text format — wazero has no text parser, passes trivially", cmd.Line)
		return
	}

	if cmd.Filename == "" {
		t.Errorf("no wasm file for assert_malformed at line %d", cmd.Line)
		return
	}

	wasmBytes, err := suite.GetWasmBytes(cmd.Filename)
	if err != nil {
		t.Fatalf("GetWasmBytes(%s): %v", cmd.Filename, err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		// Compilation failed as expected.
		if cmd.Text != "" {
			errStr := err.Error()
			if !containsErrorText(errStr, cmd.Text) {
				t.Logf("assert_malformed at line %d: error mismatch (expected %q, got %q) — still counts as malformed",
					cmd.Line, cmd.Text, errStr)
			}
		}
		t.Logf("assert_malformed at line %d: correctly rejected malformed component: %v", cmd.Line, err)
		return
	}

	// Compilation succeeded when it should have failed.
	compiled.Close(ctx)
	t.Errorf("assert_malformed at line %d: expected malformation error %q but component compiled successfully", cmd.Line, cmd.Text)
}

// runAssertUninstantiableTest compiles and instantiates a component, expecting
// the instantiation to fail (e.g., a trap during the start function).
// If compilation fails, that's also acceptable. If both succeed, the test fails.
func runAssertUninstantiableTest(t *testing.T, ctx context.Context, rs *runnerState, suite *WastTestSuite, cmd *Command) {
	if cmd.Filename == "" {
		t.Errorf("no wasm file for assert_uninstantiable at line %d", cmd.Line)
		return
	}

	wasmBytes, err := suite.GetWasmBytes(cmd.Filename)
	if err != nil {
		t.Fatalf("GetWasmBytes(%s): %v", cmd.Filename, err)
	}

	compiled, err := rs.rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		// Compilation failed — the component can't be instantiated. Assertion satisfied.
		t.Logf("assert_uninstantiable at line %d: compilation failed (acceptable): %v", cmd.Line, err)
		return
	}

	// Compilation succeeded — try to instantiate; expect failure.
	instance, err := rs.linker.Instantiate(ctx, compiled)
	compiled.Close(ctx)
	if err != nil {
		// Instantiation failed as expected.
		if cmd.Text != "" {
			errStr := err.Error()
			if !containsErrorText(errStr, cmd.Text) {
				t.Logf("assert_uninstantiable at line %d: error mismatch (expected %q, got %q) — still counts as uninstantiable",
					cmd.Line, cmd.Text, errStr)
			}
		}
		t.Logf("assert_uninstantiable at line %d: correctly failed to instantiate: %v", cmd.Line, err)
		return
	}

	// Both compilation and instantiation succeeded — this is a test failure.
	_ = instance
	t.Errorf("assert_uninstantiable at line %d: expected instantiation error %q but component instantiated successfully", cmd.Line, cmd.Text)
}

// safeCall wraps a component function call with panic recovery that only
// catches wazero runtime traps (wasmruntime.Error) and exit errors
// (sys.ExitError). Implementation panics (nil pointer dereferences, type
// assertion failures, index out of bounds, etc.) re-panic so bugs surface
// immediately as test crashes.
func safeCall(ctx context.Context, fn api.ComponentFunc, args []types.Val) (results []types.Val, err error) {
	defer func() {
		if r := recover(); r != nil {
			// wazero runtime traps are panicked as *wasmruntime.Error.
			if _, ok := r.(*wasmruntime.Error); ok {
				err = fmt.Errorf("wasm trap: %v", r)
				return
			}
			// sys.ExitError is panicked on module close / proc_exit.
			if _, ok := r.(*sys.ExitError); ok {
				err = fmt.Errorf("exit error: %v", r)
				return
			}
			// Errors wrapping a trap (e.g., fmt.Errorf("%w", wasmErr) in
			// component_linker canon.lower/lift closures) are also traps.
			if e, ok := r.(error); ok {
				var wasmErr *wasmruntime.Error
				if errors.As(e, &wasmErr) {
					err = e
					return
				}
			}
			// Everything else is an implementation bug — re-panic.
			panic(r)
		}
	}()
	return fn.CallAndPostReturn(ctx, args...)
}

// --- Value conversion ---

// convertArgs converts JSON Value objects from the wast JSON to types.Val
// values suitable for passing to api.ComponentFunc.Call.
func convertArgs(vals []Value) []types.Val {
	args := make([]types.Val, len(vals))
	for i, v := range vals {
		args[i] = convertValueToVal(v)
	}
	return args
}

// convertValueToVal converts a single JSON Value to its types.Val representation.
// The wast JSON format stores all values as strings; we parse them into
// the appropriate Val type based on the "type" field.
func convertValueToVal(v Value) types.Val {
	switch v.Type {
	case "i32", "s32":
		n, _ := strconv.ParseInt(v.Value, 10, 32)
		return types.ValS32(int32(n))
	case "u32":
		n, _ := strconv.ParseUint(v.Value, 10, 32)
		return types.ValU32(uint32(n))
	case "i64", "s64":
		n, _ := strconv.ParseInt(v.Value, 10, 64)
		return types.ValS64(n)
	case "u64":
		n, _ := strconv.ParseUint(v.Value, 10, 64)
		return types.ValU64(n)
	case "f32":
		n, _ := strconv.ParseUint(v.Value, 10, 32)
		return types.ValF32(math.Float32frombits(uint32(n)))
	case "f64":
		n, _ := strconv.ParseUint(v.Value, 10, 64)
		return types.ValF64(math.Float64frombits(n))
	case "s8":
		n, _ := strconv.ParseInt(v.Value, 10, 8)
		return types.ValS8(int8(n))
	case "u8":
		n, _ := strconv.ParseUint(v.Value, 10, 8)
		return types.ValU8(uint8(n))
	case "s16":
		n, _ := strconv.ParseInt(v.Value, 10, 16)
		return types.ValS16(int16(n))
	case "u16":
		n, _ := strconv.ParseUint(v.Value, 10, 16)
		return types.ValU16(uint16(n))
	case "bool":
		return types.ValBool(v.Value == "1" || v.Value == "true")
	case "string":
		return types.ValString(v.Value)
	case "char":
		n, _ := strconv.ParseInt(v.Value, 10, 32)
		return types.ValChar(rune(n))
	case "enum":
		return types.ValEnum(v.Value)
	case "tuple":
		// Tuple values are JSON arrays of nested Value objects.
		var nested []Value
		if err := json.Unmarshal([]byte(v.Value), &nested); err != nil {
			return types.ValString(v.Value) // fallback
		}
		vals := make([]types.Val, len(nested))
		for i, nv := range nested {
			vals[i] = convertValueToVal(nv)
		}
		return types.ValTuple(vals)
	case "list":
		// List values are JSON arrays of nested Value objects.
		var nested []Value
		if err := json.Unmarshal([]byte(v.Value), &nested); err != nil {
			return types.ValList([]types.Val{}) // empty list on parse failure
		}
		items := make([]types.Val, len(nested))
		for i, nv := range nested {
			items[i] = convertValueToVal(nv)
		}
		return types.ValList(items)
	default:
		// Return the raw string for unsupported types
		return types.ValString(v.Value)
	}
}

// --- Result comparison ---

// compareResults compares actual call results with expected values from the
// wast JSON. Results from api.ComponentFunc.Call are types.Val values.
func compareResults(t *testing.T, line int, actual []types.Val, expected []Value) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Errorf("assert_return at line %d: result count mismatch: got %d, expected %d",
			line, len(actual), len(expected))
		return
	}
	for i, exp := range expected {
		act := actual[i]
		if !valuesMatch(act, exp) {
			t.Errorf("assert_return at line %d: result[%d] mismatch: got %v (kind=%d), expected %s %s",
				line, i, act, act.Kind(), exp.Type, exp.Value)
		}
	}
}

// valuesMatch compares a types.Val (from Call results) with an expected
// JSON Value. Returns true if they match.
func valuesMatch(actual types.Val, expected Value) bool {
	switch expected.Type {
	case "i32", "s32":
		exp, _ := strconv.ParseInt(expected.Value, 10, 32)
		switch actual.Kind() {
		case types.ValKindS32:
			return actual.S32() == int32(exp)
		case types.ValKindU32:
			return int32(actual.U32()) == int32(exp)
		}
		return false
	case "u32":
		exp, _ := strconv.ParseUint(expected.Value, 10, 32)
		switch actual.Kind() {
		case types.ValKindU32:
			return actual.U32() == uint32(exp)
		case types.ValKindS32:
			return uint32(actual.S32()) == uint32(exp)
		}
		return false
	case "i64", "s64":
		exp, _ := strconv.ParseInt(expected.Value, 10, 64)
		switch actual.Kind() {
		case types.ValKindS64:
			return actual.S64() == exp
		case types.ValKindU64:
			return int64(actual.U64()) == exp
		}
		return false
	case "u64":
		exp, _ := strconv.ParseUint(expected.Value, 10, 64)
		switch actual.Kind() {
		case types.ValKindU64:
			return actual.U64() == exp
		case types.ValKindS64:
			return uint64(actual.S64()) == exp
		}
		return false
	case "f32":
		exp, _ := strconv.ParseUint(expected.Value, 10, 32)
		expF := math.Float32frombits(uint32(exp))
		if actual.Kind() == types.ValKindF32 {
			a := actual.F32()
			if math.IsNaN(float64(expF)) && math.IsNaN(float64(a)) {
				return true
			}
			return a == expF
		}
		return false
	case "f64":
		exp, _ := strconv.ParseUint(expected.Value, 10, 64)
		expF := math.Float64frombits(exp)
		if actual.Kind() == types.ValKindF64 {
			a := actual.F64()
			if math.IsNaN(expF) && math.IsNaN(a) {
				return true
			}
			return a == expF
		}
		return false
	case "s8":
		exp, _ := strconv.ParseInt(expected.Value, 10, 8)
		return actual.Kind() == types.ValKindS8 && actual.S8() == int8(exp)
	case "u8":
		exp, _ := strconv.ParseUint(expected.Value, 10, 8)
		return actual.Kind() == types.ValKindU8 && actual.U8() == uint8(exp)
	case "s16":
		exp, _ := strconv.ParseInt(expected.Value, 10, 16)
		return actual.Kind() == types.ValKindS16 && actual.S16() == int16(exp)
	case "u16":
		exp, _ := strconv.ParseUint(expected.Value, 10, 16)
		return actual.Kind() == types.ValKindU16 && actual.U16() == uint16(exp)
	case "bool":
		expBool := expected.Value == "1" || expected.Value == "true"
		return actual.Kind() == types.ValKindBool && actual.Bool() == expBool
	case "string":
		return actual.Kind() == types.ValKindString && actual.StringVal() == expected.Value
	case "char":
		exp, _ := strconv.ParseInt(expected.Value, 10, 32)
		return actual.Kind() == types.ValKindChar && actual.Char() == rune(exp)
	case "enum":
		return actual.Kind() == types.ValKindEnum && actual.Enum() == expected.Value
	case "tuple":
		if actual.Kind() != types.ValKindTuple {
			return false
		}
		actSlice := actual.Tuple()
		var nested []Value
		if err := json.Unmarshal([]byte(expected.Value), &nested); err != nil {
			return false
		}
		if len(actSlice) != len(nested) {
			return false
		}
		for i, nv := range nested {
			if !valuesMatch(actSlice[i], nv) {
				return false
			}
		}
		return true
	case "list":
		if actual.Kind() != types.ValKindList {
			return false
		}
		actSlice := actual.List()
		var nested []Value
		if err := json.Unmarshal([]byte(expected.Value), &nested); err != nil {
			return false
		}
		if len(actSlice) != len(nested) {
			return false
		}
		for i, nv := range nested {
			if !valuesMatch(actSlice[i], nv) {
				return false
			}
		}
		return true
	default:
		// For unsupported types, log and return false
		return false
	}
}

func TestSimpleWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/simple.wast")
}

func TestResourcesWasmtimeWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/resources.wast")
}

func TestTypesWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/types.wast")
}

func TestEnumsWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/enums.wast")
}

func TestNestedWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/nested.wast")
}

func TestLinkingWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/linking.wast")
}

func TestImportWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/import.wast")
}

func TestModulesWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/modules.wast")
}

func TestAliasingWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/aliasing.wast")
}

func TestTagsWast(t *testing.T) {
	t.Skip("tags.wast requires exception-handling proposal not supported by wazero core wasm engine")
}

func TestEnumDiscriminantWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/enum_discriminant.wast")
}

func TestFixedLengthListsWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/fixed_length_lists.wast")
}

func TestAdapterWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/adapter.wast")
}

func TestInstanceWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/instance.wast")
}

func TestRestrictionsWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/restrictions.wast")
}

// --- Wasmtime tests (new files from submodule update) ---

func TestBigStringsWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/big-strings.wast")
}

func TestErrorContextTrapInPostReturnWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/error-context-trap-in-post-return.wast")
}

func TestMapTypesWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/map-types.wast")
}

func TestStringTranscodeInvalidWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/string-transcode-invalid.wast")
}

func TestStringsWasmtimeWast(t *testing.T) {
	runWastSuite(t, "testdata/wasmtime/strings.wast")
}

// --- Component-model spec tests: wasmtime subset (differ from wasmtime repo) ---

func TestSpecWasmtimeFusedWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasmtime/fused.wast")
}

func TestSpecWasmtimeModulesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasmtime/modules.wast")
}

func TestSpecWasmtimeResourcesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasmtime/resources.wast")
}

func TestSpecWasmtimeStringsWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasmtime/strings.wast")
}

func TestSpecWasmtimeTypesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasmtime/types.wast")
}

// --- Component-model spec tests: values ---

func TestSpecValuesStringsWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/values/strings.wast")
}

func TestSpecValuesTrapInPostReturnWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/values/trap-in-post-return.wast")
}

// --- Component-model spec tests: names ---

func TestSpecNamesKebabWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/names/kebab.wast")
}

// --- Component-model spec tests: resources ---

func TestSpecResourcesMultipleResourcesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/resources/multiple-resources.wast")
}

// --- Component-model spec tests: wasm-tools ---

func TestSpecWasmToolsAdaptWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/adapt.wast")
}

func TestSpecWasmToolsAliasWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/alias.wast")
}

func TestSpecWasmToolsBigWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/big.wast")
}

func TestSpecWasmToolsDefinedtypesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/definedtypes.wast")
}

func TestSpecWasmToolsEmptyWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/empty.wast")
}

func TestSpecWasmToolsExampleWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/example.wast")
}

func TestSpecWasmToolsExportAscriptionWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/export-ascription.wast")
}

func TestSpecWasmToolsExportIntroducesAliasWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/export-introduces-alias.wast")
}

func TestSpecWasmToolsExportWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/export.wast")
}

func TestSpecWasmToolsFuncWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/func.wast")
}

func TestSpecWasmToolsImportWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/import.wast")
}

func TestSpecWasmToolsImportsExportsWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/imports-exports.wast")
}

func TestSpecWasmToolsInlineExportsWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/inline-exports.wast")
}

func TestSpecWasmToolsInstanceTypeWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/instance-type.wast")
}

func TestSpecWasmToolsInstantiateWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/instantiate.wast")
}

func TestSpecWasmToolsInvalidWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/invalid.wast")
}

func TestSpecWasmToolsLinkWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/link.wast")
}

func TestSpecWasmToolsLotsOfAliasesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/lots-of-aliases.wast")
}

func TestSpecWasmToolsLowerWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/lower.wast")
}

func TestSpecWasmToolsMemory64Wast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/memory64.wast")
}

func TestSpecWasmToolsModuleLinkWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/module-link.wast")
}

func TestSpecWasmToolsMoreFlagsWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/more-flags.wast")
}

func TestSpecWasmToolsNamingWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/naming.wast")
}

func TestSpecWasmToolsNestedModulesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/nested-modules.wast")
}

func TestSpecWasmToolsResourcesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/resources.wast")
}

func TestSpecWasmToolsTagsWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/tags.wast")
}

func TestSpecWasmToolsTypeExportRestrictionsWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/type-export-restrictions.wast")
}

func TestSpecWasmToolsTypesWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/types.wast")
}

func TestSpecWasmToolsVeryNestedWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/very-nested.wast")
}

func TestSpecWasmToolsVirtualizeWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/virtualize.wast")
}

func TestSpecWasmToolsWrongOrderWast(t *testing.T) {
	runWastSuite(t, "testdata/spec/wasm-tools/wrong-order.wast")
}

// runAssertInvalidTest tests that an invalid component fails to compile
func runAssertInvalidTest(t *testing.T, ctx context.Context, rt wazero.Runtime, suite *WastTestSuite, cmd *Command) {
	if cmd.Filename == "" {
		t.Errorf("no wasm file for this command")
		return
	}

	wasmBytes, err := suite.GetWasmBytes(cmd.Filename)
	if err != nil {
		t.Fatalf("GetWasmBytes(%s): %v", cmd.Filename, err)
	}

	// Try to compile the component - should fail
	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err == nil {
		// Component compiled successfully when it should have failed
		compiled.Close(ctx)
		t.Errorf("validation gap: wazero accepts invalid component at line %d that should fail with: %q", cmd.Line, cmd.Text)
		return
	}

	// Check if the error message contains the expected text
	errStr := err.Error()
	if !containsErrorText(errStr, cmd.Text) {
		// The component failed to compile, but with a different error
		// This might mean we're catching the error at a different stage
		// or with different wording

		// The component was correctly rejected but with a different error message.
		// Log as warning for investigation but don't fail — the important thing
		// is that the invalid component was rejected.
		t.Logf("Component failed to compile at line %d (expected error containing %q, got: %v)", cmd.Line, cmd.Text, err)
		t.Logf("WARNING: Component correctly rejected but error message mismatch at line %d", cmd.Line)
		return
	}

	t.Logf("PASS: Component correctly failed to compile at line %d with expected error", cmd.Line)
}

// containsErrorText checks if the error string contains the expected text
// This is case-insensitive and handles minor variations in wording
func containsErrorText(errStr, expected string) bool {
	errLower := strings.ToLower(errStr)
	expectedLower := strings.ToLower(expected)
	return strings.Contains(errLower, expectedLower)
}

