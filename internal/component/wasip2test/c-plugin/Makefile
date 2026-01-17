WASI_SDK ?= /opt/wasi-sdk
WIT_BINDGEN ?= wit-bindgen

all: subtract.wasm

# Generate C bindings from WIT
plugin.h plugin.c plugin_component_type.o:
	$(WIT_BINDGEN) c ../wit --out-dir .

# Compile to component (wasm32-wasip2 target produces component binary directly)
subtract.wasm: subtract.c plugin.c plugin.h plugin_component_type.o
	$(WASI_SDK)/bin/clang \
		--target=wasm32-wasip2 \
		-mexec-model=reactor \
		-o $@ subtract.c plugin.c plugin_component_type.o

clean:
	rm -f plugin.h plugin.c plugin_component_type.o *.wasm

.PHONY: all clean
