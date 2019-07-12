# `go-ipfs` Release Flow

## Table of Contents

- [Release Philosophy](#release-philosophy)
- [Release Flow](#release-flow)
- [Performing a Release](#performing-a-release)
- [Release Version Numbers (aka semver)](#release-version-numbers-aka-semver)

## Release Philosophy

`go-ipfs` aims to have release every six weeks, two releases per quarter. During these 6 week releases, we go through 4 different stages that gives us the opportunity to test the new version against our test environments (unit, interop, integration), QA in our current production environment, IPFS apps (e.g. Desktop and WebUI) and with our community and _early testers_<sup>[1]</sup> that have IPFS running in production.

We might expand the six week release schedule in case of:
- No new updates to be added
- In case of a large community event that takes the core team availability away (e.g. IPFS Conf, Dev Meetings, IPFS Camp, etc)

## Release Flow

`go-ipfs` releases come in 4 stages designed to gradually roll out changes and reduce the impact of any regressions that may have been introduced. If we need to merge non-trivial<sup>[2]</sup> changes during the process, we start over at stage 1.

### Stage 1 - Internal Testing

Before this stage, we expect _all_ tests (interop, testlab, performance, etc.) to pass.

At this stage, we'll:
- 1. Start a partial-rollout to our own infrastructure.
- 2. Test against ipfs and ipfs-shipyard applications.

**Goal(s):**
- 1. Make sure we haven't introduced any obvious regressions.
- 2. Test the release in an environment we can monitor and easily roll back (i.e., our own infra).

### Stage 2 - Public Beta

At this stage, we'll announce the impending release to the community and ask for beta testers.

**Goal:** Test the release in as many non-production environments as possible. This is relatively low-risk but gives us a _breadth_ of testing internal testing can't.

### Stage 3 - Soft Release

At this stage, we consider the release to be "production ready" and ask will ask the community and our early testers to (partially) deploy the release to their production infrastructure.

**Goal(s):**

1. Test the release in some production environments with heavy workloads.
2. Partially roll-out an upgrade to see how it affects the network.
3. Retain the ability to ship last-minute fixes before the final release.

### Stage 4 - Release

At this stage, the release is "battle hardened" and ready for wide deployment.

## Performing a Release

The release is managed by the `Lead Maintainer` for `go-ipfs`. It starts with the opening of an issue containing the content available on the [RELEASE_ISSUE_TEMPLATE](./RELEASE_ISSUE_TEMPLATE.md). Then, the 4 stages will be followed until the release is done.

## Release Version Numbers (aka semver)

Until `go-ipfs` 0.4.X, `go-ipfs` was not using semver to communicate the type of release

Post `go-ipfs` 0.5.X, `go-ipfs` will use semver. This means that patch releases will not contain any breaking changes nor new features. Minor releases might contain breaking changes and always contain some new feature

Post `go-ipfs` 1.X.X (future), `go-ipfs` will use semver. This means that only major releases will contain breaking changes, minors will be reserved for new features and patches for bug fixes.

We do not yet retroactively apply fixes to older releases (no Long Term Support releases for now), which means that we always recommend users to update to the latest, whenever possible.

----------------------------

- <sup>**[1]**</sup> - _early testers_ is an IPFS programme in which members of the community can self-volunteer to help test `go-ipfs` Release Candidates. You find more info about it at [EARLY_TESTERS.md](./EARLY_TESTERS.md)
- <sup>**[2]**</sup> - A non-trivial change is any change that could potentially introduce an issue not trivially caught by automated testing. This is up to the discretion of the Lead Maintainer but the assumption is that every change is non-trivial unless proven otherwise.
