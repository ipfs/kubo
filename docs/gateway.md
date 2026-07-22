# Gateway

An IPFS Gateway acts as a bridge between traditional web browsers and IPFS.
Through the gateway, users can browse files and websites stored in IPFS as if
they were stored in a traditional web server.

[More about Gateways](https://docs.ipfs.tech/concepts/ipfs-gateway/) and [addressing IPFS on the web](https://docs.ipfs.tech/how-to/address-ipfs-on-web/).

Kubo's Gateway implementation follows [IPFS Gateway Specifications](https://specs.ipfs.tech/http-gateways/) and is tested with [Gateway Conformance Test Suite](https://github.com/ipfs/gateway-conformance).

## Table of contents

- [Local gateway](#local-gateway)
- [Public gateways](#public-gateways)
- [Gateway recipes](#gateway-recipes)
  - [Serve only your own content (`Gateway.NoFetch=true`)](#serve-only-your-own-content-gatewaynofetchtrue)
  - [Serve any content from the network (`Gateway.NoFetch=false`)](#serve-any-content-from-the-network-gatewaynofetchfalse)
  - [URL-style recipes](#url-style-recipes)
    - [Subdomain gateway](#subdomain-gateway)
    - [Path gateway](#path-gateway)
    - [DNSLink gateway](#dnslink-gateway)
    - [Hardened DNSLink gateway](#hardened-dnslink-gateway)
- [Configuration](#configuration)
- [Running in Production](#running-in-production)
- [Directories](#directories)
- [Static Websites](#static-websites)
- [Filenames](#filenames)
- [Downloads](#downloads)
- [Response Format](#response-format)
- [Content-Types](#content-types)

## Local gateway

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

## Public gateways

For public gateways available as a public good, see the
[public utilities](https://docs.ipfs.tech/concepts/public-utilities/) page; for a
broader community-maintained list, see the
[public gateway checker](https://ipfs.github.io/public-gateway-checker/).

Treat public gateways as a convenience for casual use; they are provided on a
best-effort basis. For anything you depend on, run your own gateway
([recipes](#gateway-recipes) below) or retrieve content over IPFS directly
instead of through someone else's gateway. The guide
[Replace public gateways with self-hosted IPFS](https://docs.ipfs.tech/how-to/replace-public-gateways-with-self-hosted-ipfs/)
covers the options.

## Gateway recipes

Before you pick a URL style (subdomain, path, or DNSLink, in the
[URL-style recipes](#url-style-recipes) below), decide the bigger question:
when a visitor asks for content this node does not have, should the gateway go
and fetch it from the network, or just answer "not found"?

> [!IMPORTANT]
> See [Reverse Proxy Caveats](#reverse-proxy) if running behind nginx or another reverse proxy.

> [!CAUTION]
> An open public gateway fetches and serves any content a visitor asks for,
> even content you have never seen. That means a stranger can pull illegal or
> abusive material through your server and your internet connection, and you
> are the one who has to handle the complaints and takedown requests. Unless
> you are running a shared public service on purpose, it is safer to
> [serve only your own content](#serve-only-your-own-content-gatewaynofetchtrue)
> with `Gateway.NoFetch=true`. If you do run an
> [open public gateway](#serve-any-content-from-the-network-gatewaynofetchfalse),
> first set up a way to block and take down bad content, using
> [content blocking](content-blocking.md).

### Serve only your own content (`Gateway.NoFetch=true`)

The node only shares content it already has. It never downloads missing content
from other peers, so anything this node does not have returns `404 Not Found`.

This is the safest way to put your own content (a website, a dataset, some
files) on a public gateway, and the recommended setup for most people.

```console
$ ipfs config --json Gateway.NoFetch true
```

- The gateway only serves content already stored on this node: anything you
  added with `ipfs add`, pinned, or put in MFS. Anything else returns
  `404 Not Found` right away, without touching the network.
- Because the node never fetches anything for a visitor, no stranger can make it
  download content you did not choose, so there is little room for abuse.
- Pin the content you serve (`ipfs pin add`) so garbage collection does not
  remove it. Unpinned content can be deleted when garbage collection runs, after
  which it stops resolving and returns `404`.
- Pick how URLs map to your content with one of the
  [URL-style recipes](#url-style-recipes) below.
- A gateway serves content over HTTP, so it does not need to announce that
  content to the IPFS network (so other peers can discover it here) for the
  gateway to work. If the content is already announced by another node (for
  example, you also pinned it elsewhere), or you want this node to spend its
  resources on serving instead of announcing, turn announcements off with
  [`Provide.Enabled=false`](config.md#provideenabled). That drops the ongoing
  announcing work.
- `NoFetch` does not stop your node from helping other peers find *their*
  content. If you want it to act only as a client and not help route for others,
  set [`Routing.Type=autoclient`](config.md#routingtype).
- To limit this to a single DNSLink website, see the
  [hardened DNSLink recipe](#hardened-dnslink-gateway) below.

### Serve any content from the network (`Gateway.NoFetch=false`)

This is the default, and how a large shared public gateway works. The gateway
looks up and downloads any content a visitor asks for, fetching it from the
network. Only run it this way if you mean to offer that kind of public service,
and set it up carefully first.

- **Block and take down abuse.** Sooner or later, someone will ask an open
  gateway to serve illegal or abusive content. Set up
  [content blocking](content-blocking.md) so you can respond to takedown
  requests: Kubo blocks content listed in
  [IPIP-383](https://specs.ipfs.tech/ipips/ipip-0383/) denylists and returns
  `410 Gone` for it. Blocking stops your node from serving the content, but
  [not from helping other peers find it](content-blocking.md#scope-of-denylists)
  on the network; to stop that too, set
  [`Routing.Type=autoclient`](content-blocking.md#how-to-stop-facilitating-routing-for-blocked-content).
- **Do not become free web hosting.** Set
  [`Gateway.DeserializedResponses=false`](config.md#gatewaydeserializedresponses) so the
  gateway returns only verifiable data, not ready-to-view web pages. People can
  no longer use it to host random websites, while apps that verify data
  themselves (like [@helia/verified-fetch](https://www.npmjs.com/package/@helia/verified-fetch))
  keep working.
- **Protect your bandwidth.** Review
  [`Gateway.MaxRangeRequestFileSize`](config.md#gatewaymaxrangerequestfilesize),
  [`Gateway.MaxConcurrentRequests`](config.md#gatewaymaxconcurrentrequests), and the
  [Running in Production](#running-in-production) notes on timeouts,
  reverse proxy, and CDN behavior.
- **Speed.** Consider [`Routing.AcceleratedDHTClient=true`](config.md#routingaccelerateddhtclient)
  for faster lookups. Decide whether this gateway should also announce the
  content it fetches so others can find it: if yes, turn on
  [`Provide.DHT.SweepEnabled=true`](config.md#providedhtsweepenabled) (and raise
  [`Provide.DHT.MaxWorkers`](config.md#providedhtmaxworkers) if announcements are slow);
  if no, set [`Provide.Enabled=false`](config.md#provideenabled).
- Pick how URLs map to content with one of the
  [URL-style recipes](#url-style-recipes) below.

### URL-style recipes

These decide how a request URL maps to content. They work the same whether or
not `Gateway.NoFetch` is set.

#### Subdomain gateway

A [subdomain gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#subdomain-gateway)
serves each content root from its own subdomain
(`http://{cid}.ipfs.subdomain-gw.example.com`), so every root gets its own Origin.

```console
$ ipfs config --json Gateway.PublicGateways '{
    "subdomain-gw.example.com": {
      "UseSubdomains": true,
      "Paths": ["/ipfs", "/ipns"]
    }
  }'
```

- **Backward-compatible:** content paths redirect to subdomains:

   `http://subdomain-gw.example.com/ipfs/{cid}` → `http://{cid}.ipfs.subdomain-gw.example.com`

- **X-Forwarded-Proto:** if you run Kubo behind a reverse proxy that provides TLS, make it add an `X-Forwarded-Proto: https` header so users are redirected to `https://`, not `http://`. It also inlines DNSLink names into a single DNS label, so they work with a wildcard TLS cert ([details](https://github.com/ipfs/in-web-browsers/issues/169)). The NGINX directive is `proxy_set_header X-Forwarded-Proto "https";`:

   `http://subdomain-gw.example.com/ipfs/{cid}` → `https://{cid}.ipfs.subdomain-gw.example.com`

   `http://subdomain-gw.example.com/ipns/your-dnslink.example.org` → `https://your--dnslink-example-org.ipns.subdomain-gw.example.com`

- **X-Forwarded-Host:** override the gateway host from the request with `X-Forwarded-Host: example.net`:

   `http://subdomain-gw.example.com/ipfs/{cid}` → `http://{cid}.ipfs.example.net`

#### Path gateway

A [path gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#path-gateway)
serves content under a path (`http://path-gw.example.com/ipfs/{cid}`), with no
Origin separation between content roots.

```console
$ ipfs config --json Gateway.PublicGateways '{
    "path-gw.example.com": {
      "UseSubdomains": false,
      "Paths": ["/ipfs", "/ipns"]
    }
  }'
```

#### DNSLink gateway

A [DNSLink gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#dnslink-gateway)
resolves the DNSLink name in the `Host` header of each request. It is on by
default (`NoDNSLink: false`):

```console
ipfs config --json Gateway.NoDNSLink false
```

#### Hardened DNSLink gateway

To serve a single DNSLink site and nothing else, combine `NoFetch` and
`NoDNSLink`. Disable fetching remote data (`NoFetch: true`) and DNSLink at
unknown hostnames (`NoDNSLink: true`), then enable DNSLink for one hostname
whose data is already on the node, without exposing any content-addressing
`Paths`:

```console
$ ipfs config --json Gateway.NoFetch true
$ ipfs config --json Gateway.NoDNSLink true
$ ipfs config --json Gateway.PublicGateways '{
    "dnslink-site.example.com": {
      "NoDNSLink": false,
      "Paths": []
    }
  }'
```

## Configuration

The `Gateway.*` configuration options are described in the
[config documentation](config.md#gateway). See [Gateway recipes](#gateway-recipes)
above for common setups.

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

1. If the path does not end in a `/`, append a `/` and redirect. This applies to
   any directory request and helps avoid serving duplicate content from
   different paths.<sup>&dagger;</sup>
2. If the directory contains an `index.html` file, serve it.
3. Otherwise, dynamically build and serve a listing of the directory contents.

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

> http://127.0.0.1:8080/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG?filename=hello_world.txt

When you try to save the page above, your browser will use the passed `filename` instead of a CID.

## Downloads

It is possible to skip browser rendering of supported filetypes (plain text,
images, audio, video, PDF) and trigger immediate "save as" dialog by appending
`&download=true`:

> http://127.0.0.1:8080/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG?filename=hello_world.txt&download=true

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

This is the equivalent of `ipfs block get`.

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
