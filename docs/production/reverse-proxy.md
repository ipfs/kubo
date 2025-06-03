# IPFS & Reverse HTTP Proxies

When run in production environments, go-ipfs should generally be run behind a
reverse HTTP proxy (usually NGINX). You may need a reverse proxy to:

* Load balance requests across multiple go-ipfs daemons.
* Cache responses.
* Buffer requests, only releasing them to go-ipfs when complete. This can help
  protect go-ipfs from the
  [slowloris](https://en.wikipedia.org/wiki/Slowloris_(computer_security)
  attack.
* Block content.
* Rate limit and timeout requests.
* Apply QoS rules (e.g., prioritize traffic for certain important IPFS resources).

This document contains a collection of tips, tricks, and pitfalls when running a
go-ipfs node behind a reverse HTTP proxy.

**WARNING:** Due to
[nginx#1293](https://trac.nginx.org/nginx/ticket/1293)/[go-ipfs#6402](https://github.com/ipfs/go-ipfs/issues/6402),
parts of the go-ipfs API will not work correctly behind an NGINX reverse proxy
as go-ipfs starts sending back a response before it finishes reading the request
body. The gateway itself is unaffected.

## Peering

Go-ipfs gateways behind a single load balancing reverse proxy should use the
[peering](../config.md#peering) subsystem to peer with each other. That way, as
long as one go-ipfs daemon has the content being requested, the others will be
able to serve it.

# Garbage Collection

Gateways rarely store content permanently. However, running garbage collection
can slow down a go-ipfs node significantly. If you've noticed this issue in
production, consider "garbage collecting" by resetting the go-ipfs repo whenever
you run out of space, instead of garbage collecting.

1. Initialize your gateways repo to some known-good state (possibly pre-seeding
   it with some content, a config, etc.).
2. When you start running low on space, for each load-balanced go-ipfs node:
    1. Use the nginx API to set one of the upstream go-ipfs node's to "down".
    2. Wait a minute to let go-ipfs finish processing any in-progress requests
      (or the short-lived ones, at least).
    3. Take the go-ipfs node down.
    4. Rollback the go-ipfs repo to the seed state.
    5. Restart the go-ipfs daemon.
    6. Update the nginx config, removing the "down" status from the node.

This will effectively "garbage collect" without actually running the garbage
collector.

# Content Blocking

TODO:

* Filtering requests
* Checking the X-IPFS-Path header in responses to filter again after resolving.

# Subdomain Gateway

TODO: Reverse proxies and the subdomain gateway.

# Load balancing

TODO: discuss load balancing based on the CID versus the source IP.
