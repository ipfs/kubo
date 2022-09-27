<!-- Last updated by @galargh during [0.16.0 release](https://github.com/ipfs/kubo/issues/9237) -->

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

Checklist:

- [ ] **Stage 0 - Prerequisites**
  - [ ] Ensure that the `What's left for release` section has all the checkboxes checked. If that's not the case, discuss the open items with Kubo maintainers and update the release schedule accordingly.
  - [ ] Create `docs-release-vX.Y.Z` branch, open a draft PR and keep updating `docs/RELEASE_ISSUE_TEMPLATE.md` on that branch as you go.
  - [ ] Ensure you have [GPG key generated]() and [added to your GitHub account](https://docs.github.com/en/authentication/managing-commit-signature-verification/adding-a-gpg-key-to-your-github-account). This will enable you to created signed tags.
  - [ ] Ensure you have [admin access](https://discuss.ipfs.tech/g/admins) to [IPFS Discourse](https://discuss.ipfs.tech/). Admin access is required to globally pin posts and create banners. @2color might be able to assist you.
  - [ ] Access to [#bifrost](https://filecoinproject.slack.com/archives/C03MMMF606T) channel in FIL Slack might come in handy. Ask the release reviewer to invite you over.
  - [ ] After the release is deployed to our internal infrastructure, you're going to need read access to [IPFS network metrics](https://github.com/protocol/pldw/blob/624f47cf4ec14ad2cec6adf601a9f7b203ef770d/docs/sources/ipfs.md#ipfs-network-metrics) dashboards. Open an access request in https://github.com/protocol/pldw/issues/new/choose if you don't have it yet ([example](https://github.com/protocol/pldw/issues/158)).
  - [ ] You're also going to need NPM installed on your system. See [here](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm) for instructions.
  - [ ] Prepare changelog proposal in [docs/changelogs/vX.Y.md](docs/changelogs).
    - Skip filling out the `### Changelog` section (the one where which lists all the commits and contributors) for now. We will populate it after the release branch is cut.
  - [ ] Install ZSH ([instructions](https://github.com/ohmyzsh/ohmyzsh/wiki/Installing-ZSH#install-and-set-up-zsh-as-default)). It is needed by the changelog creation script.
  - [ ] Ensure you have `kubo` checked out under `$(go env GOPATH)/src/github.com/ipfs/kubo`. This is required by the changelog creation script.
    - If you want your clone to live in a different location, you can symlink it to the expected location by running `mkdir -p $(go env GOPATH)/src/github.com/ipfs && ln -s $(pwd) $(go env GOPATH)/src/github.com/ipfs/kubo`.
- [ ] **Stage 1 - Initial Preparations**
  - [ ] Upgrade to the latest patch release of Go that CircleCI has published (currently used version: `1.19.1`)
    - [ ] See the list here: https://hub.docker.com/r/cimg/go/tags
    - [ ] [ipfs/distributions](https://github.com/ipfs/distributions): bump [this version](https://github.com/ipfs/distributions/blob/master/.tool-versions#L2)
    - [ ] [ipfs/kubo](https://github.com/ipfs/kubo): [example PR](https://github.com/ipfs/kubo/pull/8599)
  - [ ] Fork a new branch (`release-vX.Y.Z`) from `master`.
  - [ ] Bump the version in `version.go` in the `master` branch to `vX.(Y+1).0-dev`.
- [ ] **Stage 2 - Release Candidate** - _if any [non-trivial](docs/releases.md#footnotes) changes need to be included in the release, return to this stage_
  - [ ] Bump the version in `version.go` in the `release-vX.Y.Z` branch to `vX.Y.Z-rcN`.
  - [ ] If applicable, add new commits to the `release-vX.Y.Z` branch from `master` using `git cherry-pick -x ...`
      - Note: `release-*` branches are protected. You can do all needed updates on a separated branch (e.g. `wip-release-vX.Y.Z`) and when everything is settled push to `release-vX.Y.Z`
  - [ ] Update the [docs/changelogs/vX.Y.md](docs/changelogs) with the new commits and contributors.
    - [ ] Run `./bin/mkreleaselog` twice to generate the changelog and copy the output.
      - The first run of the script might be poluted with `git clone` output.
    - [ ] Paste the output into the `### Changelog` section of the changelog file inside the `<details><summary></summary></details>` block.
    - [ ] Commit the changelog changes.
  - [ ] Push the `release-vX.Y.Z` branch to GitHub (`git push origin release-vX.Y.Z`) and create a draft PR from that branch if it doesn't exist yet ([example](https://github.com/ipfs/kubo/pull/9306)).
  - [ ] Wait for CI to run and complete PR checks. All checks should pass.
  - [ ] Tag HEAD `release-vX.Y.Z` commit with `vX.Y.Z-rcN` (`git tag -s vX.Y.Z-rcN`, use "Pre-release 0.15.0-rc1" as the tag message)
  - [ ] Push the `vX.Y.Z-rcN` tag to GitHub (`git push origin vX.Y.Z-rcN`; DO NOT USE `git push --tags` because it pushes all your local tags).
  - [ ] Add artifacts to https://dist.ipfs.tech by making a PR against [ipfs/distributions](https://github.com/ipfs/distributions)
    - [ ] Clone the `ipfs/distributions` repo locally.
    - [ ] Create a new branch (`kubo-release-vX.Y.Z-rcn`) from `master`.
    - [ ] Run `./dist.sh add-version kubo vX.Y.Z-rcN` to add the new version to the `versions` file ([instructions](https://github.com/ipfs/distributions#usage)).
      - If you're adding a new RC, `dist.sh` will print _WARNING: not marking pre-release kubo vX.Y.Z-rc1n as the current version._.
    - [ ] Push the `kubo-release-vX.Y.Z-rcn` branch to GitHub and create a PR from that branch ([example](https://github.com/ipfs/distributions/pull/760)).
    - [ ] Wait for PR to build artifacts and generate diff (~30min)
    - [ ] Inspect results, merge if CI is green and the diff looks ok
    - [ ] Wait for `master` branch to build. It will automatically update DNSLink at https://dist.ipfs.tech (~30min)
  - [ ] Cut a pre-release on [GitHub](https://github.com/ipfs/kubo/releases) ([instructions](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository#creating-a-release), [example](https://github.com/ipfs/kubo/releases/tag/v0.16.0-rc1))
    - Use `vX.Y.Z-rcN` as the tag.
    - Link to the release issue in the description.
    - Link to the relevant [changelog](https://github.com/ipfs/kubo/blob/master/docs/changelogs/) in the description.
    - Check `This is a pre-release`.
  - [ ] Synchronize release artifacts by running [sync-release-assets](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml) workflow.
  - [ ] Announce the RC
    - [ ] Create a new post on [IPFS Discourse](https://discuss.ipfs.tech). ([example](https://discuss.ipfs.tech/t/kubo-v0-16-0-rc1-release-candidate-is-out/15248))
      - Use `Kubo vX.Y.Z-rcn Release Candidate is out!` as the title.
      - Use `kubo` and `go-ipfs` as topics.
      - Repeat the title as a heading (`##`) in the description.
      - Link to the release issue in the description.
    - [ ] Pin the topic globally so that it stays at the top of the category.
    - [ ] If there is no more important banner currently set on Discourse (e.g. IPFS Camp announcement), make the topic into a banner.
    - [ ] Check if Discourse post was automatically copied to:
      - [ ] IPFS Discord #ipfs-chatter
      - [ ] FIL Slack #ipfs-chatter
      - [ ] Matrix
    - [ ] Mention [early testers](https://github.com/ipfs/go-ipfs/tree/master/docs/EARLY_TESTERS.md) in the comment under the release issue ([example](https://github.com/ipfs/kubo/issues/9237#issuecomment-1258072509)).
- [ ] **Stage 3 - Internal Testing**
  - [ ] Infrastructure Testing.
    - [ ] Open an issue against [bifrost-infra](https://github.com/protocol/bifrost-infra) ([example](https://github.com/protocol/bifrost-infra/issues/2109)).
      - Spell out all that we want updated - gateways, the bootstraper and the cluster/preload nodes
      - Mention @protocol/bifrost-team in the issue if you opened it prior to the release
      - [Optional] Reply under a message about the issue in the #bifrost channel on FIL Slack once the RC is out. Send the message to the channel.
      - [ ] Check [metrics](https://protocollabs.grafana.net/d/8zlhkKTZk/gateway-slis-precomputed?orgId=1) every day.
        - Compare the metrics trends week over week.
        - If there is an unexpected variation in the trend, message the #bifrost channel on FIL Slack and ask for help investigation the cause.
  - [ ] IPFS Application Testing.
    - [ ] [IPFS Desktop](https://github.com/ipfs-shipyard/ipfs-desktop)
      - [ ] Ensure the RC is published to [the NPM package](https://www.npmjs.com/package/go-ipfs?activeTab=versions) ([happens automatically, just wait for CI](https://github.com/ipfs/npm-go-ipfs/actions)), you can run https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml to force release new version
      - [ ] Upgrade to the RC in [ipfs-desktop](https://github.com/ipfs-shipyard/ipfs-desktop) and push to a branch ([example](https://github.com/ipfs/ipfs-desktop/pull/1826/commits/b0a23db31ce942b46d95965ee6fe770fb24d6bde)), and open a draft PR to track through the final release ([example](https://github.com/ipfs/ipfs-desktop/pull/1826)), you have to check out the repo to update package-lock.json
      - [ ] Ensure CI tests pass, repeat for new RCs
    - [ ] [IPFS Companion](https://github.com/ipfs-shipyard/ipfs-companion)
      - Start kubo daemon of the version to release.
      - Start a fresh chromium or chrome instance using `chromium --user-data-dir=$(mktemp -d)` (macos `/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --user-data-dir=$(mktemp -d)`)
      - Start a fresh firefox instance using `firefox --profile $(mktemp -d)` (macos `/Applications/Firefox.app/Contents/MacOS/firefox --profile $(mktemp -d)`)
      - Install IPFS Companion from [vendor-specific store](https://github.com/ipfs/ipfs-companion/#readme).
      - Check that the comunication between Kubo daemon and IPFS companion is working properly checking if the number of connected peers changes.
    - [ ] [interop](https://github.com/ipfs/interop)
      - [ ] Clone the `ipfs/interop` repo locally.
      - [ ] Create a new branch (`kubo-release-vX.Y.Z-rcn`) from `master`.
      - [ ] Update `go-ipfs` version to `vX.Y.Z-rcN` in [package.json](https://github.com/ipfs/interop/blob/master/package.json).
      - [ ] Run `npm install` locally
      - [ ] Push the `kubo-release-vX.Y.Z-rcn` branch to GitHub and create a draft PR from that branch ([example](https://github.com/ipfs/interop/pull/511)).
    - [ ] [go-ipfs-api](https://github.com/ipfs/go-ipfs-api)
      - [ ] Create a branch with kubo version pinned in the [test setup action](https://github.com/ipfs/go-ipfs-api/blob/master/.github/actions/go-test-setup/action.yml) ([example](https://github.com/ipfs/go-ipfs-api/commit/26aeb9075240e595d4a095a4e803767171eec06c)).
      - [ ] Ensure that CI is green.
      - [ ] Delete the branch.
    - [ ] [go-ipfs-http-client](https://github.com/ipfs/go-ipfs-http-client)
      - [ ] Create a branch with kubo version pinned in the [test setup action](https://github.com/ipfs/go-ipfs-http-client/blob/master/.github/actions/go-test-setup/action.yml) ([example](https://github.com/ipfs/go-ipfs-http-client/commit/940886ca500e567c42660122f405432579ecad90)).
      - [ ] Ensure that CI is green.
      - [ ] Delete the branch.
    - [ ] [WebUI](https://github.com/ipfs-shipyard/ipfs-webui)
      - [ ] Run [CI workflow](https://github.com/ipfs/ipfs-webui/actions/workflows/ci.yml) with `vX.Y.Z-rcN` for the `kubo-version` input.
      - [ ] Ensure that CI is green.
- [ ] **Stage 4 - Community Prod Testing** - _ONLY FOR FINAL RELEASE_
	  - [ ] Add a link from release notes to Discuss post (like we did here: https://github.com/ipfs/kubo/releases/tag/v0.15.0 )
	  - [ ] Keep the release notes as trim as possible (removing some of the top headers, like we did here: https://github.com/ipfs/kubo/releases/tag/v0.15.0 )
    - [ ] Ensure that [README.md](https://github.com/ipfs/go-ipfs/tree/master/README.md)  is up to date
    - [ ] Update docs by merging the auto-created PR in https://github.com/ipfs/ipfs-docs/pulls (they are auto-created every 12 hours) (only for final releases, not RCs)
- [ ] **Stage 5 - Release** - _ONLY FOR FINAL RELEASE_
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
- [ ] **Stage 6 - Post-Release**
  - [ ] Merge the `release` branch back into `master`, ignoring the changes to `version.go` (keep the `-dev` version from master).
  - [ ] Create an issue using this release issue template for the _next_ release.
  - [ ] Make sure any last-minute changelog updates from the blog post make it back into the CHANGELOG.
  - [ ] Mark PR draft created for IPFS Desktop as ready for review.
  - [ ] Mark PR draft created from `docs-release-vX.Y.Z` as ready for review.

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
