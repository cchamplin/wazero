use anyhow::{Context, Result};
use wasmtime::component::{Component, Linker, Val};
use wasmtime::{Config, Engine, Store};
use wasmtime_wasi::{WasiCtx, WasiCtxBuilder, WasiView};

/// Host state that includes WASI context
struct HostState {
    ctx: WasiCtx,
    table: wasmtime::component::ResourceTable,
}

impl WasiView for HostState {
    fn ctx(&mut self) -> &mut WasiCtx {
        &mut self.ctx
    }
    fn table(&mut self) -> &mut wasmtime::component::ResourceTable {
        &mut self.table
    }
}

/// Plugin test configuration
struct PluginTest {
    name: &'static str,
    plugin_name: &'static str,
    file: &'static str,
    expected_28_3: i32,
    expected_29_3: i32,
}

fn main() -> Result<()> {
    println!("Calculator Plugin Host Tests");
    println!("============================\n");

    let plugins = [
        PluginTest {
            name: "add",
            plugin_name: "add",
            file: "../plugins/add.wasm",
            expected_28_3: 31,  // 28 + 3
            expected_29_3: 32,  // 29 + 3
        },
        PluginTest {
            name: "subtract",
            plugin_name: "subtract",
            file: "../plugins/subtract.wasm",
            expected_28_3: 25,  // 28 - 3
            expected_29_3: 26,  // 29 - 3
        },
        PluginTest {
            name: "multi",
            plugin_name: "Simple-Go-Multi",
            file: "../plugins/multi.wasm",
            expected_28_3: 84,  // 28 * 3
            expected_29_3: 87,  // 29 * 3
        },
        PluginTest {
            name: "div",
            plugin_name: "Simple-Go-Div",
            file: "../plugins/div.wasm",
            expected_28_3: 9,   // 28 / 3 (integer division)
            expected_29_3: 9,   // 29 / 3 (integer division)
        },
    ];

    let mut passed = 0;
    let mut failed = 0;

    for plugin in &plugins {
        print!("Testing plugin '{}' ({})... ", plugin.name, plugin.file);

        match test_plugin(plugin) {
            Ok(()) => {
                println!("✓ PASSED");
                passed += 1;
            }
            Err(e) => {
                println!("✗ FAILED");
                println!("  Error: {:?}", e);
                failed += 1;
            }
        }
    }

    println!("\n============================");
    println!("Results: {} passed, {} failed", passed, failed);

    if failed > 0 {
        std::process::exit(1);
    }

    Ok(())
}

fn test_plugin(plugin: &PluginTest) -> Result<()> {
    // Create engine with component model enabled
    let mut config = Config::new();
    config.wasm_component_model(true);
    let engine = Engine::new(&config)?;

    // Create WASI context
    let wasi_ctx = WasiCtxBuilder::new()
        .inherit_stdio()
        .build();

    let host_state = HostState {
        ctx: wasi_ctx,
        table: wasmtime::component::ResourceTable::new(),
    };

    let mut store = Store::new(&engine, host_state);

    // Create linker and add WASI
    let mut linker: Linker<HostState> = Linker::new(&engine);
    wasmtime_wasi::add_to_linker_sync(&mut linker)?;

    // Load and compile the component
    let component_bytes = std::fs::read(plugin.file)
        .with_context(|| format!("Failed to read {}", plugin.file))?;

    let component = Component::new(&engine, &component_bytes)
        .with_context(|| format!("Failed to compile {}", plugin.file))?;

    // Instantiate the component
    let instance = linker.instantiate(&mut store, &component)
        .with_context(|| format!("Failed to instantiate {}", plugin.file))?;

    // Test get-plugin-name
    let get_plugin_name = instance
        .get_func(&mut store, "get-plugin-name")
        .context("get-plugin-name function not found")?;

    let mut results = vec![Val::String("".into())];
    get_plugin_name.call(&mut store, &[], &mut results)?;
    get_plugin_name.post_return(&mut store)?;

    let name = match &results[0] {
        Val::String(s) => s.to_string(),
        _ => anyhow::bail!("Expected string result from get-plugin-name"),
    };

    if name != plugin.plugin_name {
        anyhow::bail!(
            "get-plugin-name returned '{}', expected '{}'",
            name,
            plugin.plugin_name
        );
    }

    // Test evaluate(28, 3)
    let evaluate = instance
        .get_func(&mut store, "evaluate")
        .context("evaluate function not found")?;

    let mut results = vec![Val::S32(0)];
    evaluate.call(&mut store, &[Val::S32(28), Val::S32(3)], &mut results)?;
    evaluate.post_return(&mut store)?;

    let result = match &results[0] {
        Val::S32(v) => *v,
        _ => anyhow::bail!("Expected s32 result from evaluate"),
    };

    if result != plugin.expected_28_3 {
        anyhow::bail!(
            "evaluate(28, 3) returned {}, expected {}",
            result,
            plugin.expected_28_3
        );
    }

    // Test evaluate(29, 3)
    let mut results = vec![Val::S32(0)];
    evaluate.call(&mut store, &[Val::S32(29), Val::S32(3)], &mut results)?;
    evaluate.post_return(&mut store)?;

    let result = match &results[0] {
        Val::S32(v) => *v,
        _ => anyhow::bail!("Expected s32 result from evaluate"),
    };

    if result != plugin.expected_29_3 {
        anyhow::bail!(
            "evaluate(29, 3) returned {}, expected {}",
            result,
            plugin.expected_29_3
        );
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn create_test_engine() -> Result<Engine> {
        let mut config = Config::new();
        config.wasm_component_model(true);
        Engine::new(&config).context("Failed to create engine")
    }

    fn create_store_and_linker(engine: &Engine) -> Result<(Store<HostState>, Linker<HostState>)> {
        let wasi_ctx = WasiCtxBuilder::new()
            .inherit_stdio()
            .build();

        let host_state = HostState {
            ctx: wasi_ctx,
            table: wasmtime::component::ResourceTable::new(),
        };

        let store = Store::new(engine, host_state);
        let mut linker: Linker<HostState> = Linker::new(engine);
        wasmtime_wasi::add_to_linker_sync(&mut linker)?;

        Ok((store, linker))
    }

    fn load_and_test_plugin(
        engine: &Engine,
        store: &mut Store<HostState>,
        linker: &Linker<HostState>,
        path: &str,
        expected_name: &str,
        x: i32,
        y: i32,
        expected_result: i32,
    ) -> Result<()> {
        let component_bytes = std::fs::read(path)
            .with_context(|| format!("Failed to read {}", path))?;

        let component = Component::new(engine, &component_bytes)
            .with_context(|| format!("Failed to compile {}", path))?;

        let instance = linker.instantiate(&mut *store, &component)
            .with_context(|| format!("Failed to instantiate {}", path))?;

        // Test get-plugin-name
        let get_plugin_name = instance
            .get_func(&mut *store, "get-plugin-name")
            .context("get-plugin-name function not found")?;

        let mut results = vec![Val::String("".into())];
        get_plugin_name.call(&mut *store, &[], &mut results)?;
        get_plugin_name.post_return(&mut *store)?;

        let name = match &results[0] {
            Val::String(s) => s.to_string(),
            _ => anyhow::bail!("Expected string result"),
        };

        assert_eq!(name, expected_name, "Plugin name mismatch");

        // Test evaluate
        let evaluate = instance
            .get_func(&mut *store, "evaluate")
            .context("evaluate function not found")?;

        let mut results = vec![Val::S32(0)];
        evaluate.call(&mut *store, &[Val::S32(x), Val::S32(y)], &mut results)?;
        evaluate.post_return(&mut *store)?;

        let result = match &results[0] {
            Val::S32(v) => *v,
            _ => anyhow::bail!("Expected s32 result"),
        };

        assert_eq!(result, expected_result, "evaluate({}, {}) mismatch", x, y);

        Ok(())
    }

    #[test]
    fn test_add_plugin() -> Result<()> {
        let engine = create_test_engine()?;
        let (mut store, linker) = create_store_and_linker(&engine)?;

        load_and_test_plugin(
            &engine,
            &mut store,
            &linker,
            "../plugins/add.wasm",
            "add",
            28,
            3,
            31,
        )
    }

    #[test]
    fn test_subtract_plugin() -> Result<()> {
        let engine = create_test_engine()?;
        let (mut store, linker) = create_store_and_linker(&engine)?;

        load_and_test_plugin(
            &engine,
            &mut store,
            &linker,
            "../plugins/subtract.wasm",
            "subtract",
            28,
            3,
            25,
        )
    }

    #[test]
    fn test_multi_plugin() -> Result<()> {
        let engine = create_test_engine()?;
        let (mut store, linker) = create_store_and_linker(&engine)?;

        load_and_test_plugin(
            &engine,
            &mut store,
            &linker,
            "../plugins/multi.wasm",
            "Simple-Go-Multi",
            28,
            3,
            84,
        )
    }

    #[test]
    fn test_div_plugin() -> Result<()> {
        let engine = create_test_engine()?;
        let (mut store, linker) = create_store_and_linker(&engine)?;

        load_and_test_plugin(
            &engine,
            &mut store,
            &linker,
            "../plugins/div.wasm",
            "Simple-Go-Div",
            28,
            3,
            9,
        )
    }

    #[test]
    fn test_all_plugins_sequential() -> Result<()> {
        let plugins = [
            ("../plugins/add.wasm", "add", 28, 3, 31),
            ("../plugins/subtract.wasm", "subtract", 28, 3, 25),
            ("../plugins/multi.wasm", "Simple-Go-Multi", 28, 3, 84),
            ("../plugins/div.wasm", "Simple-Go-Div", 28, 3, 9),
        ];

        for (path, name, x, y, expected) in plugins {
            let engine = create_test_engine()?;
            let (mut store, linker) = create_store_and_linker(&engine)?;

            load_and_test_plugin(&engine, &mut store, &linker, path, name, x, y, expected)
                .with_context(|| format!("Failed testing plugin: {}", path))?;
        }

        Ok(())
    }

    #[test]
    fn test_div_by_zero() -> Result<()> {
        let engine = create_test_engine()?;
        let (mut store, linker) = create_store_and_linker(&engine)?;

        let component_bytes = std::fs::read("../plugins/div.wasm")?;
        let component = Component::new(&engine, &component_bytes)?;
        let instance = linker.instantiate(&mut store, &component)?;

        let evaluate = instance
            .get_func(&mut store, "evaluate")
            .context("evaluate function not found")?;

        // Division by zero should return 0 (as implemented in the plugin)
        let mut results = vec![Val::S32(0)];
        evaluate.call(&mut store, &[Val::S32(100), Val::S32(0)], &mut results)?;
        evaluate.post_return(&mut store)?;

        let result = match &results[0] {
            Val::S32(v) => *v,
            _ => anyhow::bail!("Expected s32 result"),
        };

        assert_eq!(result, 0, "Division by zero should return 0");

        Ok(())
    }
}
