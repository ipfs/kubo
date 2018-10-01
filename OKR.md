# Quarterly Objectives and Key Results

We try to frame our ongoing work using a process based on quarterly Objectives and Key Results (OKRs). Objectives reflect outcomes that are challenging, but realistic. Results are tangible and measurable.

## 2018 Q4

**go-ipfs handles large datasets (1TB++) without a sweat**
- `P0` - It takes less than 48 hours to transfer 1TB dataset over Fast Ethernet (100Mbps)
- `P0` @hannahhoward - List a sharded directory with 1M entries over LAN in under 1 minute, with less than a second to the first entry.
- `PX` - There is a prototype implementation of GraphSync for go-ipfs
- `P0` @magik6k - There is a better and more performant datastore module (e.g Badger or better)
- `P1` - Rewrite pinning data structure to support large pinsets and flexible pin modes

**The bandwidth usage is reduced significantly and is well kept under control**
- `PX` - Spec and draft implementation of allowing users to opt out of providing every IPLD node (and only provide root hashes)
- `PX` - Bitswap improvements reduce number of duplicate blocks downloaded by 75%
- `PX` @stebalien - The number of messages sent by Bitswap is on average <= 2x the number of blocks received

**It is a joy to use go-ipfs programatically**
- `PX` @magik6k - The Core API is finalized and released. Make it easier to import go-ipfs as a package
- `PX` @magik6k - go-ipfs-api exposes the new Core API
- `PX` @magik6k - go-ipfs Daemon, Gateway, and cmds library use the new Core API
- `PX` @magik6k - The legacy non Core API is deprecated and the diagram on go-ipfs README is updated

**go-ipfs becomes a well maintained project**
- `P2` @eingenito - The Go Contributing Guidelines are updated to contemplate expecations from Core Devs and instructions on how to be an effective contributor (e.g include PR templates)
- `P1` @eingenito - A Lead Maintainer Protocol equivalent is proposed, reviewed by the team, merged and implemented
- `P0` @eingenito - Every issue on https://waffle.io/ipfs/go-ipfs gets triaged (reviewed and labeled following https://github.com/ipfs/pm/blob/master/GOLANG_CORE_DEV_MGMT.md)
- `P0` @eingenito - Every non-trivial PR is first reviewed by someone other than @Stebalien before he looks at it.
- `P2` - Every package has tests and tests+code coverage are running on Jenkins
- `P2` - There is an up-to-date Architecture Diagram of the Go implementation of IPFS that links packages to subsystems to workflows

**gx becomes a beloved tool by the Go Core Contributors**
- `P0` - You can update a minor version of a transitive dependancy without updating intermediate dependancies
- `P0` - go-ipfs doesn't have checked-in gx paths

**Complete outstanding endeavours and still high priorities from Q3**
- `P0` @kevina - base32 is supported and enabled by default
- `P1` - go-ipfs gets a unixfsV2 spec and prototype
- `P2` @djdv - Add mutable methods (r+w) to the new mount implementation and get it building+tested on all supported platforms

## 2018 Q3

Find the **go-ipfs OKRs** for 2018 Q3 at the [2018 Q3 IPFS OKRs Spreadsheet](https://docs.google.com/spreadsheets/d/19vjigg4locq4fO6JXyobS2yTx-k-fSzlFM5ngZDPDbQ/edit#gid=274358435)

## 2018 Q2

Find the **go-ipfs OKRs** for 2018 Q2 at the [2018 Q2 IPFS OKRs Spreadsheet](https://docs.google.com/spreadsheets/d/1xIhKROxFlsY9M9on37D5rkbSsm4YtjRQvG2unHScApA/edit#gid=274358435)

## 2018 Q1

Find the **go-ipfs OKRs** for 2018 Q1 at the [2018 Q1 IPFS OKRs Spreadsheet](https://docs.google.com/spreadsheets/u/1/d/1clB-W489rJpbOEs2Q7Q2Jf1WMXHQxXgccBcUJS9QTiI/edit#gid=2079514081)
