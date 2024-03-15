# Gateway

An IPFS Gateway acts as a bridge between traditional web browsers and IPFS.
Through the gateway, users can browse files and websites stored in IPFS as if
they were stored in a traditional web server. 

[More about Gateways](https://docs.ipfs.tech/concepts/ipfs-gateway/) and [addressing IPFS on the web](https://docs.ipfs.tech/how-to/address-ipfs-on-web/).

Kubo's Gateway implementation follows [ipfs/specs: Specification for HTTP Gateways](https://github.com/ipfs/specs/tree/main/http-gateways#readme).

### Local gateway

By default, Kubo nodes run
a [path gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#path-gateway) at `http://127.0.0.1:8080/`
and a [subdomain gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://localhost:8080/`.

The path one also implements [trustless gateway spec](https://specs.ipfs.tech/http-gateways/trustless-gateway/)
and supports [trustless responses](https://docs.ipfs.tech/reference/http/gateway/#trustless-verifiable-retrieval) as opt-in via `Accept` header.

Additional listening addresses and gateway behaviors can be set in the [config](#configuration) file.

### Public gateways

Protocol Labs provides a public gateway at
`https://ipfs.io` ([path](https://specs.ipfs.tech/http-gateways/path-gateway/)),
`https://dweb.link` ([subdomain](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#subdomain-gateway)),
and `https://trustless-gateway.link` ([trustless](https://specs.ipfs.tech/http-gateways/trustless-gateway/) only).
If you've ever seen a link in the form `https://ipfs.io/ipfs/Qm...`, that's being served from a *public goods* gateway.

There is a list of third-party public gateways provided by the IPFS community at https://ipfs.github.io/public-gateway-checker/

## Configuration

The `Gateway.*` configuration options are (briefly) described in the
[config](https://github.com/ipfs/kubo/blob/master/docs/config.md#gateway)
documentation, including a list of common [gateway recipes](https://github.com/ipfs/kubo/blob/master/docs/config.md#gateway-recipes).

### Debug
The gateway's log level can be changed with this command:
```
> ipfs log level core/server debug
```

## Directories

For convenience, the gateway (mostly) acts like a normal web-server when serving
a directory:

1. If the directory contains an `index.html` file:
  1. If the path does not end in a `/`, append a `/` and redirect. This helps
     avoid serving duplicate content from different paths.<sup>&dagger;</sup>
  2. Otherwise, serve the `index.html` file.
2. Dynamically build and serve a listing of the contents of the directory.

<sub><sup>&dagger;</sup>This redirect is skipped if the query string contains a
`go-get=1` parameter. See [PR#3964](https://github.com/ipfs/kubo/pull/3963)
for details</sub>

## Static Websites

You can use an IPFS gateway to serve static websites at a custom domain using
[DNSLink](https://docs.ipfs.tech/concepts/glossary/#dnslink). See [Example: IPFS
Gateway](https://dnslink.dev/#example-ipfs-gateway) for instructions.

## Filenames

When downloading files, browsers will usually guess a file's filename by looking
at the last component of the path. Unfortunately, when linking *directly* to a
file (with no containing directory), the final component is just a CID
(`bafy..` or `Qm...`). This isn't exactly user-friendly.

To work around this issue, you can add a `filename=some_filename` parameter to
your query string to explicitly specify the filename. For example:

> https://ipfs.io/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG?filename=hello_world.txt

When you try to save above page, you browser will use passed `filename` instead of a CID.

## Downloads

It is possible to skip browser rendering of supported filetypes (plain text,
images, audio, video, PDF) and trigger immediate "save as" dialog by appending
`&download=true`:

> https://ipfs.io/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG?filename=hello_world.txt&download=true

## Response Format

An explicit response format can be requested using `?format=raw|car|..` URL parameter,
or by sending `Accept: application/vnd.ipld.{format}` HTTP header with one of supported content types.

## Content-Types

Majority of resources can be retrieved trustlessly by requesting specific content type via `Accept` header or `?format=raw|car|ipns-record` URL query parameter.

See [trustless gateway specification](https://specs.ipfs.tech/http-gateways/trustless-gateway/)
and [verifiable retrieval documentation](https://docs.ipfs.tech/reference/http/gateway/#trustless-verifiable-retrieval) for more details.

### `application/vnd.ipld.raw`

Returns a byte array for a single `raw` block.

Sending such requests for `/ipfs/{cid}` allows for efficient fetch of blocks with data
encoded in custom format, without the need for deserialization and traversal on the gateway.

This is equivalent of `ipfs block get`.

### `application/vnd.ipld.car`

Returns a [CAR](https://ipld.io/specs/transport/car/) stream for specific DAG and selector.

Right now only 'full DAG' implicit selector is implemented.
Support for user-provided IPLD selectors is tracked in https://github.com/ipfs/kubo/issues/8769.

This is a rough equivalent of `ipfs dag export`.

### `application/vnd.ipfs.ipns-record`

Only works on `/ipns/{ipns-name}` content paths that use cryptographically signed [IPNS Records](https://specs.ipfs.tech/ipns/ipns-record/).

Returns [IPNS Record in Protobuf Serialization Format](https://specs.ipfs.tech/ipns/ipns-record/#record-serialization-format)
which can be verified on end client, without trusting gateway.
