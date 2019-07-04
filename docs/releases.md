# `go-ipfs` Release Flow

## Table of Contents

- [Release Philosophy](#release-philosophy)
- [Release Flow](#release-flow)
- [Performing a Release](#performing-a-release)
- [Release Version Numbers (aka semver)](#release-version-numbers-aka-semver)

## Release Philosophy

`go-ipfs` aims to have release every six weeks, two releases per quarter. During these 6 week releases, we go through 4 different stages of Release Candidates (RC) that gives us the opportunity to test the new version against our test environments (unit, interop, integration), QA in our current production environmment, IPFS apps (e.g. Desktop and WebUI) and with our _early testers_<sup>[1]</sup> that have IPFS running in Production, by leveraging their own test infrastructure and QA systems.

We might expand the six week release schedule in case of:
- No new updates to be added
- In case of a large community event that takes the core team availability away (e.g. IPFS Conf, Dev Meetings, IPFS Camp, etc)

## Release Flow

`go-ipfs` releases come in 4 stages:

- **Release Stage 1 - Internal testing** - Test the Release against our testing infrastructure, including interoperability, integration, test lab, multiple runtimes and the apps we've built (WebUI, Desktop, NPM on IPFS, HTTP Client Libraries). The intent is to make this stage fully automated (and somewhat is already), until then, we manually check a list and ensure that all tests have been run
- **Release Stage 2 - Invite _early testers_ to try it out** - Reach out to our _early testers_ (i.e. projects that have volunteered to support `go-ipfs` by using their own test infrastructure and tell us the results)
- **Release Stage 3 - Announce to the broader community** - Communicate to the community that a new Release Candidate is ready and that everyone is welcome to test it with us
- **Release Stage 4 - Complete the Release** - Finalize the release, start the next release.

The Release Stages are not linked to Release Candidate numbers, in fact, there can be multiple release candidate per stages as we catch bugs and improve the release itself.

<p align="center">
  <a href="https://ipfs.io">
    <img src="https://gateway.ipfs.io/ipfs/QmaFtLxoCAm5vFQ9AftKkhJwSAdDdF1jzV9DfzW6gbXqFL/Paper.Sketches.23.png" width="450" />
  </a>
</p>

## Performing a Release

The first step is for the `Lead Maintainer` for `go-ipfs` to open an issue with Title `go-ipfs <version> Release` and a c&p of the following template:

```
> <short tl;dr; of the release>

# 🗺 What's left for release

<List of items with PRs and/or Issues to be considered for this release>

# 🔦 Highlights

<List of items with PRs and/or Issues included for this release>

# 🏗 API Changes

<List of API changes, if any>

# ✅ Release Checklist

For each RC published in each stage:
- [ ] version string in `version.go` has been updated
- [ ] tag commit with vX.Y.Z-rcN

### Release Stage 1 - Internal testing

When Release Stage 1, there is a features freeze for the release branch.

- [ ] CHANGELOG.md has been updated
  - use `./bin/mkreleaselog` to generate a nice starter list
- [ ] Automated Testing - Ensure that all tests are passing, this includes:
  - [ ] unit
  - [ ] sharness
  - [ ] [interop](https://github.com/ipfs/interop#test-with-a-non-yet-released-version-of-go-ipfs)
- [ ] Network Testing:
  - [ ] test lab things
- [ ] Infrastructure Testing:
  - [ ] Deploy new version to a subset of Bootstrappers
  - [ ] Deploy new version to a subset of Gateways
  - [ ] Deploy new version to a subset of Preload nodes
  - [ ] Collect metrics every day. Work with the Infrastructure team to learn of any hiccup
- [ ] IPFS HTTP Client Libraries Testing:
  - [ ] [JS](http://github.com/ipfs/js-ipfs-http-client)
  - [ ] [Go](https://github.com/ipfs/go-ipfs-api)
- [ ] IPFS Application Testing -  Run the tests of the following applications:
  - [ ] WebUI
  - [ ] IPFS Desktop
  - [ ] IPFS Companion
  - [ ] NPM on IPFS

### Release Stage 2 - Invite _early testers_ to try it out

- [ ] Reach out to the following IPFS _early testers_ for testing this release (check when no more problems have been reported):
  - [ ] Infura
  - [ ] Textila
  - [ ] Pinata
  - [ ] QRI
- [ ] Run tests available in the following repos with the latest RC (check when all tests pass):
  - [ ] [orbit-db](https://github.com/orbitdb/orbit-db)

PSA: If you are a heavy user of `go-ipfs`, have developed a solid test infrastructure for your application and would love to help us would like to help us test `go-ipfs` release candidates, reach out to go-ipfs-wg@ipfs.io.

### Release Stage 3 - Announce to the broader community

- [ ] Documentation
  - [ ] Ensure that CHANGELOG.md is up to date
  - [ ] Ensure that README.md is up to date
  - [ ] Ensure that all the examples we have produced for go-ipfs run without problems
  - [ ] Update HTTP-API Documentation on the Website using https://github.com/ipfs/http-api-docs
- [ ] Invite the community through (link to the release issue):
  - [ ] [discuss.ipfs.io](https://discuss.ipfs.io/c/announcements)
  - [ ] Twitter
  - [ ] IRC

### Release Stage 4 - Complete the Release

- [ ] Final preparation
  - [ ] Verify that version string in `repo/version.go` has been updated
  - [ ] tag commit with vX.Y.Z
  - [ ] update release branch to point to release commit (`git merge vX.Y.Z`).
  - [ ] publish dist.ipfs.io
  - [ ] publish next version to https://github.com/ipfs/npm-go-ipfs
- [ ] Publish a Release Blog post (at minimum, a c&p of this release issue with all the highlights, API changes, link to changelog and thank yous)
- [ ] Broadcasting (link to blog post)
  - [ ] Twitter
  - [ ] IRC
  - [ ] Reddit
  - [ ] [discuss.ipfs.io](https://discuss.ipfs.io/c/announcements)
  - [ ] Announce it on the [IPFS Users mlist](https://groups.google.com/forum/#!forum/ipfs-users)

# ❤️ Huge thank you to everyone that made this release possible

In alphabetical order, here are all the humans that contributed to the release:

- <use script -- listed in Release Stage 4 -- to generate a list of everyone that contributed for this release>

# 🙌🏽 Want to contribute?

Would you like to contribute to the IPFS project and don't know how? Well, there are a few places you can get started:

- Check the issues with the `help wanted` label in the [go-ipfs repo](https://github.com/ipfs/go-ipfs/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22)
- Join an IPFS All Hands, introduce yourself and let us know where you would like to contribute - https://github.com/ipfs/team-mgmt/#weekly-ipfs-all-hands
- Hack with IPFS and show us what you made! The All Hands call is also the perfect venue for demos, join in and show us what you built
- Join the discussion at http://discuss.ipfs.io/ and help users finding their answers.
- Join the [Go Core Dev Team Weekly Sync](https://github.com/ipfs/team-mgmt/issues/650) and be part of the Sprint action!

# ⁉️ Do you have questions?

The best place to ask your questions about IPFS, how it works and what you can do with it is at [discuss.ipfs.io](http://discuss.ipfs.io). We are also available at the `#ipfs` channel on Freenode, which is also [accessible through our Matrix bridge](https://riot.im/app/#/room/#freenode_#ipfs:matrix.org).
```

## Release Version Numbers (aka semver)

Until `go-ipfs` 0.4.X, `go-ipfs` was not using semver to communicate the type of release

Post `go-ipfs` 0.5.X, `go-ipfs` will use semver. This means that patch releases will not contain any breaking changes nor new features. Minor releases might contain breaking changes and always contain some new feature

Post `go-ipfs` 1.X.X (future), `go-ipfs` will use semver. This means that only major releases will contain breaking changes, minors will be reserved for new features and patches for bug fixes.

We do not yet retroactively apply fixes to older releases (no Long Term Support releases for now), which means that we always recommend users to update to the latest, whenever possible.
TBW

----------------------------

- <sup>**[1]**</sup> - _early testers_ is an IPFS program in which members of the community can self-volunteer to help test `go-ipfs` Release Candidates. You find more info about it at [EARLY_TESTERS.md](./EARLY_TESTERS.md)
