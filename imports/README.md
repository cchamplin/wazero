## wazero imports

Packages in this directory implement the *host* imports needed for specific
languages or shared compiler toolchains.

### Core Module Imports

* [AssemblyScript](assemblyscript) e.g. `asc X.ts --debug -b none -o X.wasm`
* [Emscripten](emscripten) e.g. `em++ ... -s STANDALONE_WASM -o X.wasm X.cc`
* [WASI Preview 1](wasi_snapshot_preview1) e.g. `tinygo build -o X.wasm -target=wasi X.go`

### Component Model Imports

* [WASI Preview 2](wasip2) - Full WASI P2 interface support for WebAssembly
  Components, including:
  * **CLI** (`wasi:cli`) - environment variables, arguments, stdin/stdout/stderr
  * **Clocks** (`wasi:clocks`) - wall clock and monotonic clock
  * **Random** (`wasi:random`) - cryptographic and insecure random
  * **Filesystem** (`wasi:filesystem`) - file and directory operations with preopens
  * **Sockets** (`wasi:sockets`) - TCP and UDP networking with DNS resolution
  * **HTTP** (`wasi:http`) - outgoing HTTP requests and incoming handler support
  * **I/O** (`wasi:io`) - streams and pollable I/O

  Use `wasip2.MergeInto(linker)` to register all WASI P2 interfaces onto a
  component linker. See the [component-wasip2 example](../examples/component-wasip2/)
  for a complete working demonstration.

Note: You may not see a language listed here because it either works without
host imports, or it uses WASI. Refer to https://wazero.io/languages/ for more.

Please [open an issue](https://github.com/tetratelabs/wazero/issues/new) if you
would like to see support for another compiled language or toolchain.

## Overview

WebAssembly has a virtual machine architecture where the *host* is the process
embedding wazero and the *guest* is a program compiled into the WebAssembly
Binary Format, also known as Wasm (`%.wasm`).

The only features that work by default are computational in nature, and the
only way to communicate is via functions, memory or global variables.

When a compiler targets Wasm, it often needs to import functions from the host
to satisfy system calls needed for functionality like printing to the console,
getting the time, or generating random values. The technical term for this
bridge is Application Binary Interface (ABI), but we'll call them simply host
imports.

### Core Modules

Packages in this directory are sometimes well re-used, such as the case in
[WASI](https://wazero.io/specs/#wasi). For example, Rust, TinyGo, and Zig can
all target WebAssembly in a way that imports the same "wasi_snapshot_preview1"
module in the compiled `%.wasm` file. To support any of these, wazero users can
invoke `wasi_snapshot_preview1.Instantiate` on their `wazero.Runtime`.

### Components

The [Component Model](https://component-model.bytecodealliance.org/) introduces
a higher-level composition system with typed interfaces defined in
[WIT](https://component-model.bytecodealliance.org/design/wit.html). Components
compiled by toolchains such as `cargo component` (Rust), `wit-bindgen-go` +
TinyGo (Go), or `wasm-tools` (C) typically import WASI Preview 2 interfaces.

To support these components, use the `wasip2` package:

```go
linker := rt.NewComponentLinker()
linker.SetRelaxedSemverMatching(true) // required for WASI 0.2.x interfaces
wasip2.MergeInto(linker)             // registers all WASI P2 interfaces
instance, _ := linker.Instantiate(ctx, compiled)
```

See the [`api/component`](../api/component/) package for the dynamic value
types needed when defining custom host functions.
