> Release Issue Template

# go-ipfs X.Y.Z Release

We're happy to announce go-ipfs X.Y.Z, bla bla...

As usual, this release includes important fixes, some of which may be critical for security. Unless the fix addresses a bug being exploited in the wild, the fix will _not_ be called out in the release notes. Please make sure to update ASAP. See our [release process](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md#security-fix-policy) for details.

## 🗺 What's left for release

<List of items with PRs and/or Issues to be considered for this release>

# 🚢 Estimated shipping date

<Date this release will ship on if everything goes to plan (week beginning...)>

## 🔦 Highlights

< top highlights for this release notes >

## ✅ Release Checklist

For each RC published in each stage:

- version string in `version.go` has been updated (in the `release-vX.Y.Z` branch).
- tag commit with `vX.Y.Z-rcN`
- upload to dist.ipfs.io
  1. Build: https://github.com/ipfs/distributions#usage.
  2. Pin the resulting release.
  3. Make a PR against ipfs/distributions with the updated versions, including the new hash in the PR comment.
  4. Ask the infra team to update the DNSLink record for dist.ipfs.io to point to the new distribution.
- cut a pre-release on [github](https://github.com/ipfs/go-ipfs/releases) and upload the result of the ipfs/distributions build in the previous step.
- Announce the RC:
  - [ ] On Matrix (both #ipfs and #ipfs-dev)
  - [ ] To the _early testers_ listed in [docs/EARLY_TESTERS.md](https://github.com/ipfs/go-ipfs/tree/master/docs/EARLY_TESTERS.md).  Do this by copy/pasting their GitHub usernames and checkboxes as a comment so they get a GitHub notification.  ([example](https://github.com/ipfs/go-ipfs/issues/8176#issuecomment-909356394))

Checklist:

- [ ] **Stage 0 - Automated Testing**
  - [ ] Upgrade to the latest patch release of Go that CircleCI has published
    - [ ] See the list here: https://hub.docker.com/r/cimg/go/tags
    - [ ] [ipfs/distributions](https://github.com/ipfs/distributions): bump [this version](https://github.com/ipfs/distributions/blob/master/.tool-versions#L2)
    - [ ] [ipfs/go-ipfs](https://github.com/ipfs/go-ipfs): [example PR](https://github.com/ipfs/go-ipfs/pull/8599)
  - [ ] Fork a new branch (`release-vX.Y.Z`) from `master` and make any further release related changes to this branch. If any "non-trivial" changes (see the footnotes of [docs/releases.md](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md) for a definition) get added to the release, uncheck all the checkboxes and return to this stage.
    - [ ] Follow the RC release process to cut the first RC.
    - [ ] Bump the version in `version.go` in the `master` branch to `vX.(Y+1).0-dev`.
  - [ ] Automated Testing (already tested in CI) - Ensure that all tests are passing, this includes:
    - [ ] unit, sharness, cross-build, etc (`make test`)
    - [ ] lint (`make test_go_lint`)
    - [ ] [interop](https://github.com/ipfs/interop#test-with-a-non-yet-released-version-of-go-ipfs)
    - [ ] [go-ipfs-api](https://github.com/ipfs/go-ipfs-api)
    - [ ] [go-ipfs-http-client](https://github.com/ipfs/go-ipfs-http-client)
    - [ ] [WebUI](https://github.com/ipfs-shipyard/ipfs-webui)
- [ ] **Stage 1 - Internal Testing**
  - [ ] CHANGELOG.md has been updated
    - use [`./bin/mkreleaselog`](https://github.com/ipfs/go-ipfs/tree/master/bin/mkreleaselog) to generate a nice starter list
  - [ ] Infrastructure Testing:
    - [ ] Deploy new version to a subset of Bootstrappers
    - [ ] Deploy new version to a subset of Gateways
    - [ ] Deploy new version to a subset of Preload nodes
    - [ ] Collect metrics every day. Work with the Infrastructure team to learn of any hiccup
  - [ ] IPFS Application Testing -  Run the tests of the following applications:
    - [ ] [IPFS Desktop](https://github.com/ipfs-shipyard/ipfs-desktop)
      - [ ] Ensure the RC is published to [the NPM package](https://www.npmjs.com/package/go-ipfs?activeTab=versions) ([happens automatically, just wait for CI](https://github.com/ipfs/npm-go-ipfs/actions))
      - [ ] Upgrade to the RC in [ipfs-desktop](https://github.com/ipfs-shipyard/ipfs-desktop) and push to a branch ([example](https://github.com/ipfs/ipfs-desktop/pull/1826/commits/b0a23db31ce942b46d95965ee6fe770fb24d6bde)), and open a draft PR to track through the final release ([example](https://github.com/ipfs/ipfs-desktop/pull/1826))
      - [ ] Ensure CI tests pass, repeat for new RCs
    - [ ] [IPFS Companion](https://github.com/ipfs-shipyard/ipfs-companion) - @lidel
- [ ] **Stage 2 - Community Prod Testing**
  - [ ] Documentation
    - [ ] Ensure that [CHANGELOG.md](https://github.com/ipfs/go-ipfs/tree/master/CHANGELOG.md) is up to date
    - [ ] Ensure that [README.md](https://github.com/ipfs/go-ipfs/tree/master/README.md)  is up to date
    - [ ] Update docs by merging the auto-created PR in https://github.com/ipfs/ipfs-docs/pulls (they are auto-created every 12 hours)
  - [ ] Invite the wider community through (link to the release issue):
    - [ ] [discuss.ipfs.io](https://discuss.ipfs.io/c/announcements)
    - [ ] Matrix
- [ ] **Stage 3 - Release**
  - [ ] Final preparation
    - [ ] Verify that version string in [`version.go`](https://github.com/ipfs/go-ipfs/tree/master/version.go) has been updated.
    - [ ] Merge `release-vX.Y.Z` into the `release` branch.
    - [ ] Tag this merge commit (on the `release` branch) with `vX.Y.Z`.
    - [ ] Release published
      - [ ] to [dist.ipfs.io](https://dist.ipfs.io)
      - [ ] to [npm-go-ipfs](https://github.com/ipfs/npm-go-ipfs)
      - [ ] to [chocolatey](https://chocolatey.org/packages/go-ipfs)
      - [ ] to [snap](https://snapcraft.io/ipfs)
      - [ ] to [github](https://github.com/ipfs/go-ipfs/releases)
        - [ ] use the artifacts built in CI for dist.ipfs.io: `wget "https://ipfs.io/api/v0/get?arg=/ipns/dist.ipfs.io/go-ipfs/$(curl -s https://dist.ipfs.io/go-ipfs/versions | tail -n 1)"`
      - [ ] to [arch](https://www.archlinux.org/packages/community/x86_64/go-ipfs/) (flag it out of date)
    - [ ] Cut a new ipfs-desktop release
  - [ ] Submit [this form](https://airtable.com/shrNH8YWole1xc70I) to publish a blog post, linking to the GitHub release notes
  - [ ] Broadcasting (link to blog post)
    - [ ] Twitter (request in Slack channel #pl-marketing-requests)
    - [ ] Matrix
    - [ ] [Reddit](https://reddit.com/r/ipfs)
    - [ ] [discuss.ipfs.io](https://discuss.ipfs.io/c/announcements)
    - [ ] Announce it on the [IPFS Users Mailing List](https://groups.google.com/forum/#!forum/ipfs-users)
- [ ] **Post-Release**
  - [ ] Merge the `release` branch back into `master`, ignoring the changes to `version.go` (keep the `-dev` version from master).
  - [ ] Create an issue using this release issue template for the _next_ release.
  - [ ] Make sure any last-minute changelog updates from the blog post make it back into the CHANGELOG.
  - [ ] Mark PR draft created for IPFS Desktop as ready for review.


## ⁉️ Do you have questions?

The best place to ask your questions about IPFS, how it works and what you can do with it is at [discuss.ipfs.io](http://discuss.ipfs.io). We are also available at the `#ipfs` channel on Freenode, which is also [accessible through our Matrix bridge](https://riot.im/app/#/room/#freenode_#ipfs:matrix.org).

## Release improvements for next time

< Add any release improvements that were observed this cycle here so they can get incorporated into future releases. >

## Items for a separate comment

< Do these as a separate comment to avoid the main issue from getting too large and checkbox updates taking too long. >

### Changelog

< changelog generated by bin/mkreleaselog >

### ❤️ Contributors

< list generated by bin/mkreleaselog >

Would you like to contribute to the IPFS project and don't know how? Well, there are a few places you can get started:

- Check the issues with the `help wanted` label in the [go-ipfs repo](https://github.com/ipfs/go-ipfs/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22)
- Join an IPFS All Hands, introduce yourself and let us know where you would like to contribute - https://github.com/ipfs/team-mgmt/#weekly-ipfs-all-hands
- Hack with IPFS and show us what you made! The All Hands call is also the perfect venue for demos, join in and show us what you built
- Join the discussion at [discuss.ipfs.io](https://discuss.ipfs.io/) and help users finding their answers.
- Join the [🚀 IPFS Core Implementations Weekly Sync 🛰](https://github.com/ipfs/team-mgmt/issues/992) and be part of the action!
