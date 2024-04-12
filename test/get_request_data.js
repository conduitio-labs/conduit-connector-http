function getRequestData(cfg, previousResponse, position) {
    let data = new RequestData()
    let url = new URL(cfg.URL)
    if (previousResponse["nextPageToken"] != undefined) {
        url.searchParams.set("pageToken", previousResponse["nextPageToken"])
    } else {
        var positionStr = String.fromCharCode.apply(String, position);
        url.searchParams.set("syncToken", positionStr)
    }

    url.searchParams.set("pageSize", 2)

    data.URL = url.toString()

    return data
}