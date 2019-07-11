# `go-ipfs` Release Flow

## Table of Contents

- [Release Philosophy](#release-philosophy)
- [Release Flow](#release-flow)
- [Performing a Release](#performing-a-release)
- [Release Version Numbers (aka semver)](#release-version-numbers-aka-semver)

## Release Philosophy

`go-ipfs` aims to have release every six weeks, two releases per quarter. During these 6 week releases, we go through 4 different stages that gives us the opportunity to test the new version against our test environments (unit, interop, integration), QA in our current production environment, IPFS apps (e.g. Desktop and WebUI) and with our community and _early testers_<sup>[1]</sup> that have IPFS running in Production.

We might expand the six week release schedule in case of:
- No new updates to be added
- In case of a large community event that takes the core team availability away (e.g. IPFS Conf, Dev Meetings, IPFS Camp, etc)

## Release Flow

`go-ipfs` releases come in 4 stages designed to gradually roll out changes and reduce the impact of any regressions that may have been introduced.

### Alpha

Before this stage, we expect _all_ tests (interop, testlab, performance, etc.) to pass.

At this stage, we'll:

1. Start a partial-rollout to our own infrastructure.
2. Test against ipfs and ipfs-shipyard applications.

**Goal(s):**

1. Make sure we haven't introduced any obvious regressions.
2. Test the release in an environment we can monitor and easily roll back (i.e., our own infra).

### Beta

At this stage, we'll announce the impending release to the community and ask for beta testers.

**Goal:** Test the release in as many non-production environments as possible. This is relatively low-risk but gives us a _breadth_ of testing internal testing can't.

### Release Candidate: Stage 1

At this stage, we consider the release to be "production ready" and ask will ask our early testers to (partially) deploy the release to their production infrastructure.

**Goal(s):**

1. Test the release in some production environments with heavy workloads.
2. Partially roll-out an upgrade to see how it affects the network.
3. Retain the ability to ship last-minute fixes before the final release.

### Release Candidate: Stage 2

At this stage, the release is "battle hardened" and ready for wide deployment. We ask the wider community to deploy and test but expect no changes between this RC and the final release.

**Goal:** Test the release on as many production workloads as possible while retaining the ability to ship last-minute fixes before the final release.

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

For each pre-release published in each stage:
- [ ] version string in `version.go` has been updated
- [ ] tag commit with vX.Y.Z-{alphaN,betaN,rcN}

### Alpha - Internal testing

When releasing the first alpha, there is a features freeze for the release branch.

- [ ] CHANGELOG.md has been updated
  - use `./bin/mkreleaselog` to generate a nice starter list
  - use `./doc/RELEASE_TEMPLATE.md` as a template
- [ ] Automated Testing (already tested in CI) - Ensure that all tests are passing, this includes:
  - [ ] unit
  - [ ] sharness
  - [ ] [interop](https://github.com/ipfs/interop#test-with-a-non-yet-released-version-of-go-ipfs)
  - [ ] go-ipfs-api
  - [ ] go-ipfs-http-client
- [ ] Network Testing:
  - [ ] test lab things
- [ ] Infrastructure Testing:
  - [ ] Deploy new version to a subset of Bootstrappers
  - [ ] Deploy new version to a subset of Gateways
  - [ ] Deploy new version to a subset of Preload nodes
  - [ ] Collect metrics every day. Work with the Infrastructure team to learn of any hiccup
- [ ] IPFS Application Testing -  Run the tests of the following applications:
  - [ ] WebUI
  - [ ] IPFS Desktop
  - [ ] IPFS Companion
  - [ ] NPM on IPFS

### Beta - Community Testing

- [ ] Reach out to the IPFS _early testers_ listed in `docs/EARLY_TESTERS.md` for testing this release (check when no more problems have been reported).
- [ ] Reach out to on IRC for beta testers.
- [ ] Run tests available in the following repos with the latest beta (check when all tests pass):
  - [ ] [orbit-db](https://github.com/orbitdb/orbit-db)

PSA: If you are a heavy user of `go-ipfs`, have developed a solid test infrastructure for your application and would love to help us would like to help us test `go-ipfs` release candidates, reach out to go-ipfs-wg@ipfs.io.

### Release Candidate - Stage 1 - Early Tester Production Testing

- [ ] Documentation
  - [ ] Ensure that CHANGELOG.md is up to date
  - [ ] Ensure that README.md is up to date
  - [ ] Ensure that all the examples we have produced for go-ipfs run without problems
  - [ ] Update HTTP-API Documentation on the Website using https://github.com/ipfs/http-api-docs
- [ ] Invite the IPFS _early testers_ to deploy the release to part of their production infrastructure.

### Release Candidate - Stage 2 - Wider Production Testing

- [ ] Invite the wider community through (link to the release issue):
  - [ ] [discuss.ipfs.io](https://discuss.ipfs.io/c/announcements)
  - [ ] Twitter
  - [ ] IRC
  
### Release

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
```

## Release Version Numbers (aka semver)

Until `go-ipfs` 0.4.X, `go-ipfs` was not using semver to communicate the type of release

Post `go-ipfs` 0.5.X, `go-ipfs` will use semver. This means that patch releases will not contain any breaking changes nor new features. Minor releases might contain breaking changes and always contain some new feature

Post `go-ipfs` 1.X.X (future), `go-ipfs` will use semver. This means that only major releases will contain breaking changes, minors will be reserved for new features and patches for bug fixes.

We do not yet retroactively apply fixes to older releases (no Long Term Support releases for now), which means that we always recommend users to update to the latest, whenever possible.
TBW

----------------------------

- <sup>**[1]**</sup> - _early testers_ is an IPFS programme in which members of the community can self-volunteer to help test `go-ipfs` Release Candidates. You find more info about it at [EARLY_TESTERS.md](./EARLY_TESTERS.md)
