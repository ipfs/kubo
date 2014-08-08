#IPFS Protocol Specification

The purpose of this document is to lay out the structure of all messages sent by ipfs.
This includes the structure and types for all data sent over the network.

##DHT Messages
All DHT Messages are wrapped in a protobuf message structure as defined [over here](https://github.com/jbenet/go-ipfs/blob/dht/routing/dht/messages.proto) (currently only in dht branch).
Every message includes a unique message ID. This is used to identify responses to requests that are sent out, a response will have the same message ID as the request.

The fields are commonly overloaded to mean different things depending on which message type is being sent.

### Put
Type Value: 0

Request:

Key and Value are a common Key Value pair for insertion into the DHT.


Response:

Put has no response message

### Get
Type Value: 1

Key is the key being requested.


Response:

On success:

Value is the value for the requested key

On Failure:

Peers will contain a list of peers closer to the requested key


### Add Provider
Type Value: 2

Key represents the value that the sender is announcing that they can provide

Response:

Add provider has no response message

### Get Providers
Type Value: 3

Key is the value that the sender is looking for providers of.

Response:

Peers is an array of Peer IDs and Multiaddrs (See messages.proto)

### Find Peer
Type Value: 4

Key is the peer ID of the peer the sender is searching for

Response:

Peers will contain the closest peer the receiver has to the requested peer.
The Success variable will indicate if the peer was found or not.

### Ping
Type Value: 5

No Extra fields used.
