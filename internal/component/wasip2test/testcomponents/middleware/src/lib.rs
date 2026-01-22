wit_bindgen::generate!({
    world: "middleware",
    path: "wit/service.wit",
});

use exports::example::service::handler::{Error, Guest, Request, Response};
use example::service::handler as downstream;
use example::service::logging::get_logger;
use example::service::types::{Request as TypesRequest, Response as TypesResponse};

struct Component;

impl Guest for Component {
    fn execute(req: Request) -> Result<Response, Error> {
        let logger = get_logger();
        logger.log("middleware: intercepting request");

        // Get the request data
        let headers = req.headers();
        let body = req.body();

        // Add a middleware header
        let mut new_headers = headers.clone();
        new_headers.push((b"x-middleware".to_vec(), b"processed".to_vec()));

        // Create a new request for the downstream service
        let downstream_req = TypesRequest::new(&new_headers, &body);

        // Call the downstream handler
        logger.log("middleware: forwarding to downstream service");
        let response = downstream::execute(downstream_req)?;

        // Get response data and add middleware header
        let mut response_headers = response.headers();
        response_headers.push((b"x-middleware-response".to_vec(), b"processed".to_vec()));
        let response_body = response.body();

        logger.log("middleware: returning response");
        Ok(TypesResponse::new(&response_headers, &response_body))
    }
}

export!(Component);
