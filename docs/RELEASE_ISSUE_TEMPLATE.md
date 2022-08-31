> Release Issue Template.  If doing a patch release, see [here](https://github.com/ipfs/kubo/blob/master/docs/PATCH_RELEASE_TEMPLATE.md)

# Items to do upon creating the release issue
- [ ] Fill in the Meta section
- [ ] Assign the issue to the release owner and reviewer.
- [ ] Name the issue "Release vX.Y.Z"
- [ ] Set the proper values for X.Y.Z
- [ ] Pin the issue

# Meta
* Release owner: @who
* Release reviewer: @who
* Expected RC date: week of 2022-MM-DD
* 🚢 Expected final release date: 2022-MM-DD
* Accompanying PR for improving the release process: (example: https://github.com/ipfs/kubo/pull/9100)

See the [Kubo release process](https://pl-strflt.notion.site/Kubo-Release-Process-5a5d066264704009a28a79cff93062c4) for more info.

# Kubo X.Y.Z Release

We're happy to announce Kubo X.Y.Z, bla bla...

As usual, this release includes important fixes, some of which may be critical for security. Unless the fix addresses a bug being exploited in the wild, the fix will _not_ be called out in the release notes. Please make sure to update ASAP. See our [release process](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md#security-fix-policy) for details.

## 🗺 What's left for release

<List of items with PRs and/or Issues to be considered for this release>

## 🔦 Highlights

< top highlights for this release notes. For ANY version (final or RCs) >

## ✅ Release Checklist

For each RC published in each stage:

- version string in `version.go` has been updated (in the `release-vX.Y.Z` branch).
- new commits should be added to the `release-vX.Y.Z` branch from `master` using `git cherry-pick -x ...`
  - `release-` branches are protected. You can do all needed updates on a separated branch and when everything is settled push to `release-vX.Y.Z`
- tag commit with `vX.Y.Z-rcN`
- add artifacts to https://dist.ipfs.tech
  1. Make a PR against [ipfs/distributions](https://github.com/ipfs/distributions) with local changes produced by `add-version` (see [usage](https://github.com/ipfs/distributions#usage))
  2. Wait for PR to build artifacts and generate diff (~30min)
  3. Inspect results, merge if CI is green and the diff looks ok
  4. Wait for `master` branch to build and update DNSLink at https://dist.ipfs.tech (~30min)
- cut a pre-release on [github](https://github.com/ipfs/kubo/releases) and reuse signed artifacts from https://dist.ipfs.tech/kubo (upload the result of the ipfs/distributions build in the previous step).
- Announce the RC:
  - [ ] Create a new post on [Discuss](https://discuss.ipfs.tech)
    - This will automatically post to IPFS Discord #ipfs-chatter
    - Examples from the past: [0.14.0](https://discuss.ipfs.io/t/kubo-formerly-go-ipfs-v0-14-0-release-is-out/14794)
  - [ ] Pin the topic. You need admin privileges for that.
  - [ ] To the _early testers_ listed in [docs/EARLY_TESTERS.md](https://github.com/ipfs/go-ipfs/tree/master/docs/EARLY_TESTERS.md).  Do this by copy/pasting their GitHub usernames and checkboxes as a comment so they get a GitHub notification.  ([example](https://github.com/ipfs/go-ipfs/issues/8176#issuecomment-909356394))

Checklist:

- [ ] **Stage 0 - Automated Testing**
  - [ ] Upgrade to the latest patch release of Go that CircleCI has published
    - [ ] See the list here: https://hub.docker.com/r/cimg/go/tags
    - [ ] [ipfs/distributions](https://github.com/ipfs/distributions): bump [this version](https://github.com/ipfs/distributions/blob/master/.tool-versions#L2)
    - [ ] [ipfs/kubo](https://github.com/ipfs/kubo): [example PR](https://github.com/ipfs/kubo/pull/8599)
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
  - [ ] Infrastructure Testing. Open an issue against https://github.com/protocol/bifrost-infra like https://github.com/protocol/bifrost-infra/issues/2046 but spell out all that we want (gateways, bootstrapper, and cluster)
    - [ ] Deploy new version to a subset of Bootstrappers
    - [ ] Deploy new version to a subset of Gateways
    - [ ] Deploy new version to a subset of Preload nodes
    - [ ] Collect metrics every day. Work with the Infrastructure team to learn of any hiccup
  - [ ] IPFS Application Testing -  Run the tests of the following applications:
    - [ ] [IPFS Desktop](https://github.com/ipfs-shipyard/ipfs-desktop)
      - [ ] Ensure the RC is published to [the NPM package](https://www.npmjs.com/package/go-ipfs?activeTab=versions) ([happens automatically, just wait for CI](https://github.com/ipfs/npm-go-ipfs/actions))
      - [ ] Upgrade to the RC in [ipfs-desktop](https://github.com/ipfs-shipyard/ipfs-desktop) and push to a branch ([example](https://github.com/ipfs/ipfs-desktop/pull/1826/commits/b0a23db31ce942b46d95965ee6fe770fb24d6bde)), and open a draft PR to track through the final release ([example](https://github.com/ipfs/ipfs-desktop/pull/1826))
      - [ ] Ensure CI tests pass, repeat for new RCs
    - [ ] [IPFS Companion](https://github.com/ipfs-shipyard/ipfs-companion)
      - Start kubo daemon of the version to release.
      - Start a fresh chromium or chrome instance using `chromium --user-data-dir=$(mktemp -d)`
      - Start a fresh firefox instance using `firefox --profile $(mktemp -d)`
- Install IPFS Companion from [vendor-specific store](https://github.com/ipfs/ipfs-companion/#readme).
      - Check that the comunication between Kubo daemon and IPFS companion is working properly checking if the number of connected peers changes.
- [ ] **Stage 2 - Community Prod Testing**
  - [ ] Documentation
    - [ ] Ensure that [CHANGELOG.md](https://github.com/ipfs/go-ipfs/tree/master/CHANGELOG.md) is up to date
    - [ ] Ensure that [README.md](https://github.com/ipfs/go-ipfs/tree/master/README.md)  is up to date
    - [ ] Update docs by merging the auto-created PR in https://github.com/ipfs/ipfs-docs/pulls (they are auto-created every 12 hours) (only for final releases, not RCs)
  - [ ] Invite the wider community through (link to the release issue):
    - [ ] [discuss.ipfs.io](https://discuss.ipfs.io/c/announcements)
    - [ ] Matrix
- [ ] **Stage 3 - Release**
  - [ ] Final preparation
    - [ ] Verify that version string in [`version.go`](https://github.com/ipfs/go-ipfs/tree/master/version.go) has been updated.
    - [ ] Open a PR merging `release-vX.Y.Z` into the `release` branch.
      - This should be reviewed by the person who most recently released a version of `go-ipfs`.
	  - Use a merge commit (no rebase, no squash)
    - [ ] Prepare the command to use for tagging the merge commit (on the `release` branch) with `vX.Y.Z`.
      - Use `git tag -s` to ensure the tag is signed
    - [ ] Have the tagging command reviewed by the person who most recently released a version of `go-ipfs`
      - This is a dangerous operation, as it is difficult to reverse due to Go modules and automated Docker image publishing
    - [ ] Push the tag
      - Use `git push origin <tag>`
      - DO NOT USE `git push --tags`, as it will push ALL of your local tags
      - This should initiate a Docker build in GitHub Actions that publishes a `vX.Y.Z` tagged Docker image to DockerHub
    - [ ] Release published
      - [ ] to [dist.ipfs.tech](https://dist.ipfs.tech)
      - [ ] to [npm-go-ipfs](https://www.npmjs.com/package/go-ipfs) (done by CI at [ipfs/npm-go-ipfs](https://github.com/ipfs/npm-go-ipfs), but ok to dispatch [this job](https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml) manually)
      - [ ] to [chocolatey](https://chocolatey.org/packages/go-ipfs) (done by CI at [ipfs/choco-go-ipfs](https://github.com/ipfs/choco-go-ipfs/), but ok to dispatch [this job](https://github.com/ipfs/choco-go-ipfs/actions/workflows/main.yml) manually)
         - [ ] Manually run [the release workflow](https://github.com/ipfs/choco-go-ipfs/actions/workflows/main.yml)
         - [ ] Wait for Chocolatey to approve the release (usually takes a few hours)
      - [ ] to [snap](https://snapcraft.io/ipfs) (done CI at [snap/snapcraft.yaml](https://github.com/ipfs/kubo/blob/master/snap/snapcraft.yaml))
      - [ ] to [github](https://github.com/ipfs/go-ipfs/releases)
        - [ ] After publishing the GitHub release, run the workflow to attach the release assets: https://github.com/ipfs/go-ipfs/actions/workflows/sync-release-assets.yml
      - [ ] to [arch](https://www.archlinux.org/packages/community/x86_64/go-ipfs/) (flag it out of date)
    - [ ] Cut a new ipfs-desktop release
  - [ ] Get a blog post created 
    - [Submit a request using this form](https://airtable.com/shrNH8YWole1xc70I).
    - Notify marketing in #shared-pl-marketing-requests about the blog entry request (since the form gets spam).
    - Don't mark this as done until the blog entry is live.
  - [ ] Broadcasting (link to blog post)
    - [ ] Twitter (request in Filecoin Slack channel #shared-pl-marketing-requests)
    - [ ] [Reddit](https://reddit.com/r/ipfs)
    - [ ] [discuss.ipfs.io](https://discuss.ipfs.io/c/announcements)
      - A bot auto-posts this to Discord and Matrix
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

< changelog generated by bin/mkreleaselog > (add it to a separated comment if it is too big)

### ❤️ Contributors

< list generated by bin/mkreleaselog >

Would you like to contribute to the IPFS project and don't know how? Well, there are a few places you can get started:

- Check the issues with the `help wanted` label in the [ipfs/kubo repo](https://github.com/ipfs/kubo/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22)
- Join an IPFS All Hands, introduce yourself and let us know where you would like to contribute - https://github.com/ipfs/team-mgmt/#weekly-ipfs-all-hands
- Hack with IPFS and show us what you made! The All Hands call is also the perfect venue for demos, join in and show us what you built
- Join the discussion at [discuss.ipfs.io](https://discuss.ipfs.io/) and help users finding their answers.
- Join the [🚀 IPFS Core Implementations Weekly Sync 🛰](https://github.com/ipfs/team-mgmt/issues/992) and be part of the action!
