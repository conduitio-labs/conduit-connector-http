function connectionToRecord(connection) {
    let rec = new Record()
    rec.Payload.After = new RawData(JSON.stringify(connection))
    rec.Key = new RawData(connection.resourceName)
    return rec;
}

function parseResponse(bytes) {
    logger.Info().Msg("[parseResponse] start")

    var str = String.fromCharCode.apply(String, bytes);
    var data = JSON.parse(str);

    logger.Info().Msg("[parseResponse] nextSyncToken: " + data.nextSyncToken)

    const records = [];

    if (data.connections != undefined) {
        for (const conn of data.connections) {
            let rec = connectionToRecord(conn)
            rec.Position = data.nextSyncToken
            records.push(rec);
        }
    }

    logger.Info().Msg("[parseResponse] parsed records")

    var resp = new ResponseData()
    resp.Stuff["nextPageToken"] = data["nextPageToken"]
    resp.Records = records

    return resp
}
