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

| name            | description                                                                         | required | default value |
|-----------------|-------------------------------------------------------------------------------------|----------|---------------|
| `url`           | Http URL to send requests to.                                                       | true     |               |
| `method`        | Http method to use in the request, supported methods are (`GET`,`HEAD`,`OPTIONS`).  | false    | `GET`         |
| `headers`       | Http headers to use in the request, comma separated list of : separated pairs.      | false    |               |
| `params`        | parameters to use in the request, comma separated list of : separated pairs.        | false    |               |
| `pollingperiod` | how often the connector will get data from the url, formatted as a `time.Duration`. | false    | "5m"          |

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
| `method`  | Http method to use in the request, supported methods are (`POST`,`PUT`,`DELETE`,`PATCH`). | false      | `GET`         |
| `headers` | Http headers to use in the request, comma separated list of : separated pairs.            | false      |               |
| `params`  | parameters to use in the request, comma separated list of : separated pairs.              | false      |               |

