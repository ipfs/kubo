# `go-ipfs` Release Flow

## Table of Contents

- [Release Philosophy](#release-philosophy)
- [Release Flow](#release-flow)
  - [Stage 0 - Automated Testing](#stage-0---automated-testing)
  - [Stage 1 - Internal Testing](#stage-1---internal-testing)
  - [Stage 2 - Community Dev Testing](#stage-2---community-dev-testing)
  - [Stage 3 - Community Prod Testing](#stage-3---community-prod-testing)
  - [Stage 4 - Release](#stage-4---release)
- [Release Cycle](#release-cycle)
  - [Patch Releases](#patch-releases)
- [Performing a Release](#performing-a-release)
- [Release Version Numbers (aka semver)](#release-version-numbers-aka-semver)

## Release Philosophy

`go-ipfs` aims to have release every six weeks, two releases per quarter. During these 6 week releases, we go through 4 different stages that gives us the opportunity to test the new version against our test environments (unit, interop, integration), QA in our current production environment, IPFS apps (e.g. Desktop and WebUI) and with our community and _early testers_<sup>[1]</sup> that have IPFS running in production.

We might expand the six week release schedule in case of:

- No new updates to be added
- In case of a large community event that takes the core team availability away (e.g. IPFS Conf, Dev Meetings, IPFS Camp, etc.)

## Release Flow

`go-ipfs` releases come in 5 stages designed to gradually roll out changes and reduce the impact of any regressions that may have been introduced. If we need to merge non-trivial<sup>[2]</sup> changes during the process, we start over at stage 0.

![go-ipfs-release-process-illustration](https://user-images.githubusercontent.com/618519/62986422-653fee00-bdf0-11e9-8f61-197117b61da2.png)

### Stage 0 - Automated Testing

At this stage, we expect _all_ automated tests (interop, testlab, performance, etc.) to pass.

### Stage 1 - Internal Testing

At this stage, we'll:

1. Start a partial-rollout to our own infrastructure.
2. Test against ipfs and ipfs-shipyard applications.

**Goals:**

1. Make sure we haven't introduced any obvious regressions.
2. Test the release in an environment we can monitor and easily roll back (i.e. our own infra).

### Stage 2 - Community Dev Testing

At this stage, we'll announce the impending release to the community and ask for beta testers.

**Goal:**

Test the release in as many non-production environments as possible. This is relatively low-risk but gives us a _breadth_ of testing internal testing can't.

### Stage 3 - Community Prod Testing

At this stage, we consider the release to be "production ready" and will ask the community and our early testers to (partially) deploy the release to their production infrastructure.

**Goals:**

1. Test the release in some production environments with heavy workloads.
2. Partially roll-out an upgrade to see how it affects the network.
3. Retain the ability to ship last-minute fixes before the final release.

### Stage 4 - Release

At this stage, the release is "battle hardened" and ready for wide deployment.

## Release Cycle

A full release process should take about 3 weeks, a week per stage 1-3. We will start a new process every 6 weeks, regardless of when the previous release landed unless it's still ongoing.

### Patch Releases

If we encounter a serious bug in the stable latest release, we will create a patch release based on this release. For now, bug fixes will _not_ be backported to previous releases.

Patch releases will usually follow a compressed release cycle and should take 2-3 days. In a patch release:

1. Automated and internal testing (stage 0 and 1) will be compressed into a few hours - ideally less than a day.
2. Stage 2 will be skipped.
3. Community production testing will be shortened to 1-2 days of opt-in testing in production (early testers can choose to pass).

Some patch releases, especially ones fixing one or more complex bugs, may undergo the full release process.

## Performing a Release

The release is managed by the `Lead Maintainer` for `go-ipfs`. It starts with the opening of an issue containing the content available on the [RELEASE_ISSUE_TEMPLATE](./RELEASE_ISSUE_TEMPLATE.md) not more than **48 hours** after the previous release.

This issue is pinned and labeled ["release"](https://github.com/ipfs/go-ipfs/issues?utf8=%E2%9C%93&q=is%3Aissue+is%3Aopen+label%3Arelease). When the cycle is due to begin the 5 stages will be followed until the release is done.

## Release Version Numbers (aka semver)

Until `go-ipfs` 0.4.X, `go-ipfs` was not using semver to communicate the type of release

Post `go-ipfs` 0.5.X, `go-ipfs` will use semver. This means that patch releases will not contain any breaking changes nor new features. Minor releases might contain breaking changes and always contain some new feature

Post `go-ipfs` 1.X.X (future), `go-ipfs` will use semver. This means that only major releases will contain breaking changes, minors will be reserved for new features and patches for bug fixes.

We do not yet retroactively apply fixes to older releases (no Long Term Support releases for now), which means that we always recommend users to update to the latest, whenever possible.

----------------------------

- <sup>**[1]**</sup> - _early testers_ is an IPFS programme in which members of the community can self-volunteer to help test `go-ipfs` Release Candidates. You find more info about it at [EARLY_TESTERS.md](./EARLY_TESTERS.md)
- <sup>**[2]**</sup> - A non-trivial change is any change that could potentially introduce an issue not trivially caught by automated testing. This is up to the discretion of the Lead Maintainer but the assumption is that every change is non-trivial unless proven otherwise.
