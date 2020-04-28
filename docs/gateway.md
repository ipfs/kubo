# Gateway

An IPFS Gateway acts as a bridge between traditional web browsers and IPFS.
Through the gateway, users can browse files and websites stored in IPFS as if
they were stored in a traditional web server.

By default, go-ipfs nodes run a gateway at `http://127.0.0.1:8080/`.

We also provide a public gateway at `https://ipfs.io`. If you've ever seen a
link in the form `https://ipfs.io/ipfs/Qm...`, that's being served from *our*
gateway.

## Configuration

The gateway's configuration options are (briefly) described in the
[config](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#gateway)
documentation.

## Directories

For convenience, the gateway (mostly) acts like a normal web-server when serving
a directory:

1. If the directory contains an `index.html` file:
  1. If the path does not end in a `/`, append a `/` and redirect. This helps
     avoid serving duplicate content from different paths.<sup>&dagger;</sup>
  2. Otherwise, serve the `index.html` file.
2. Dynamically build and serve a listing of the contents of the directory.

<sub><sup>&dagger;</sup>This redirect is skipped if the query string contains a
`go-get=1` parameter. See [PR#3964](https://github.com/ipfs/go-ipfs/pull/3963)
for details</sub>

## Static Websites

You can use an IPFS gateway to serve static websites at a custom domain using
[DNSLink](https://dnslink.io). See [Example: IPFS
Gateway](https://dnslink.io/#example-ipfs-gateway) for instructions.

## Filenames

When downloading files, browsers will usually guess a file's filename by looking
at the last component of the path. Unfortunately, when linking *directly* to a
file (with no containing directory), the final component is just a CID
(`Qm...`). This isn't exactly user-friendly.

To work around this issue, you can add a `filename=some_filename` parameter to
your query string to explicitly specify the filename. For example:

> https://ipfs.io/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG?filename=hello_world.txt

## MIME-Types

TODO

## Read-Only API

For convenience, the gateway exposes a read-only API. This read-only API exposes
a read-only, "safe" subset of the normal API.

For example, you use this to download a block:

```
> curl https://ipfs.io/api/v0/block/get/bafkreifjjcie6lypi6ny7amxnfftagclbuxndqonfipmb64f2km2devei4
```
