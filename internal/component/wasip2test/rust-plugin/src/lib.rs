wit_bindgen::generate!({
    world: "plugin",
    path: "../wit",
});

struct AddPlugin;

impl Guest for AddPlugin {
    fn get_plugin_name() -> String {
        "add".to_string()
    }

    fn evaluate(x: i32, y: i32) -> i32 {
        x + y
    }
}

export!(AddPlugin);
