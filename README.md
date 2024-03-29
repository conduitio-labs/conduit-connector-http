# Conduit Connector for <resource>
The HTTP connector is a [Conduit](https://github.com/ConduitIO/conduit) plugin. It provides both, a source
and a destination HTTP connectors.

## How to build?
Run `make build` to build the connector.

## Testing
Run `make test` to run all the unit tests. 

## Source
A source connector pulls data from an external resource and pushes it to downstream resources via Conduit.

### Configuration

| name            | description                                                                         | required | default value |
|-----------------|-------------------------------------------------------------------------------------|----------|---------------|
| `url`           | Http URL to send requests to.                                                       | true     |               |
| `method`        | Http method to use in the request, supported methods are (`GET`,`HEAD`,`OPTIONS`).  | false    | `GET`         |
| `headers`       | Http headers to use in the request, comma separated list of : separated pairs.      | false    |               |
| `params`        | parameters to use in the request, & separated list of = separated pairs.            | false    |               |
| `pollingperiod` | how often the connector will get data from the url, formatted as a `time.Duration`. | false    | "5m"          |

## Destination
A destination connector pushes data from upstream resources to an external resource via Conduit.

### Configuration

| name      | description                                                                               | required   | default value |
|-----------|-------------------------------------------------------------------------------------------|------------|---------------|
| `url`     | Http URL to send requests to.                                                             | true       |               |
| `method`  | Http method to use in the request, supported methods are (`POST`,`PUT`,`DELETE`,`PATCH`). | false      | `GET`         |
| `headers` | Http headers to use in the request, comma separated list of : separated pairs.            | false      |               |
| `params`  | parameters to use in the request, & separated list of = separated pairs.                  | false      |               |

