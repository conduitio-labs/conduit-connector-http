# Conduit Connector for HTTP

<!-- readmegen:description -->
Conduit HTTP source and destination connectors, they connect to an HTTP URL and send HTTP requests.<!-- /readmegen:description -->

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

<!-- readmegen:source.parameters.yaml -->
```yaml
version: 2.2
pipelines:
  - id: example
    status: running
    connectors:
      - id: example
        plugin: "http"
        settings:
          # Http url to send requests to
          # Type: string
          # Required: yes
          url: ""
          # Http headers to use in the request, comma separated list of :
          # separated pairs
          # Type: string
          # Required: no
          headers: ""
          # HTTP method to use in the request
          # Type: string
          # Required: no
          method: "GET"
          # parameters to use in the request, use params.* as the config key and
          # specify its value, ex: set "params.id" as "1".
          # Type: string
          # Required: no
          params.*: ""
          # how often the connector will get data from the url
          # Type: duration
          # Required: no
          pollingPeriod: "5m"
          # The path to a .js file containing the code to prepare the request
          # data. The signature of the function needs to be: `function
          # getRequestData(cfg, previousResponse, position)` where: * `cfg` (a
          # map) is the connector configuration * `previousResponse` (a map)
          # contains data from the previous response (if any), returned by
          # `parseResponse` * `position` (a byte array) contains the starting
          # position of the connector. The function needs to return a Request
          # object.
          # Type: string
          # Required: no
          script.getRequestData: ""
          # The path to a .js file containing the code to parse the response.
          # The signature of the function needs to be: `function
          # parseResponse(bytes)` where `bytes` are the original response's raw
          # bytes (i.e. unparsed). The response should be a Response object.
          # Type: string
          # Required: no
          script.parseResponse: ""
          # Maximum delay before an incomplete batch is read from the source.
          # Type: duration
          # Required: no
          sdk.batch.delay: "0"
          # Maximum size of batch before it gets read from the source.
          # Type: int
          # Required: no
          sdk.batch.size: "0"
          # Specifies whether to use a schema context name. If set to false, no
          # schema context name will be used, and schemas will be saved with the
          # subject name specified in the connector (not safe because of name
          # conflicts).
          # Type: bool
          # Required: no
          sdk.schema.context.enabled: "true"
          # Schema context name to be used. Used as a prefix for all schema
          # subject names. If empty, defaults to the connector ID.
          # Type: string
          # Required: no
          sdk.schema.context.name: ""
          # Whether to extract and encode the record key with a schema.
          # Type: bool
          # Required: no
          sdk.schema.extract.key.enabled: "true"
          # The subject of the key schema. If the record metadata contains the
          # field "opencdc.collection" it is prepended to the subject name and
          # separated with a dot.
          # Type: string
          # Required: no
          sdk.schema.extract.key.subject: "key"
          # Whether to extract and encode the record payload with a schema.
          # Type: bool
          # Required: no
          sdk.schema.extract.payload.enabled: "true"
          # The subject of the payload schema. If the record metadata contains
          # the field "opencdc.collection" it is prepended to the subject name
          # and separated with a dot.
          # Type: string
          # Required: no
          sdk.schema.extract.payload.subject: "payload"
          # The type of the payload schema.
          # Type: string
          # Required: no
          sdk.schema.extract.type: "avro"
```
<!-- /readmegen:source.parameters.yaml -->

## Destination

The HTTP destination connector pushes data from upstream resources to an HTTP URL via Conduit. the destination adds the
`params` and `headers` to the request, and sends it to the URL with the specified `method` from the `Configuration`. 

Note: The request `Body` that will be sent is the value under `record.Payload.After`, if you want to change the format
of that or manipulate the field in any way, please check our [Builtin Processors Docs](https://conduit.io/docs/processors/builtin/)
, or check [Standalone Processors Docs](https://conduit.io/docs/processors/standalone/) if you'd like to build your own processor .

### Configuration

<!-- readmegen:destination.parameters.yaml -->
```yaml
version: 2.2
pipelines:
  - id: example
    status: running
    connectors:
      - id: example
        plugin: "http"
        settings:
          # URL is a Go template expression for the URL used in the HTTP
          # request, using Go [templates](https://pkg.go.dev/text/template). The
          # value provided to the template is
          # [opencdc.Record](https://conduit.io/docs/using/opencdc-record), so
          # the template has access to all its fields (e.g. .Position, .Key,
          # .Metadata, and so on). We also inject all template functions
          # provided by [sprig](https://masterminds.github.io/sprig/) to make it
          # easier to write templates.
          # Type: string
          # Required: yes
          url: ""
          # Http headers to use in the request, comma separated list of :
          # separated pairs
          # Type: string
          # Required: no
          headers: ""
          # HTTP method to use in the request
          # Type: string
          # Required: no
          method: "POST"
          # parameters to use in the request, use params.* as the config key and
          # specify its value, ex: set "params.id" as "1".
          # Type: string
          # Required: no
          params.*: ""
          # Maximum delay before an incomplete batch is written to the
          # destination.
          # Type: duration
          # Required: no
          sdk.batch.delay: "0"
          # Maximum size of batch before it gets written to the destination.
          # Type: int
          # Required: no
          sdk.batch.size: "0"
          # Allow bursts of at most X records (0 or less means that bursts are
          # not limited). Only takes effect if a rate limit per second is set.
          # Note that if `sdk.batch.size` is bigger than `sdk.rate.burst`, the
          # effective batch size will be equal to `sdk.rate.burst`.
          # Type: int
          # Required: no
          sdk.rate.burst: "0"
          # Maximum number of records written per second (0 means no rate
          # limit).
          # Type: float
          # Required: no
          sdk.rate.perSecond: "0"
          # The format of the output record. See the Conduit documentation for a
          # full list of supported formats
          # (https://conduit.io/docs/using/connectors/configuration-parameters/output-format).
          # Type: string
          # Required: no
          sdk.record.format: "opencdc/json"
          # Options to configure the chosen output record format. Options are
          # normally key=value pairs separated with comma (e.g.
          # opt1=val2,opt2=val2), except for the `template` record format, where
          # options are a Go template.
          # Type: string
          # Required: no
          sdk.record.format.options: ""
          # Whether to extract and decode the record key with a schema.
          # Type: bool
          # Required: no
          sdk.schema.extract.key.enabled: "true"
          # Whether to extract and decode the record payload with a schema.
          # Type: bool
          # Required: no
          sdk.schema.extract.payload.enabled: "true"
```
<!-- /readmegen:destination.parameters.yaml -->