package main

import (
    "bytes"
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/imports/wasip2"
    "github.com/tetratelabs/wazero/internal/component"
)

func main() {
    ctx := context.Background()
    rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
    defer rt.Close(ctx)

    wasiLinker := component.NewLinker()
    if err := wasip2.Instantiate(wasiLinker); err != nil {
        panic(err)
    }

    wasmBytes, err := os.ReadFile(filepath.Join("internal", "component", "wasip2test", "plugins", "multi.wasm"))
    if err != nil {
        panic(err)
    }

    compiled, err := rt.CompileComponent(ctx, wasmBytes)
    if err != nil {
        panic(err)
    }
    defer compiled.Close(ctx)

    linker := component.NewComponentLinker(rt)
    linker.SetRelaxedSemverMatching(true)
    linker.MergeFrom(wasiLinker)

    var stdout, stderr bytes.Buffer
    wasiConfig := wasip2.NewConfig().WithStdout(&stdout).WithStderr(&stderr)
    resourceTable := component.NewResourceTable()
    testCtx := wasip2.WithConfig(ctx, wasiConfig)
    testCtx = component.WithResourceTable(testCtx, resourceTable)

    instance, err := linker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
    if err != nil {
        panic(err)
    }

    nameFunc := instance.ExportedFunction("get-plugin-name")
    if nameFunc == nil {
        panic("no get-plugin-name")
    }
    result, err := nameFunc.Call(testCtx)
    if err != nil {
        fmt.Printf("call error: %v\n", err)
        return
    }
    fmt.Printf("name=%s\n", result[0].StringVal())
}
