function parseResponse(bytes) {
    // logger.Info().Msg("[parseResponse] start")

    var str = String.fromCharCode.apply(String, bytes);
    var data = JSON.parse(str);

    const records = [];

    if (data.some_objects != undefined) {
        for (const obj of data.some_objects) {
            let rec = new Record()
            rec.Position = data.nextSyncToken
            rec.Payload.After = new RawData(JSON.stringify(obj))
            records.push(rec);
        }
    }

    var resp = new ResponseData()
    resp.CustomData["nextPageToken"] = data["nextPageToken"]
    resp.Records = records

    return resp
}
