# Conduit Connector for HTTP
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

<!-- Configuration table -->
<table>
  <thead>
    <tr>
      <th>name</th>
      <th>description</th>
      <th>required</th>
      <th>default value</th>
      <th>example</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>url</code></td>
      <td>HTTP URL to send requests to.</td>
      <td>true</td>
      <td></td>
      <td>https://example.com/api/v1</td>
    </tr>
    <tr>
      <td><code>method</code></td>
      <td>HTTP method to use in the request, supported methods are (<code>GET</code>,<code>HEAD</code>,<code>OPTIONS</code>).</td>
      <td>false</td>
      <td><code>GET</code></td>
      <td><code>POST</code></td>
    </tr>
    <tr>
      <td><code>headers</code></td>
      <td>HTTP headers to use in the request, comma separated list of <code>:</code> separated pairs.</td>
      <td>false</td>
      <td></td>
      <td><code>Authorization:Bearer TOKEN_VALUE,Content-Type:application/xml</code></td>
    </tr>
    <tr>
      <td><code>params.*</code></td>
      <td>parameters to use in the request, use params.* as the config key and specify its value, ex: set "params.id" as "1".</td>
      <td>false</td>
      <td></td>
      <td><code>params.query="foobar"</code></td>
    </tr>
    <tr>
      <td><code>pollingperiod</code></td>
      <td>how often the connector will get data from the url, formatted as a <code>time.Duration</code>.</td>
      <td>false</td>
      <td><code>"5m"</code></td>
      <td><code>"5m"</code></td>
    </tr>
    <tr>
      <td><code>script.parseResponse</code></td>
      <td>
        <p>The path to a .js file containing the code to parse the response.</p>
        <p>The signature of the function needs to be:</p>
        <pre><code>function parseResponse(bytes)
        </code></pre> <br/>
        <p>where <code>bytes</code> is the original response's raw bytes (i.e. unparsed).</p>
        <p>The function needs to return a <code>Response</code> object.</p>
      </td>
      <td>false</td>
      <td></td>
      <td><code>/path/to/get_request_data.js</code> <br/><br/>
An example script can be found in <code>test/get_request_data.js</code></td>
    </tr>
    <tr>
      <td><code>script.getRequestData</code></td>
      <td>
        <p>The path to a .js file containing the code to prepare the request data.</p>
        <p>The signature of the function needs to be:</p>
        <pre><code>function getRequestData(cfg, previousResponse, position)
        </code></pre>
        <p>where:</p>
        <ul>
        <li><code>cfg</code> (a map) is the connector configuration</li>
        <li><code>previousResponse</code> (a map) contains data from the previous response (if any), returned by <code>parseResponse</code></li>
        <li><code>position</code> (a byte array) contains the starting position of the connector.</li>
        </ul>
        <p>The function needs to return a <code>Request</code> object.</p>
      </td>
      <td>false</td>
      <td></td>
      <td><code>/path/to/parse_response.js</code> <br/><br/>
An example script can be found in <code>test/parse_response.js</code>
      </td>
    </tr>
  </tbody>
</table>

<!-- End of configuration table -->

## Destination
The HTTP destination connector pushes data from upstream resources to an HTTP URL via Conduit. the destination adds the
`params` and `headers` to the request, and sends it to the URL with the specified `method` from the `Configuration`. 

Note: The request `Body` that will be sent is the value under `record.Payload.After`, if you want to change the format
of that or manipulate the field in any way, please check our [Builtin Processors Docs](https://conduit.io/docs/processors/builtin/)
, or check [Standalone Processors Docs](https://conduit.io/docs/processors/standalone/) if you'd like to build your own processor .

### Configuration

| name       | description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | required   | default value |
|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|---------------|
| `url`      | Is a Go template expression for the URL used in the HTTP request, using Go [templates](https://pkg.go.dev/text/template). The value provided to the template is [opencdc.Record](https://github.com/ConduitIO/conduit-connector-sdk/blob/bfc1d83eb75460564fde8cb4f8f96318f30bd1b4/record.go#L81), so the template has access to all its fields (e.g. .Position, .Key, .Metadata, and so on). We also inject all template functions provided by [sprig](https://masterminds.github.io/sprig/) to make it easier to write templates. | true       |               |
| `method`   | Http method to use in the request, supported methods are (`POST`,`PUT`,`DELETE`,`PATCH`).                                                                                                                                                                                                                                                                                                                                                                                                                                      | false      | `POST`        |
| `headers`  | Http headers to use in the request, comma separated list of : separated pairs.                                                                                                                                                                                                                                                                                                                                                                                                                                                 | false      |               |
| `params.*` | parameters to use in the request, use params.* as the config key and specify its value, ex: set "params.id" as "1".                                                                                                                                                                                                                                                                                                                                                                                                            | false      |               |

