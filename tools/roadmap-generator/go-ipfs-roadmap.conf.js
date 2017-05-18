'use strict'

module.exports = {
  // Name of the organization or project this roadmap is generated for
  organization: 'go-ipfs',

  // Include open and closed milestones where due date is after milestonesStartDate
  milestonesStartDate: '2016-10-01T00:00:00Z', // ISO formatted timestamp

  // Include open and closed milestones where due date is before milestonesEndDate
  milestonesEndDate: '2016-12-30T00:00:00Z', // ISO formatted timestamp

  // Github repository to open open a Pull Request with the generated roadmap
  targetRepo: "ipfs/go-ipfs", // 'owner/repo'

  // List of projects that this roadmap covers
  projects: [
    {
      name: "go-ipfs",
      // Repositories that this project consists of.
      repos: [
        "ipfs/go-ipfs",
        "ipfs/go-cid",
        "ipfs/go-datastore",
        "ipfs/go-ds-flatfs",
        "ipfs/go-ds-measure",
        "ipfs/go-ipfs-api",
        "ipfs/go-ipfs-util",
        "ipfs/go-key",
        "ipfs/go-log",
        "libp2p/go-libp2p",
        "libp2p/go-libp2p-crypto",
        "libp2p/go-libp2p-loggable",
        "libp2p/go-libp2p-peer",
        "libp2p/go-libp2p-peerstore",
        "libp2p/go-libp2p-secio",
        "libp2p/go-libp2p-transport",
        "whyrusleeping/go-libp2p-pubsub"
      ],
      // WIP
      links: {
        status: `## Status and Progress\n
[![Project Status](https://badge.waffle.io/ipfs/go-ipfs.svg?label=Backlog&title=Backlog)](http://waffle.io/ipfs/go-ipfs) [![Project Status](https://badge.waffle.io/ipfs/go-ipfs.svg?label=In%20Progress&title=In%20Progress)](http://waffle.io/ipfs/go-ipfs) [![Project Status](https://badge.waffle.io/ipfs/go-ipfs.svg?label=Done&title=Done)](http://waffle.io/ipfs/go-ipfs)\n
See details of current progress on [Orbit's project board](https://waffle.io/haadcode/orbit)\n\n`
      }
    },
  ]
}
