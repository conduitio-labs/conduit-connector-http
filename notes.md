* cannot import conduit-commons
  ```
  panic: proto: file "opencdc/v1/opencdc.proto" is already registered
	previously from: "github.com/conduitio/conduit-commons/proto/opencdc/v1"
	currently from:  "github.com/conduitio/conduit-connector-protocol/proto/opencdc/v1"
  ```
* not possible to execute http api requests from within the JS code in goja
* panic in connector sdk shown only in debug logs