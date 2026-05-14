# Gateway

An IPFS Gateway acts as a bridge between traditional web browsers and IPFS.
Through the gateway, users can browse files and websites stored in IPFS as if
they were stored in a traditional web server. 

[More about Gateways](https://docs.ipfs.tech/concepts/ipfs-gateway/) and [addressing IPFS on the web](https://docs.ipfs.tech/how-to/address-ipfs-on-web/).

Kubo's Gateway implementation follows [IPFS Gateway Specifications](https://specs.ipfs.tech/http-gateways/) and is tested with [Gateway Conformance Test Suite](https://github.com/ipfs/gateway-conformance).

### Local gateway

By default, Kubo nodes run
a [path gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#path-gateway) at `http://127.0.0.1:8080/`
and a [subdomain gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://localhost:8080/`.

> [!CAUTION]
> **For browsing websites, web apps, and dapps in a browser, use the subdomain
> gateway** (`localhost`). Each content root gets its own
> [web origin](https://developer.mozilla.org/en-US/docs/Web/Security/Same-origin_policy),
> isolating localStorage, cookies, and session data between sites.
>
> **For file retrieval, use the path gateway** (`127.0.0.1`). Path gateways are
> suited for downloading files or fetching [verifiable](https://docs.ipfs.tech/reference/http/gateway/#trustless-verifiable-retrieval)
> content, but lack origin isolation (all content shares the same origin).

Additional listening addresses and gateway behaviors can be set in the [config](#configuration) file.

### Public gateways

IPFS Foundation [provides public gateways](https://docs.ipfs.tech/concepts/public-utilities/) at
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

## Running in Production

When deploying Kubo's gateway in production, be aware of these important considerations:

<a id="reverse-proxy"></a>
> [!IMPORTANT]
> **Reverse Proxy:** When running Kubo behind a reverse proxy (such as nginx),
> the original `Host` header **must** be forwarded to Kubo for
> [`Gateway.PublicGateways`](config.md#gatewaypublicgateways) to work.
> Kubo uses the `Host` header to match configured hostnames and detect
> subdomain gateway patterns like `{cid}.ipfs.example.org` or DNSLink hostnames.
>
> If the `Host` header is not forwarded correctly, Kubo will not recognize
> the configured gateway hostnames and requests may be handled incorrectly.
>
> If `X-Forwarded-Proto` is not set, redirects over HTTPS will use wrong protocol
> and DNSLink names will not be inlined for subdomain gateways.
>
> Example: minimal nginx configuration for `example.org`
>
> ```nginx
> server {
>     listen 80;
>     listen [::]:80;
>
>     # IMPORTANT: Include wildcard to match subdomain gateway requests.
>     # The dot prefix matches both apex domain and all subdomains.
>     server_name .example.org;
>
>     location / {
>         proxy_pass http://127.0.0.1:8080;
>
>         # IMPORTANT: Forward the original Host header to Kubo.
>         # Without this, PublicGateways configuration will not work.
>         proxy_set_header Host $host;
>
>         # IMPORTANT: X-Forwarded-Proto is required for correct behavior:
>         # - Redirects will use https:// URLs when set to "https"
>         # - DNSLink names will be inlined for subdomain gateways
>         #   (e.g., /ipns/en.wikipedia-on-ipfs.org → en-wikipedia--on--ipfs-org.ipns.example.org)
>         proxy_set_header X-Forwarded-Proto $scheme;
>         proxy_set_header X-Forwarded-Host  $host;
>     }
> }
> ```
>
> Common mistakes to avoid:
>
> - **Missing wildcard in `server_name`:** Using only `server_name example.org;`
>   will not match subdomain requests like `{cid}.ipfs.example.org`. Always
>   include `*.example.org` or use the dot prefix `.example.org`.
>
> - **Wrong `Host` header value:** Using `proxy_set_header Host $proxy_host;`
>   sends the backend's hostname (e.g., `127.0.0.1:8080`) instead of the
>   original `Host` header. Always use `$host` or `$http_host`.
>
> - **Missing `Host` header entirely:** If `proxy_set_header Host` is not
>   specified, nginx defaults to `$proxy_host`, which breaks gateway routing.

> [!IMPORTANT]
> **Timeouts:** Configure [`Gateway.RetrievalTimeout`](config.md#gatewayretrievaltimeout)
> to terminate stalled transfers (resets on each data write, catches unresponsive operations),
> and [`Gateway.MaxRequestDuration`](config.md#gatewaymaxrequestduration) as a fallback
> deadline (default: 1 hour, catches cases when other timeouts are misconfigured or fail to fire).

> [!IMPORTANT]
> **Rate Limiting:** Use [`Gateway.MaxConcurrentRequests`](config.md#gatewaymaxconcurrentrequests)
> to protect against traffic spikes.

> [!IMPORTANT]
> **CDN/Cloudflare:** If using Cloudflare or other CDNs with
> [deserialized responses](config.md#gatewaydeserializedresponses) enabled, review
> [`Gateway.MaxRangeRequestFileSize`](config.md#gatewaymaxrangerequestfilesize) to avoid
> excess bandwidth billing from range request bugs. Cloudflare users may need additional
> protection via [Cloudflare Snippets](https://github.com/ipfs/boxo/issues/856#issuecomment-3523944976).

## Directories

For convenience, the gateway (mostly) acts like a normal web-server when serving
a directory:

1. If the directory contains an `index.html` file:
  1. If the path does not end in a `/`, append a `/` and redirect. This helps
     avoid serving duplicate content from different paths.<sup>&dagger;</sup>
  2. Otherwise, serve the `index.html` file.
2. Dynamically build and serve a listing of the contents of the directory.

<sub><sup>&dagger;</sup>This redirect is skipped if the query string contains a
`go-get=1` parameter. See [PR#3963](https://github.com/ipfs/kubo/pull/3963)
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

Returns a [CAR](https://ipld.io/specs/transport/car/) stream for a DAG or a subset of it.

The `dag-scope` parameter controls which blocks are included: `all` (default, entire DAG),
`entity` (logical unit like a file), or `block` (single block). For [UnixFS](https://specs.ipfs.tech/unixfs/) files,
`entity-bytes` enables byte range requests. See [IPIP-402](https://specs.ipfs.tech/ipips/ipip-0402/)
for details.

This is a rough equivalent of `ipfs dag export`.

### `application/vnd.ipfs.ipns-record`

Only works on `/ipns/{ipns-name}` content paths that use cryptographically signed [IPNS Records](https://specs.ipfs.tech/ipns/ipns-record/).

Returns [IPNS Record in Protobuf Serialization Format](https://specs.ipfs.tech/ipns/ipns-record/#record-serialization-format)
which can be verified on end client, without trusting gateway.
