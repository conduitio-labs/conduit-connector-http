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
      <td><code>params</code></td>
      <td>parameters to use in the request, comma separated list of <code>:</code> separated pairs.</td>
      <td>false</td>
      <td></td>
      <td><code>"query:foobar,language:english</code></td>
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
      <td>The path to a .js file containing the code to parse the response.<br/> 
        The signature of the function needs to be: <br/>
        <code>function parseResponse(bytes)</code> <br/>
        where <code>bytes</code> is the original response's raw bytes (i.e. unparsed).<br/> 
        The response should be a <code>Response</code> object.</td>
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

| name      | description                                                                               | required   | default value |
|-----------|-------------------------------------------------------------------------------------------|------------|---------------|
| `url`     | HTTP URL to send requests to.                                                             | true       |               |
| `method`  | HTTP method to use in the request, supported methods are (`POST`,`PUT`,`DELETE`,`PATCH`). | false      | `POST`        |
| `headers` | HTTP headers to use in the request, comma separated list of : separated pairs.            | false      |               |
| `params`  | parameters to use in the request, comma separated list of : separated pairs.              | false      |               |

