/*
Package fsnodes provides constructors and interfaces, for composing various 9P
file systems implementations and wrappers.

The default RootIndex provided by RootAttacher, is a file system of itself
which relays request to various IPFS subsystems.
Utilizing the subsystem implementations itself, in the same way a client program would.
*/
package fsnodes
