function getRequestData(cfg, stuff, position) {
    let data = new RequestData()
    let url = new URL(cfg.URL)
    if (stuff["nextPageToken"] != undefined) {
        url.searchParams.set("pageToken", stuff["nextPageToken"])
    } else {
        var positionStr = String.fromCharCode.apply(String, position);
        url.searchParams.set("syncToken", positionStr.split("_people")[0])
    }

    url.searchParams.set("pageSize", 2)
    url.searchParams.set("personFields", "names")
    url.searchParams.set("requestSyncToken", true)

    data.URL = url.toString()

    return data
}
