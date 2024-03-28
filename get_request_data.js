function getRequestData(cfg, stuff, position) {
    let data = new RequestData()
    let url = new URL("https://people.googleapis.com/v1/people/me/connections")
    if (stuff["nextPageToken"] != undefined) {
        url.searchParams.set("pageToken", stuff["nextPageToken"])
    } else {
        var positionStr = String.fromCharCode.apply(String, position);
        url.searchParams.set("syncToken", positionStr)
    }

    url.searchParams.set("pageSize", 10)
    url.searchParams.set("personFields", "names")
    url.searchParams.set("requestSyncToken", true)

    data.URL = url.toString()

    return data
}
