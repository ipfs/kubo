#Welcome to Bitswap
###(The data trading engine)

Bitswap is the module that is responsible for requesting and providing data
blocks over the network to and from other ipfs peers. The role of bitswap is
to be a merchant in the large global marketplace of data.

##Main Operations
Bitswap has three high level operations:

- **GetBlocks**
  - `GetBlocks` is a bitswap method used to request multiple blocks that are likely
to all be provided by the same set of peers (part of a single file, for example).

- **GetBlock**
  - `GetBlock` is a special case of `GetBlocks` that just requests a single block.

- **HasBlock**
  - `HasBlock` registers a local block with bitswap. Bitswap will then send that
block to any connected peers who want it (with the strategies approval), record
that transaction in the ledger and announce to the DHT that the block is being
provided.

##Internal Details
All `GetBlock` requests are relayed into a single for-select loop via channels.
Calls to `GetBlocks` will have `FindProviders` called for only the first key in
the set initially, This is an optimization attempting to cut down on the number
of RPCs required. After a timeout (specified by the strategies
`GetRebroadcastDelay`) Bitswap will iterate through all keys still in the local
wantlist, perform a find providers call for each, and sent the wantlist out to
those providers. This is the fallback behaviour for cases where our initial
assumption about one peer potentially having multiple blocks in a set does not
hold true.

When receiving messages, Bitswaps `ReceiveMessage` method is called. A bitswap
message may contain the wantlist of the peer who sent the message, and an array
of blocks that were on our local wantlist. Any blocks we receive in a bitswap
message will be passed to `HasBlock`, and the other peers wantlist gets updated
in the strategy by `bs.strategy.MessageReceived`.
If another peers wantlist is received, Bitswap will call its strategies
`ShouldSendBlockToPeer` method to determine whether or not the other peer will
be sent the block they are requesting (if we even have it).

##Outstanding TODOs:
- [ ] Ensure only one request active per key
- [ ] More involved strategies
- [ ] Ensure only wanted blocks are counted in ledgers
