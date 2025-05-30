function getRequestData(cfg, previousResponse, position) {
    // Create a Request object using the provided constructor
    let request = new Request()
    
    // Make sure URL is valid by ensuring it has a protocol
    let urlStr = cfg["URL"]
    if (!urlStr.startsWith('http://') && !urlStr.startsWith('https://')) {
        urlStr = 'http://' + urlStr
    }
    
    let url = new URL(urlStr)
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
