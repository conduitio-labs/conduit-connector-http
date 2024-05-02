function getRequestData(cfg, previousResponse, position) {
    let request = new Request()
    let url = new URL(cfg["url"])
    if (previousResponse["nextPageToken"] != undefined) {
        url.searchParams.set("pageToken", previousResponse["nextPageToken"])
    } else {
        var positionStr = String.fromCharCode.apply(String, position);
        url.searchParams.set("syncToken", positionStr)
    }

    url.searchParams.set("pageSize", 2)

    request.URL = url.toString()

    return request
}
