/*
package handshake implements the ipfs handshake protocol

IPFS Handshake

The IPFS Protocol Handshake is divided into three sequential steps

1. Version Handshake (`Hanshake1`)

2. Secure Channel (`NewSecureConn`)

3. Services (`Handshake3`)

Currently these parts currently happen sequentially (costing an awful 5 RTT),
but can be optimized to 2 RTT.

Version Handshake

The Version Handshake ensures that nodes speaking to each other can interoperate.
They send each other protocol versions and ensure there is a match on the major
version (semver).

Secure Channel

The second part exchanges keys and establishes a secure comm channel. This
follows ECDHE TLS, but *isn't* TLS. (why will be written up elsewhere).

Services

The Services portion sends any additional information on nodes needed
by the nodes, e.g. Listen Address (the received address could be a Dial addr),
and later on can include Service listing (dht, exchange, ipns, etc).
*/
package handshake
