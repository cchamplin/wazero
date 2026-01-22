wit_bindgen::generate!({
    world: "service",
    path: "wit/service.wit",
});

use exports::example::service::handler::{Error, Guest, Request, Response};
use example::service::logging::get_logger;
use example::service::types::Response as TypesResponse;

struct Component;

impl Guest for Component {
    fn execute(req: Request) -> Result<Response, Error> {
        let logger = get_logger();
        logger.log("service: handling request");

        let body = req.body();
        let body_str = String::from_utf8_lossy(&body);
        logger.log(&format!("service: received body: {}", body_str));

        // Create a simple echo response
        let response_body = format!("Service received: {}", body_str).into_bytes();
        let response_headers = vec![
            (b"content-type".to_vec(), b"text/plain".to_vec()),
        ];

        Ok(TypesResponse::new(&response_headers, &response_body))
    }
}

export!(Component);
