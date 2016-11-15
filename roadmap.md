# go-ipfs - Roadmap

This document describes the current status and the upcoming milestones of the go-ipfs project.

*Updated: Tue, 15 Nov 2016 19:08:06 GMT*

## Status and Progress

[![Project Status](https://badge.waffle.io/ipfs/go-ipfs.svg?label=Backlog&title=Backlog)](http://waffle.io/ipfs/go-ipfs) [![Project Status](https://badge.waffle.io/ipfs/go-ipfs.svg?label=In%20Progress&title=In%20Progress)](http://waffle.io/ipfs/go-ipfs) [![Project Status](https://badge.waffle.io/ipfs/go-ipfs.svg?label=Done&title=Done)](http://waffle.io/ipfs/go-ipfs)

See details of current progress on [Orbit's project board](https://waffle.io/haadcode/orbit)

#### Milestone Summary

| Status | Milestone | Goals | ETA |
| :---: | :--- | :---: | :---: |
| 🚀 | **[ipld integration](#ipld-integration)** | 2 / 2 | Fri Oct 28 2016 |
| 🚀 | **[IPFS Core API](#ipfs-core-api)** | 0 / 0 | Sun Oct 30 2016 |
| 🚀 | **[Directory Sharding](#directory-sharding)** | 1 / 2 | Mon Nov 07 2016 |
| 🚀 | **[ipfs 0.4.5](#ipfs-0.4.5)** | 0 / 2 | Fri Nov 18 2016 |
| 🚀 | **[Filestore implementation](#filestore-implementation)** | 5 / 9 | Sun Dec 04 2016 |
| 🚀 | **[Dont Kill Routers](#dont-kill-routers)** | 0 / 1 | Sun Dec 11 2016 |

## Milestones and Goals

#### ipld integration

> integration of the ipld data format into go-ipfs

🚀 &nbsp;**OPEN** &nbsp;&nbsp;📉 &nbsp;&nbsp;**2 / 2** goals completed **(100%)** &nbsp;&nbsp;📅 &nbsp;&nbsp;**Fri Oct 28 2016**

See [milestone goals](https://waffle.io/ipfs/go-ipfs?milestone=ipld%20integration) for the list of goals this milestone has.
#### IPFS Core API

> This milestone's goal is to extract the gateway code into its own tool. This will facilitate the implementation of the Core API in go-ipfs.

In the past months we've established a core set of commands that IPFS nodes can support. The JS implementation (js-ipfs and js-ipfs-api) is already compliant, and this milestone is all about starting to make the Go implementation (go-ipfs and go-ipfs-api) compliant. Check out https://github.com/ipfs/interface-ipfs-core

🚀 &nbsp;**OPEN** &nbsp;&nbsp;📉 &nbsp;&nbsp;**0 / 0** goals completed **(0%)** &nbsp;&nbsp;📅 &nbsp;&nbsp;**Sun Oct 30 2016**

See [milestone goals](https://waffle.io/ipfs/go-ipfs?milestone=IPFS%20Core%20API) for the list of goals this milestone has.
#### Directory Sharding

> ipfs unixfs currently can't handle large directories. We need to shard directories after they get to a certain size.

🚀 &nbsp;**OPEN** &nbsp;&nbsp;📉 &nbsp;&nbsp;**1 / 2** goals completed **(50%)** &nbsp;&nbsp;📅 &nbsp;&nbsp;**Mon Nov 07 2016**

See [milestone goals](https://waffle.io/ipfs/go-ipfs?milestone=Directory%20Sharding) for the list of goals this milestone has.
#### ipfs 0.4.5

> Version 0.4.5 of go-ipfs

🚀 &nbsp;**OPEN** &nbsp;&nbsp;📉 &nbsp;&nbsp;**0 / 2** goals completed **(0%)** &nbsp;&nbsp;📅 &nbsp;&nbsp;**Fri Nov 18 2016**

See [milestone goals](https://waffle.io/ipfs/go-ipfs?milestone=ipfs%200.4.5) for the list of goals this milestone has.
#### Filestore implementation

> 

🚀 &nbsp;**OPEN** &nbsp;&nbsp;📉 &nbsp;&nbsp;**5 / 9** goals completed **(55%)** &nbsp;&nbsp;📅 &nbsp;&nbsp;**Sun Dec 04 2016**

See [milestone goals](https://waffle.io/ipfs/go-ipfs?milestone=Filestore%20implementation) for the list of goals this milestone has.
#### Dont Kill Routers

> Ipfs should strive not to kill peoples home internet connection. 

This milestone is for tracking router killer issues beyond the normal bandwidth problems.

🚀 &nbsp;**OPEN** &nbsp;&nbsp;📉 &nbsp;&nbsp;**0 / 1** goals completed **(0%)** &nbsp;&nbsp;📅 &nbsp;&nbsp;**Sun Dec 11 2016**

See [milestone goals](https://waffle.io/ipfs/go-ipfs?milestone=Dont%20Kill%20Routers) for the list of goals this milestone has.

