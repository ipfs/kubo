<!-- Last updated during [v0.18.0 release](https://github.com/ipfs/kubo/issues/9417) -->

# Items to do upon creating the release issue
- [ ] Fill in the Meta section
- [ ] Assign the issue to the release owner and reviewer.
- [ ] Name the issue "Release vX.Y.Z"
- [ ] Set the proper values for X.Y.Z
- [ ] Pin the issue

# Meta
* Release owner: @who
* Release reviewer: @who
* Expected RC date: week of YYYY-MM-DD
* 🚢 Expected final release date: YYYY-MM-DD
* Accompanying PR for improving the release process: (example: https://github.com/ipfs/kubo/pull/9391)

See the [Kubo release process](https://pl-strflt.notion.site/Kubo-Release-Process-5a5d066264704009a28a79cff93062c4) for more info.

# Kubo X.Y.Z Release

We're happy to announce Kubo X.Y.Z!

As usual, this release includes important fixes, some of which may be critical for security. Unless the fix addresses a bug being exploited in the wild, the fix will _not_ be called out in the release notes. Please make sure to update ASAP. See our [security fix policy](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md#security-fix-policy) for details.

## 🗺 What's left for release

<List of items with PRs and/or Issues to be considered for this release>

### Required

### Nice to have

## 🔦 Highlights

< top highlights for this release notes. For ANY version (final or RCs) >

## ✅ Release Checklist

### Labels

If an item should be executed for a specific release type, it should be labeled with one of the following labels:

- ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) execute **ONLY** when releasing a Release Candidate
- ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) execute **ONLY** when releasing a Final Release

Otherwise, it means it should be executed for **ALL** release types.

Patch releases should follow the same process as `.0` releases. If some item should **NOT** be executed for a Patch Release, it should be labeled with:

- ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) do **NOT** execute when releasing a Patch Release

### Before the release

This section covers tasks to be done ahead of the release.

- [ ] Verify you have access to all the services and tools required for the release
  - [ ] [GPG signature](https://docs.github.com/en/authentication/managing-commit-signature-verification) configured in local git and in GitHub
  - [ ] [admin access to IPFS Discourse](https://discuss.ipfs.tech/g/admins)
    - ask the previous release owner (or @2color) for an invite
  - [ ] ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) [access to #shared-pl-marketing-requests](https://filecoinproject.slack.com/archives/C018EJ8LWH1) channel in FIL Slack
    - ask the previous release owner for an invite
  - [ ] [access to IPFS network metrics](https://github.com/protocol/pldw/blob/624f47cf4ec14ad2cec6adf601a9f7b203ef770d/docs/sources/ipfs.md#ipfs-network-metrics) dashboards in Grafana
    - open an access request in the [pldw](https://github.com/protocol/pldw/issues/new/choose)
    - [example](https://github.com/protocol/pldw/issues/158)
  - [ ] [kuboreleaser](https://github.com/ipfs/kuboreleaser) checked out on your system (_only if you're using [kuboreleaser](https://github.com/ipfs/kuboreleaser)_)
  - [ ] [Thunderdome](https://github.com/ipfs-shipyard/thunderdome) checked out on your system and configured (see the [Thunderdome release docs](./releases_thunderdome.md) for setup)
  - [ ] [docker](https://docs.docker.com/get-docker/) installed on your system (_only if you're using [kuboreleaser](https://github.com/ipfs/kuboreleaser)_)
  - [ ] [npm](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm) installed on your system (_only if you're **NOT** using [kuboreleaser](https://github.com/ipfs/kuboreleaser)_)
  - [ ] [zsh](https://github.com/ohmyzsh/ohmyzsh/wiki/Installing-ZSH#install-and-set-up-zsh-as-default) installed on your system
  - [ ] [kubo](https://github.com/ipfs/kubo) checked out under `$(go env GOPATH)/src/github.com/ipfs/kubo`
    - you can also symlink your clone to the expected location by running `mkdir -p $(go env GOPATH)/src/github.com/ipfs && ln -s $(pwd) $(go env GOPATH)/src/github.com/ipfs/kubo`
  - [ ] ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) [Reddit](https://www.reddit.com) account
- ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) Upgrade Go used in CI to the latest patch release available in [CircleCI](https://hub.docker.com/r/cimg/go/tags) in:
  - [ ] ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) [ipfs/distributions](https://github.com/ipfs/distributions)
    - [example](https://github.com/ipfs/distributions/pull/756)
  - [ ] ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) [ipfs/ipfs-docs](https://github.com/ipfs/ipfs-docs)
    - [example](https://github.com/ipfs/ipfs-docs/pull/1298)
- [ ] Verify there is nothing [left for release](-what-s-left-for-release)
- [ ] Create a release process improvement PR
  - [ ] update the [release issue template](docs/RELEASE_ISSUE_TEMPLATE.md) as you go
  - [ ] link it in the [Meta](#meta) section

### The release

This section covers tasks to be done during each release.

- [ ] Prepare the release branch and update version numbers accordingly <details><summary>using `./kuboreleaser --skip-check-before release --version vX.Y.Z(-rcN) prepare-branch` or ...</summary>
  - [ ] create a new branch `release-vX.Y.Z`
    - use `master` as base if `Z == 0`
    - use `release` as base if `Z > 0`
  - [ ] ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) update the `CurrentVersionNumber` in [version.go](version.go) in the `master` branch to `vX.Y+1.0-dev`
    - [example](https://github.com/ipfs/kubo/pull/9305)
  - [ ] update the `CurrentVersionNumber` in [version.go](version.go) in the `release-vX.Y` branch to `vX.Y.Z(-RCN)`
    - [example](https://github.com/ipfs/kubo/pull/9394)
  - [ ] create a draft PR from `release-vX.Y` to `release`
    - [example](https://github.com/ipfs/kubo/pull/9306)
  - [ ] Cherry-pick commits from `master` to the `release-vX.Y.Z` using `git cherry-pick -x <commit>`
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Add full changelog and contributors to the [changelog](docs/changelogs/vX.Y.md)
    - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Replace the `Changelog` and `Contributors` sections of the [changelog](docs/changelogs/vX.Y.md) with the stdout of `./bin/mkreleaselog`
      - do **NOT** copy the stderr
  - [ ] verify all CI checks on the PR from `release-vX.Y` to `release` are passing
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Merge the PR from `release-vX.Y` to `release` using the `Create a merge commit`
    - do **NOT** use `Squash and merge` nor `Rebase and merge` because we need to be able to sign the merge commit
    - do **NOT** delete the `release-vX.Y` branch
  </details>
- [ ] Run Thunderdome testing, see the [Thunderdome release docs](./releases_thunderdome.md) for details
  - [ ] create a PR and merge the experiment config into Thunderdome
- [ ] Create the release tag <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) tag` or ...</summary>
  - This is a dangerous operation! Go and Docker publishing are difficult to reverse! Have the release reviewer verify all the commands marked with ⚠️!
  - [ ] ⚠️ ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) tag the HEAD commit using `git tag -s vX.Y.Z(-RCN) -m 'Prerelease X.Y.Z(-RCN)'`
  - [ ] ⚠️ ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) tag the HEAD commit of the `release` branch using `git tag -s vX.Y.Z(-RCN) -m 'Release X.Y.Z(-RCN)'`
  - [ ] ⚠️ verify the tag is signed and tied to the correct commit using `git show vX.Y.Z(-RCN)`
  - [ ] ⚠️ push the tag to GitHub using `git push origin vX.Y.Z(-RCN)`
    - do **NOT** use `git push --tags` because it pushes all your local tags
  </details>
- [ ] Publish the release to [DockerHub](https://hub.docker.com/r/ipfs/kubo/) <details><summary>using `./kuboreleaser --skip-check-before --skip-run release --version vX.Y.Z(-rcN) publish-to-dockerhub` or ...</summary>
  - [ ] Wait for [Publish docker image](https://github.com/ipfs/kubo/actions/workflows/docker-image.yml) workflow run initiated by the tag push to finish
  - [ ] verify the image is available on [Docker Hub](https://hub.docker.com/r/ipfs/kubo/tags)
- [ ] Publish the release to [dist.ipfs.tech](https://dist.ipfs.tech) <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) publish-to-distributions` or ...</summary>
  - [ ] check out [ipfs/distributions](https://github.com/ipfs/distributions)
  - [ ] run `./dist.sh add-version kubo vX.Y.Z(-RCN)` to add the new version to the `versions` file
    - [usage](https://github.com/ipfs/distributions#usage)
  - [ ] create and merge the PR which updates `dists/kubo/versions` and `dists/go-ipfs/versions` (![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) and `dists/kubo/current_version` and `dists/go-ipfs/current_version`)
    - [example](https://github.com/ipfs/distributions/pull/760)
  - [ ] wait for the [CI](https://github.com/ipfs/distributions/actions/workflows/main.yml) workflow run initiated by the merge to master to finish
  - [ ] verify the release is available on [dist.ipfs.io](https://dist.ipfs.io/#kubo)
  </details>
- [ ] Publish the release to [NPM](https://www.npmjs.com/package/go-ipfs?activeTab=versions) <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) publish-to-npm` (⚠️ you might need to run the command a couple of times because GHA might not be able to see the new distribution straight away due to caching) or ...</summary>
  - [ ] run the [Release to npm](https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml) workflow
  - [ ] check [Release to npm](https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml) workflow run logs to verify it discovered the new release
  - [ ] verify the release is available on [NPM](https://www.npmjs.com/package/go-ipfs?activeTab=versions)
  </details>
- [ ] Publish the release to [GitHub](https://github.com/ipfs/kubo/releases) <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) publish-to-github` or ...</summary>
  - [ ] create a new release on [GitHub](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository#creating-a-release)
    - [RC example](https://github.com/ipfs/kubo/releases/tag/v0.17.0-rc1)
    - [FINAL example](https://github.com/ipfs/kubo/releases/tag/v0.17.0)
    - [ ] use the `vX.Y.Z(-RCN)` tag
    - [ ] link to the release issue
    - [ ] ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) link to the changelog in the description
    - [ ] ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) check the `This is a pre-release` checkbox
    - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) copy the changelog (without the header) in the description
    - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) do **NOT** check the `This is a pre-release` checkbox
  - [ ] run the [sync-release-assets](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml) workflow
  - [ ] wait for the [sync-release-assets](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml) workflow run to finish
  - [ ] verify the release assets are present in the [GitHub release](https://github.com/ipfs/kubo/releases/tag/vX.Y.Z(-RCN))
  </details>
- [ ] Promote the release <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) promote` or ...</summary>
  - [ ] create an [IPFS Discourse](https://discuss.ipfs.tech) topic
    - [prerelease example](https://discuss.ipfs.tech/t/kubo-v0-16-0-rc1-release-candidate-is-out/15248)
    - [release example](https://discuss.ipfs.tech/t/kubo-v0-16-0-release-is-out/15249)
    - [ ] use `Kubo vX.Y.Z(-RCN) is out!` as the title
    - [ ] use `kubo` and `go-ipfs` as topics
    - [ ] repeat the title as a heading (`##`) in the description
    - [ ] link to the GitHub Release, binaries on IPNS, docker pull command and release notes in the description
  - [ ] pin the [IPFS Discourse](https://discuss.ipfs.tech) topic globally
    - you can make the topic a banner if there is no banner already
  - verify the [IPFS Discourse](https://discuss.ipfs.tech) topic was copied to:
    - [ ] [#ipfs-chatter](https://discord.com/channels/669268347736686612/669268347736686615) in IPFS Discord
    - [ ] [#ipfs-chatter](https://filecoinproject.slack.com/archives/C018EJ8LWH1) in FIL Slack
    - [ ] [#ipfs-chatter:ipfs.io](https://matrix.to/#/#ipfs-chatter:ipfs.io) in Matrix
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Add the link to the [IPFS Discourse](https://discuss.ipfs.tech) topic to the [GitHub Release](https://github.com/ipfs/kubo/releases/tag/vX.Y.Z(-RCN)) description
    - [example](https://github.com/ipfs/kubo/releases/tag/v0.17.0)
  - [ ] ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) create an issue comment mentioning early testers on the release issue
    - [example](https://github.com/ipfs/kubo/issues/9319#issuecomment-1311002478)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) create an issue comment linking to the release on the release issue
    - [example](https://github.com/ipfs/kubo/issues/9417#issuecomment-1400740975)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) ask the marketing team to tweet about the release in [#shared-pl-marketing-requests](https://filecoinproject.slack.com/archives/C018EJ8LWH1) in FIL Slack
    - [example](https://filecoinproject.slack.com/archives/C018EJ8LWH1/p1664885305374900)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) post the link to the [GitHub Release](https://github.com/ipfs/kubo/releases/tag/vX.Y.Z(-RCN)) to [Reddit](https://reddit.com/r/ipfs)
    - [example](https://www.reddit.com/r/ipfs/comments/9x0q0k/kubo_v0160_release_is_out/)
  </details>
- [ ] Test the new version with `ipfs-companion` <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) test-ipfs-companion` or ...</summary>
  - [ ] run the [e2e](https://github.com/ipfs/ipfs-companion/actions/workflows/e2e.yml)
    - use `vX.Y.Z(-RCN)` as the Kubo image version
  - [ ] wait for the [e2e](https://github.com/ipfs/ipfs-companion/actions/workflows/e2e.yml) workflow run to finish
  </details>
- [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Update Kubo in [ipfs-desktop](https://github.com/ipfs/ipfs-desktop) <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) update-ipfs-desktop` or ...</summary>
  - [ ] check out [ipfs/ipfs-desktop](https://github.com/ipfs/ipfs-desktop)
  - [ ] run `npm install`
  - [ ] create a PR which updates `package.json` and `package-lock.json`
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) add @SgtPooki and @whizzzkid as reviewers
  </details>
- [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Update Kubo docs <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) update-ipfs-docs` or ...</summary>
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) run the [update-on-new-ipfs-tag.yml](https://github.com/ipfs/ipfs-docs/actions/workflows/update-on-new-ipfs-tag.yml) workflow
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) merge the PR created by the [update-on-new-ipfs-tag.yml](https://github.com/ipfs/ipfs-docs/actions/workflows/update-on-new-ipfs-tag.yml) workflow run
  </details>
- [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Ask Brave to update Kubo in Brave Desktop
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) use [this link](https://github.com/brave/brave-browser/issues/new?assignees=&labels=OS%2FDesktop&projects=&template=desktop.md&title=) to create an issue for the new Kubo version
    - [basic example](https://github.com/brave/brave-browser/issues/31453), [example with additional notes](https://github.com/brave/brave-browser/issues/27965)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) post link to the issue in `#shared-pl-brave` for visibility
- [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Create a blog entry on [blog.ipfs.tech](https://blog.ipfs.tech) <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) update-ipfs-blog --date YYYY-MM-DD` or ...</summary>
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) create a PR which adds a release note for the new Kubo version
    - [example](https://github.com/ipfs/ipfs-blog/pull/529)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) merge the PR
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) verify the blog entry was published
  </details>
- [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Merge the [release](https://github.com/ipfs/kubo/tree/release) branch back into [master](https://github.com/ipfs/kubo/tree/master), ignoring the changes to [version.go](version.go) (keep the `-dev`) version, <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) merge-branch` or ...</summary>
  - [ ] create a new branch `merge-release-vX.Y.Z` from `release`
  - [ ] create and merge a PR from `merge-release-vX.Y.Z` to `master`
  </details>
- [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) Prepare for the next release <details><summary>using `./kuboreleaser release --version vX.Y.Z(-rcN) prepare-next` or ...</summary>
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) Create the next [changelog](https://github.com/ipfs/kubo/blob/master/docs/changelogs/vX.(Y+1).md)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) Link to the new changelog in the [CHANGELOG.md](CHANGELOG.md) file
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) Create the next release issue
  </details>
- [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) Create a dependency update PR
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) check out [ipfs/kubo](https://github.com/ipfs/kubo)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) run `go get -u` in root directory
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) run `go mod tidy` in root directory
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) run `go mod tidy` in `docs/examples/kubo-as-a-library` directory
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) create a PR which updates `go.mod` and `go.sum`
  - [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-yellow?style=flat-square) add the PR to the next release milestone
- [ ] ![](https://img.shields.io/badge/only-FINAL-green?style=flat-square) Close the release issue

## How to contribute?

Would you like to contribute to the IPFS project and don't know how? Well, there are a few places you can get started:

- Check the issues with the `help wanted` label in the [ipfs/kubo repo](https://github.com/ipfs/kubo/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22)
- Join the discussion at [discuss.ipfs.tech](https://discuss.ipfs.tech/) and help users finding their answers.
- See other options at https://docs.ipfs.tech/community/
