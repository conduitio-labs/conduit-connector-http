function getRequestData(cfg, previousResponse, position) {
    let request = new Request()
    request.URL = new URL("resource1", cfg["url"])

    return request
}