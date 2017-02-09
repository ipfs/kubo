# go-ipfs OpenBazaar fork
[![GoDoc](https://godoc.org/github.com/ipfs/go-ipfs?status.svg)](https://godoc.org/github.com/ipfs/go-ipfs) [![Build Status](https://travis-ci.org/ipfs/go-ipfs.svg?branch=master)](https://travis-ci.org/ipfs/go-ipfs)

This is the official fork of IPFS used in OpenBazaar. It's comes bundled in the `vendor`
package of openbazaar-go so if you run openbazaar-go you are running this fork.

It is not safe to run the main IPFS codebase in the OpenBazaar network as your node will
not be able to communicate with other OpenBazaar nodes.

## Diff
This fork is currently based on IPFS v0.4.5-pre2 with the following changes:

- The `/ipfs/dht`, `/ipfs/bitswap/`, and `/ipfs/supernoderouting/` protocol strings have been changed to `/openbazaar/dht`, `/openbazaar/bitswap/`, and `/openbazaar/supernoderouting/` respectively. This keeps the OpenBazaar network from merging with the main IPFS network.
- Changed the TTL of providers which use a magic number for an ID from 24 hours to 7 days. A longer TTL on certain data is needed for OpenBazaar's messaging system.
- Accept providers whose peer ID does not match the ID of the sender. This, again, is needed for the messaging system.
- Change the `swarm` `peers` output to []string from a private struct. The access control on the struct made the return unusable otherwise.
- Resolve IPNS queries locally if the query is for our own peer ID. This will make browsing one's own OpenBazaar page much faster.
- Increase MaxRecordAge to 7 days to match the message TTL.
- Accept gateway IPNS queries using a blockchainID. Resolves names to a peer ID.
- Cache INPS gateway queries for 10 minutes.
- Change gateway PUT to accept a directory hash.
- Added optional cookie and basic authentication to gateway.
- Added persistent cache for IPNS. Used if network query fails.
- Remove private key check from config initialization as OpenBazaar doesn't store the private key in the config.
- Bundled go-libp2p-kad-dht so we can modify protocol strings without maintaining another fork.
