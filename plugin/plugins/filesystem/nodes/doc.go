/*
Package fsnodes provides constructors and interfaces, for composing various 9P
file systems implementations and wrappers of them.

The default RootIndex provided by RootAttacher, is a file system of itself
which wraps the other `p9.File` implementations, and relays request to them.

It does so by simply utilizing these subsystems in the same way a client program would if used independently.
Linking them together in a slash delimited hierarchy.

The current mapping looks like this:
Table key:
mountpoint - associated file system - purpose
 - /          - points back to itself, returning the root index - Maintains the hierarchy and dispatches requests
 - /ipfs      - PinFS - Exposes the node's pins as a series of files
 - /ipfs/*    - IPFS  - Relays requests to the IPFS namespace, translating UnixFS objects into 9P constructs
 - /ipns/     - KeyFS - Exposes the node's keys as a series of files
 - /ipns/*    - IPNS  - Another relay, but for the IPNS namespace
*/
package fsnodes
