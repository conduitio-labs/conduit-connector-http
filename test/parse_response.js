function convertToRecord(obj, nextSyncToken) {
    let rec = new Record()
    rec.Position = nextSyncToken
    rec.Payload.After = new StructuredData()
    Object.keys(obj).forEach(key => {
        if (key == "id") {
            rec.Key = new RawData(obj[key])
        } else if (key == "action") {
            rec.Operation = obj[key]
        } else {
            rec.Payload.After[key] = obj[key]
        }
    })

    return rec
}

function parseResponse(bytes) {
    logger.Info("[parseResponse] start")

    var str = String.fromCharCode.apply(String, bytes);
    var data = JSON.parse(str);

    const records = [];

    if (data.some_objects != undefined) {
        for (const obj of data.some_objects) {
            records.push(convertToRecord(obj, data.nextSyncToken));
        }
    }

    var resp = new Response()
    resp.CustomData["nextPageToken"] = data["nextPageToken"]
    resp.Records = records

    return resp
}
