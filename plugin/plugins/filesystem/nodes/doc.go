/*
Package fsnodes provides constructors and interfaces, for composing various 9P
file systems implementations and wrappers.

The default RootIndex provided by RootAttacher, is a file system of itself
which relays request to various IPFS subsystems.
Attaching to and utilizing P9.File implementations themselves, in the same way a client program could use them independently.

The current mapping looks like this:
 - mountpoint - associated filesystem
 - /          - points back to itself, returning the root index
 - /ipfs      - PinFS
 - /ipfs/*    - IPFS
 - /ipns/*    - IPNS
*/
package fsnodes
