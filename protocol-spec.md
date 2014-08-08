#IPFS Protocol Specification

The purpose of this document is to lay out the structure of all messages sent by ipfs.
This includes the structure and types for all data sent over the network.

##DHT Messages
All DHT Messages are wrapped in a protobuf message structure as defined in routing/dht/messages.proto (currently only in dht branch)
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

Value is the multiaddr of a node who is closer to the requested value

### Add Provider
Type Value: 2

Key represents the value that the sender is announcing that they can provide

Response:

Add provider has no response message

### Get Providers
Type Value: 3

Key is the value that the sender is looking for providers of.

Response:

Value is a JSON encoded map of peer ID''s to multiaddr strings of peers that can provide the requested Value

### Find Peer
Type Value: 4

Key is the peer ID of the peer the sender is searching for

Response:

The closest peer the receiver has to the requested peer (could potentially be the searched for peer)

### Ping
Type Value: 5

No Extra fields used.
