# Quarterly Objectives and Key Results

We try to frame our ongoing work using a process based on quarterly Objectives and Key Results (OKRs). Objectives reflect outcomes that are challenging, but realistic. Results are tangible and measurable.

## 2018 Q4

**go-ipfs handles large datasets (1TB++) without a sweat**
- `PX` - It takes less than 48 hours to transfer 1TB dataset over Fast Ethernet (100Mbps)
- `PX` - It takes less than 12 hours to transfer 200GB sharded dataset over Fast Ethernet (100Mbps)
- `PX` - There is a prototype implementation of GraphSync for go-ipfs
- `PX` - There is a better and more performant datastore module (e.g Badger or better)
- `P1` - Rewrite pinning data structures to support large data sets / many files performantly

**The bandwidth usage is reduced significantly and is well kept under control**
- `PX` - Users can opt out of providing every IPLD node (and only provide root hashes)
- `PX` - "Bitswap improvements around reducing chattiness, decreasing bandwidth usage (fewer dupe blocks), and increasing throughput"

**It is a joy to use go-ipfs programatically**
- `PX` @magik6k - The Core API is finalized and released. Make it easier to import go-ipfs as a package
- `PX` - go-ipfs-api exposes the new Core API
- `PX` - go-ipfs Daemon uses the new Core API
- `PX` - go-ipfs Gateway uses the new Core API
- `PX` - go-ipfs-cms uses uses the new Core API
- `PX` - The legacy non Core API is deprecated and the diagram on go-ipfs README is updated

**go-ipfs becomes a well maintained project**
- `PX` @eingenito - The Go Contributing Guidelines are updated to contemplate expecations from Core Devs and instructions on how to be an effective contributor (e.g include PR templates)
- `PX` @eingenito - A Lead Maintainer Protocol equivalent is proposed, reviewed by the team, merged and implemented
- `PX` @eingenito - Every issue on https://waffle.io/ipfs/go-ipfs gets triaged (reviewed and labeled following https://github.com/ipfs/pm/blob/master/GOLANG_CORE_DEV_MGMT.md)
- `PX` - Every package has tests and tests+code coverage are running on Jenkins
- `PX` - There is an up-to-date Architecture Diagram of the Go implementation of IPFS that links packages to subsystems to workflows

**gx becomes a beloved tool by the Go Core Contributors**
- `PX` - 
- `PX` - 

**Complete outstanding endeavours and still high priorities from Q3**
- `P0` @kevina - base32 is supported and enabled by default
- `PX` - go-ipfs gets a unixfsV2 prototype
- `PX` @djdv - IPFS Mount

## 2018 Q3

Find the **go-ipfs OKRs** for 2018 Q3 at the [2018 Q3 IPFS OKRs Spreadsheet](https://docs.google.com/spreadsheets/d/19vjigg4locq4fO6JXyobS2yTx-k-fSzlFM5ngZDPDbQ/edit#gid=274358435)

## 2018 Q2

Find the **go-ipfs OKRs** for 2018 Q2 at the [2018 Q2 IPFS OKRs Spreadsheet](https://docs.google.com/spreadsheets/d/1xIhKROxFlsY9M9on37D5rkbSsm4YtjRQvG2unHScApA/edit#gid=274358435)

## 2018 Q1

Find the **go-ipfs OKRs** for 2018 Q1 at the [2018 Q1 IPFS OKRs Spreadsheet](https://docs.google.com/spreadsheets/u/1/d/1clB-W489rJpbOEs2Q7Q2Jf1WMXHQxXgccBcUJS9QTiI/edit#gid=2079514081)
