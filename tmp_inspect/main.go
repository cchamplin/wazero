package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wasm/binary"
)

func printModule(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m, err := binary.DecodeModule(data, api.CoreFeaturesV2, wasm.MemoryLimitPages, false, false, false)
	if err != nil {
		return err
	}
	fmt.Printf("== %s ==\n", path)
	fmt.Printf("Functions: %d (imported %d)\n", len(m.FunctionSection), m.ImportFunctionCount)
	fmt.Printf("Imports: %d\n", len(m.ImportSection))
	for _, imp := range m.ImportSection {
		fmt.Printf("  %s.%s kind=%d\n", imp.Module, imp.Name, imp.Type)
	}
	fmt.Printf("Tables: %d\n", len(m.TableSection))
	for i, t := range m.TableSection {
		fmt.Printf("  table %d: min=%d max=%v type=%d\n", i, t.Min, t.Max, t.Type)
	}
	fmt.Printf("Element segments: %d\n", len(m.ElementSection))
	for i, e := range m.ElementSection {
		fmt.Printf("  elem %d: mode=%d table=%d type=%d init_len=%d\n", i, e.Mode, e.TableIndex, e.Type, len(e.Init))
		switch e.OffsetExpr.Opcode {
		case wasm.OpcodeI32Const:
			v, _, _ := leb128.LoadInt32(e.OffsetExpr.Data)
			fmt.Printf("    offset const %d\n", v)
		case 0:
			fmt.Printf("    offset <none>\n")
		default:
			fmt.Printf("    offset opcode=%d data=%v\n", e.OffsetExpr.Opcode, e.OffsetExpr.Data)
		}
		min, max := uint32(^uint32(0)), uint32(0)
		for _, init := range e.Init {
			if init == wasm.ElementInitNullReference {
				continue
			}
			if init < min {
				min = init
			}
			if init > max {
				max = init
			}
		}
		if len(e.Init) > 0 {
			fmt.Printf("    init index range: %d..%d\n", min, max)
		}
	}
	if strings.Contains(path, "unbundled-module0.wasm") {
		idx := wasm.Index(1252)
		def := m.FunctionDefinition(idx)
		fmt.Printf("Debug func %d: %s exports=%v\n", idx, def.DebugName(), def.ExportNames())
		dumpCallIndirects(m, uint32(idx))
	}
	return nil
}

func dumpCallIndirects(m *wasm.Module, funcIdx uint32) {
	if funcIdx < m.ImportFunctionCount {
		fmt.Printf("function %d is imported\n", funcIdx)
		return
	}
	codeIdx := funcIdx - m.ImportFunctionCount
	if int(codeIdx) >= len(m.CodeSection) {
		fmt.Printf("function %d code missing\n", funcIdx)
		return
	}
	body := m.CodeSection[codeIdx].Body
	fmt.Printf("  func %d body len=%d\n", funcIdx, len(body))
	count := 0
	for i := 0; i < len(body); i++ {
		if body[i] == byte(wasm.OpcodeCallIndirect) {
			typeIndex, n1, _ := leb128.LoadUint32(body[i+1:])
			tableIndex, _, _ := leb128.LoadUint32(body[i+1+int(n1):])
			fmt.Printf("  call_indirect at byte %d type=%d table=%d\n", i, typeIndex, tableIndex)
			count++
		}
	}
	if count == 0 {
		fmt.Println("  no call_indirect found")
	}
}

func main() {
	paths := []string{
		"internal/component/wasip2test/go-plugin/core.wasm",
		"internal/component/wasip2test/go-plugin/core-with-wit.wasm",
		"/tmp/wasm-unbundle/multi/unbundled-module0.wasm",
		"/tmp/wasm-unbundle/multi/unbundled-module1.wasm",
		"/tmp/wasm-unbundle/multi/component-unbundled.wasm",
	}
	for _, p := range paths {
		if err := printModule(p); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
}
