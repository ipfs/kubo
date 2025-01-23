<!-- Last updated during [v0.33.0 release](https://github.com/ipfs/kubo/issues/10547) -->

# ✅ Release Checklist (vX.Y.Z[-rcN])

## Labels

If an item should be executed only for a specific release type, it is labeled with:

- ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) execute **ONLY** when releasing a Release Candidate
- ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) execute **ONLY** when releasing a Final Release
- ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) do **NOT** execute when releasing a Patch Release

Otherwise, it means a step should be executed for **ALL** release types.

## Before the release

This section covers tasks to be done ahead of the release.

- [ ] Verify you have access to all the services and tools required for the release
  - [ ] [GPG signature](https://docs.github.com/en/authentication/managing-commit-signature-verification) configured in local git and in GitHub
  - [ ] [docker](https://docs.docker.com/get-docker/) installed on your system
  - [ ] [npm](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm) installed on your system
  - [ ] [kubo](https://github.com/ipfs/kubo) checked out under `$(go env GOPATH)/src/github.com/ipfs/kubo`
    - you can also symlink your clone to the expected location by running `mkdir -p $(go env GOPATH)/src/github.com/ipfs && ln -s $(pwd) $(go env GOPATH)/src/github.com/ipfs/kubo`
- ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) Upgrade Go used in CI to the latest patch release available at <https://go.dev/dl/>

## The release

This section covers tasks to be done during each release.

### 1. Prepare release branch

- [x] Prepare the release branch and update version numbers accordingly 
  - [ ] create a new branch `release-vX.Y.Z`
    - use `master` as base if `Z == 0`
    - use `release` as base if `Z > 0`
  - [ ] ![](https://img.shields.io/badge/only-RC1-blue?style=flat-square) update the `CurrentVersionNumber` in [version.go](version.go) in the `master` branch to `vX.Y+1.0-dev` ([example](https://github.com/ipfs/kubo/pull/9305))
  - [x] update the `CurrentVersionNumber` in [version.go](version.go) in the `release-vX.Y` branch to `vX.Y.Z(-rcN)`  ([example](https://github.com/ipfs/kubo/pull/9394))
  - [ ] create a draft PR from `release-vX.Y.Z` to `release` ([example](https://github.com/ipfs/kubo/pull/9306))
  - [x] Cherry-pick commits from `master` to the `release-vX.Y.Z` using `git cherry-pick -x <commit>`  ([example](https://github.com/ipfs/kubo/pull/10636/commits/033de22e3bc6191dbb024ad6472f5b96b34e3ccf))
    - **NOTE:** cherry-picking with `-x` is important
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Add full changelog and contributors to the [changelog](docs/changelogs/vX.Y.md)
    - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Replace the `Changelog` and `Contributors` sections of the [changelog](docs/changelogs/vX.Y.md) with the stdout of `./bin/mkreleaselog`.
      - Note that the command expects your `$GOPATH/src/github.com/ipfs/kubo` to include latest commits from `release-vX.Y.Z`
      - do **NOT** copy the stderr
  - [x] verify all CI checks on the PR from `release-vX.Y.Z` to `release` are passing
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Merge the PR from `release-vX.Y.Z` to `release` using the `Create a merge commit`
    - do **NOT** use `Squash and merge` nor `Rebase and merge` because we need to be able to sign the merge commit
    - do **NOT** delete the `release-vX.Y.Z` branch

### 2. Tag release

- [x] Create the release tag 
  - ⚠️ **NOTE:** This is a dangerous operation! Go and Docker publishing are difficult to reverse! Have the release reviewer verify all the commands marked with !
  - [x]  ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) tag the HEAD commit using `git tag -s vX.Y.Z(-rcN) -m 'Prerelease X.Y.Z(-rcN)'`
  - [ ]  ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) tag the HEAD commit of the `release` branch using `git tag -s vX.Y.Z -m 'Release X.Y.Z'`
  - [x]  ⚠️ verify the tag is signed and tied to the correct commit using `git show vX.Y.Z(-rcN)`
  - [x]  push the tag to GitHub using `git push origin vX.Y.Z(-rcN)`
    - ⚠️ do **NOT** use `git push --tags` because it pushes all your local tags

### 3. Publish

- [x] Publish Docker image to [DockerHub](https://hub.docker.com/r/ipfs/kubo/tags)
  - [x] Wait for [Publish docker image](https://github.com/ipfs/kubo/actions/workflows/docker-image.yml) workflow run initiated by the tag push to finish
  - [x] verify the image is available on [Docker Hub → tags](https://hub.docker.com/r/ipfs/kubo/tags)
- [x] Publish the release to [dist.ipfs.tech](https://dist.ipfs.tech) 
  - [ ] check out [ipfs/distributions](https://github.com/ipfs/distributions)
  - [ ] create new branch: run `git checkout -b release-kubo-X.Y.Z(-rcN)` 
  - [x] run `./dist.sh add-version kubo vX.Y.Z(-rcN)` to add the new version to the `versions` file ([usage](https://github.com/ipfs/distributions#usage))
  - [x] Verify [ipfs/distributions](https://github.com/ipfs/distributions)'s `.tool-versions`'s `golang` entry is set to the [latest go release](https://go.dev/doc/devel/release) on the major go branch [Kubo is being tested on](https://github.com/ipfs/kubo/blob/master/.github/workflows/gotest.yml) (see `go-version:`).  If not, update `.tool-versions` to match the latest golang.
  - [x] create and merge the PR which updates `dists/kubo/versions` and `dists/go-ipfs/versions` (![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) and `dists/kubo/current` and `dists/go-ipfs/current`)
    - [example](https://github.com/ipfs/distributions/pull/760)
  - [x] wait for the [CI](https://github.com/ipfs/distributions/actions/workflows/main.yml) workflow run initiated by the merge to master to finish
  - [x] verify the release is available on [dist.ipfs.tech](https://dist.ipfs.tech/#kubo)
- [x] Publish the release to [NPM](https://www.npmjs.com/package/kubo?activeTab=versions)
  - [x] manually dispatch the [Release to npm](https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml) workflow
  - [x] check [Release to npm](https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml) workflow run logs to verify it discovered the new release
  - [x] verify the release is available on [NPM](https://www.npmjs.com/package/kubo?activeTab=versions)
- [x] Publish the release to [GitHub kubo/releases](https://github.com/ipfs/kubo/releases)
  - [x] create a new release on [github.com/ipfs/kubo/releases](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository#creating-a-release)
    - [RC example](https://github.com/ipfs/kubo/releases/tag/v0.17.0-rc1)
    - [FINAL example](https://github.com/ipfs/kubo/releases/tag/v0.17.0)
    - [ ] use the `vX.Y.Z(-rcN)` tag
    - [ ] link to the release issue
    - [x] ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) link to the changelog in the description
    - [x] ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) check the `This is a pre-release` checkbox
    - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) copy the changelog (without the header) in the description
    - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) do **NOT** check the `This is a pre-release` checkbox
  - [x] run the [sync-release-assets](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml) workflow
  - [x] wait for the [sync-release-assets](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml) workflow run to finish
  - [x] verify the release assets are present in the [GitHub release](https://github.com/ipfs/kubo/releases/tag/vX.Y.Z(-rcN))

### 4. After Publishing

- [x] Update Kubo staging environment, see the [Running Kubo tests on staging](https://www.notion.so/Running-Kubo-tests-on-staging-488578bb46154f9bad982e4205621af8) for details.
  - [x] ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) Test last release against the current RC
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Test last release against the current one
- [x] Promote the release  
  - [x] create an [IPFS Discourse](https://discuss.ipfs.tech) topic ([prerelease example](https://discuss.ipfs.tech/t/kubo-v0-16-0-rc1-release-candidate-is-out/15248), [release example](https://discuss.ipfs.tech/t/kubo-v0-16-0-release-is-out/15249))
    - [x] use `Kubo vX.Y.Z(-rcN) is out!` as the title and `kubo` and `go-ipfs` as tags
    - [x] repeat the title as a heading (`##`) in the description
    - [x] link to the GitHub Release, binaries on IPNS, docker pull command and release notes in the description
    - [x] pin the [IPFS Discourse](https://discuss.ipfs.tech) topic globally, you can make the topic a banner if there is no banner already
  - [ ] verify the [IPFS Discourse](https://discuss.ipfs.tech) topic was copied to:
    - [x] [#ipfs-chatter](https://discord.com/channels/669268347736686612/669268347736686615) in IPFS Discord
    - [ ] [#ipfs-chatter](https://filecoinproject.slack.com/archives/C018EJ8LWH1) in FIL Slack
    - [ ] [#ipfs-chatter:ipfs.io](https://matrix.to/#/#ipfs-chatter:ipfs.io) in Matrix
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Add the link to the [IPFS Discourse](https://discuss.ipfs.tech) topic to the [GitHub Release](https://github.com/ipfs/kubo/releases/tag/vX.Y.Z(-rcN)) description ([example](https://github.com/ipfs/kubo/releases/tag/v0.17.0))
  - [x] ![](https://img.shields.io/badge/only-RC-blue?style=flat-square) create an issue comment mentioning early testers on the release issue ([example](https://github.com/ipfs/kubo/issues/9319#issuecomment-1311002478))
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) create an issue comment linking to the release on the release issue ([example](https://github.com/ipfs/kubo/issues/9417#issuecomment-1400740975))
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) ask the marketing team to tweet about the release in [#shared-pl-marketing-requests](https://filecoinproject.slack.com/archives/C018EJ8LWH1) in FIL Slack ([example](https://filecoinproject.slack.com/archives/C018EJ8LWH1/p1664885305374900))
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) post the link to the [GitHub Release](https://github.com/ipfs/kubo/releases/tag/vX.Y.Z(-rcN)) to [Reddit](https://reddit.com/r/ipfs) ([example](https://www.reddit.com/r/ipfs/comments/9x0q0k/kubo_v0160_release_is_out/))
- [x] Manually smoke-test the new version with [IPFS Companion Browser Extension](https://docs.ipfs.tech/install/ipfs-companion/)
- [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Update Kubo in [ipfs-desktop](https://github.com/ipfs/ipfs-desktop) 
  - [ ] check out [ipfs/ipfs-desktop](https://github.com/ipfs/ipfs-desktop)
  - [ ] run `npm install `
  - [ ] create a PR which updates `package.json` and `package-lock.json` 
- [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Update Kubo docs  at docs.ipfs.tech:
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) run the [update-on-new-ipfs-tag.yml](https://github.com/ipfs/ipfs-docs/actions/workflows/update-on-new-ipfs-tag.yml) workflow
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) merge the PR created by the [update-on-new-ipfs-tag.yml](https://github.com/ipfs/ipfs-docs/actions/workflows/update-on-new-ipfs-tag.yml) workflow run
  </details>
- [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Create a blog entry on [blog.ipfs.tech](https://blog.ipfs.tech) 
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) create a PR which adds a release note for the new Kubo version ([example](https://github.com/ipfs/ipfs-blog/pull/529))
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) merge the PR
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) verify the blog entry was published
- [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Merge the [release](https://github.com/ipfs/kubo/tree/release) branch back into [master](https://github.com/ipfs/kubo/tree/master)
  - [ ] create a new branch `merge-release-vX.Y.Z` from `release`
  - [ ] create and merge a PR from `merge-release-vX.Y.Z` to `master`
    - ⚠️ **NOTE:** make sure to ignore the changes to [version.go](version.go) (keep the `-dev` in `master`) 
- [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) Prepare for the next release 
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) Create the next [changelog](https://github.com/ipfs/kubo/blob/master/docs/changelogs/vX.(Y+1).md)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) Link to the new changelog in the [CHANGELOG.md](CHANGELOG.md) file
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) Create the next release issue
- [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) Create a dependency update PR
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) check out [ipfs/kubo](https://github.com/ipfs/kubo)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) go over direct dependencies from `go.mod` in the root directory (NOTE: do not run `go get -u` as it will upgrade indirect dependencies which may cause problems)
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) run `make mod_tidy` 
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) create a PR which updates `go.mod` and `go.sum`
  - [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) ![](https://img.shields.io/badge/not-PATCH-orange?style=flat-square) add the PR to the next release milestone
- [ ] ![](https://img.shields.io/badge/only-FINAL-darkgreen?style=flat-square) Close the release issue
