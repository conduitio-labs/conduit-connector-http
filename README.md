# Conduit Connector for <resource>
The HTTP connector is a [Conduit](https://github.com/ConduitIO/conduit) plugin. It provides both, a source
and a destination HTTP connectors.

## How to build?
Run `make build` to build the connector's binary.

## Testing
Run `make test` to run all the unit tests. 

## Source
The HTTP source connector pulls data from the HTTP URL every `pollingPeriod`, the source adds the `params` and `headers`
to the request, and sends it to the URL with the specified `method` from the `Configuration`. The returned data is
used to create an openCDC record and return it.

Note: when using the `OPTIONS` method, the resulted options will be added to the record's metadata.

### Configuration

- `name`:
    - description: HTTP URL to send requests to.
    - required: true
    - default value:

- `method`:
    - description: HTTP method to use in the request, supported methods are (`GET`,`HEAD`,`OPTIONS`).
    - required: false
    - default value: `GET`

- `headers`:
    - description: HTTP headers to use in the request, comma separated list of `:` separated pairs.
    - required: false
    - default value:

- `params`:
    - description: parameters to use in the request, comma separated list of `:` separated pairs.
    - required: false
    - default value:

- `pollingperiod`:
    - description: how often the connector will get data from the url, formatted as a `time.Duration`.
    - required: false
    - default value: "5m"
- 
- `script.getRequestData`:
    - description: The path to a .js file containing the code to prepare the request data. The signature of the function needs to be: `function getRequestData(cfg, previousResponse, position)` where: * `cfg` (a map) is the connector configuration * `previousResponse` (a map) contains data from the previous response (if any), returned by `parseResponse` * `position` (a byte array) contains the starting position of the connector. The function needs to return a Request object.
    - required: false
    - default: ""

- `script.parseResponse`:
    - description: The path to a .js file containing the code to parse the response. The signature of the function needs to be: `function parseResponse(bytes)` where `bytes` are the original response's raw bytes (i.e. unparsed). The response should be a Response object.
    - required: false
    - default: ""

## Destination
The HTTP destination connector pushes data from upstream resources to an HTTP URL via Conduit. the destination adds the
`params` and `headers` to the request, and sends it to the URL with the specified `method` from the `Configuration`. 

Note: The request `Body` that will be sent is the value under `record.Payload.After`, if you want to change the format
of that or manipulate the field in any way, please check our [Builtin Processors Docs](https://conduit.io/docs/processors/builtin/)
, or check [Standalone Processors Docs](https://conduit.io/docs/processors/standalone/) if you'd like to build your own processor .

### Configuration

| name      | description                                                                               | required   | default value |
|-----------|-------------------------------------------------------------------------------------------|------------|---------------|
| `url`     | Http URL to send requests to.                                                             | true       |               |
| `method`  | Http method to use in the request, supported methods are (`POST`,`PUT`,`DELETE`,`PATCH`). | false      | `POST`        |
| `headers` | Http headers to use in the request, comma separated list of : separated pairs.            | false      |               |
| `params`  | parameters to use in the request, comma separated list of : separated pairs.              | false      |               |

