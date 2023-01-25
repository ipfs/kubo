# IPFS Gateway

> IPFS Gateway HTTP handler.

## Documentation

* Go Documentation: https://pkg.go.dev/github.com/ipfs/kubo/core/corehttp/gateway

## Example

```go
// Initialize your headers and apply the default headers.
headers := map[string][]string{}
gateway.AddAccessControlHeaders(headers)

conf := gateway.Config{
  Writable: false,
  Headers:  headers,
}

// Initialize a NodeAPI interface for both an online and offline versions.
// The offline version should not make any network request for missing content.
ipfs := ...
offlineIPFS := ...

// Create http mux and setup gateway handler.
mux := http.NewServeMux()
gwHandler := gateway.NewHandler(conf, ipfs, offlineIPFS)
mux.Handle("/ipfs/", gwHandler)
mux.Handle("/ipns/", gwHandler)

// Start the server on :8080 and voilá! You have an IPFS gateway running
// in http://localhost:8080.
_ = http.ListenAndServe(":8080", mux)
```