<!-- Last updated during [v0.38.0 release](https://github.com/ipfs/kubo/issues/10884) -->

# ✅ Release Checklist (vX.Y.Z[-rcN])

**Release types:** RC (Release Candidate) | FINAL | PATCH

## Prerequisites

- [ ] [GPG signature](https://docs.github.com/en/authentication/managing-commit-signature-verification) configured in local git and GitHub
- [ ] [Docker](https://docs.docker.com/get-docker/) installed on your system
- [ ] [npm](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm) installed on your system
- [ ] kubo repository cloned locally
- [ ] **non-PATCH:** Upgrade Go in CI to latest patch from <https://go.dev/dl/>

## 1. Prepare Release Branch

- [ ] Fetch latest changes: `git fetch origin master release`
- [ ] Create branch `release-vX.Y.Z` (base from: `master` if Z=0 for new minor/major, `release` if Z>0 for patch)
- [ ] **RC1 only:** Switch to `master` branch and prepare for next release cycle:
  - [ ] Update [version.go](https://github.com/ipfs/kubo/blob/master/version.go) to `vX.Y+1.0-dev` (⚠️ double-check Y+1 is correct) ([example PR](https://github.com/ipfs/kubo/pull/9305))
  - [ ] Create `./docs/changelogs/vX.Y+1.md` and add link in [CHANGELOG.md](https://github.com/ipfs/kubo/blob/master/CHANGELOG.md)
- [ ] Switch to `release-vX.Y.Z` branch and update [version.go](https://github.com/ipfs/kubo/blob/master/version.go) to `vX.Y.Z(-rcN)` (⚠️ double-check Y matches release) ([example](https://github.com/ipfs/kubo/pull/9394))
- [ ] Create draft PR: `release-vX.Y.Z` → `release` ([example](https://github.com/ipfs/kubo/pull/9306))
- [ ] In `release-vX.Y.Z` branch, cherry-pick commits from `master`: `git cherry-pick -x <commit>` ([example](https://github.com/ipfs/kubo/pull/10636/commits/033de22e3bc6191dbb024ad6472f5b96b34e3ccf))
  - ⚠️ **NOTE:** `-x` flag records original commit SHA for traceability and ensures cleaner merges with deduplicated commits in history
- [ ] Verify all CI checks on the PR are passing
- [ ] **FINAL only:** In `release-vX.Y.Z` branch, replace `Changelog` and `Contributors` sections with `./bin/mkreleaselog` stdout (do **NOT** copy stderr)
- [ ] **FINAL only:** Merge PR (`release-vX.Y.Z` → `release`) using `Create a merge commit`
  - ⚠️ do **NOT** use `Squash and merge` nor `Rebase and merge` because we need to be able to sign the merge commit
  - ⚠️ do **NOT** delete the `release-vX.Y.Z` branch (needed for future patch releases and git history)

## 2. Tag & Publish

### Create Tag
⚠️ **POINT OF NO RETURN:** Once pushed, tags trigger automatic Docker/NPM publishing that cannot be reversed!
If you're making a release for the first time, do pair programming and have the release reviewer verify all commands.

- [ ] **RC:** From `release-vX.Y.Z` branch: `git tag -s vX.Y.Z-rcN -m 'Prerelease X.Y.Z-rcN'`
- [ ] **FINAL:** After PR merge, from `release` branch: `git tag -s vX.Y.Z -m 'Release X.Y.Z'`
- [ ] ⚠️ Verify tag is signed and correct: `git show vX.Y.Z(-rcN)`
- [ ] Push tag: `git push origin vX.Y.Z(-rcN)`
  - ⚠️ do **NOT** use `git push --tags` because it pushes all your local tags
- [ ] **STOP:** Wait for [Docker build](https://github.com/ipfs/kubo/actions/workflows/docker-image.yml) to complete before proceeding

### Publish Artifacts

- [ ] **Docker:** Publish to [DockerHub](https://hub.docker.com/r/ipfs/kubo/tags)
  - [ ] Wait for [Publish docker image](https://github.com/ipfs/kubo/actions/workflows/docker-image.yml) workflow triggered by tag push
  - [ ] Verify image is available on [Docker Hub → tags](https://hub.docker.com/r/ipfs/kubo/tags)
- [ ] **dist.ipfs.tech:** Publish to [dist.ipfs.tech](https://dist.ipfs.tech)
  - [ ] Check out [ipfs/distributions](https://github.com/ipfs/distributions)
  - [ ] Create branch: `git checkout -b release-kubo-X.Y.Z(-rcN)`
  - [ ] Verify `.tool-versions` golang matches [Kubo's CI](https://github.com/ipfs/kubo/blob/master/.github/workflows/gotest.yml) `go-version:` (update if needed)
  - [ ] Run: `./dist.sh add-version kubo vX.Y.Z(-rcN)` ([usage](https://github.com/ipfs/distributions#usage))
  - [ ] Create and merge PR (updates `dists/kubo/versions`, **FINAL** also updates `dists/kubo/current` - [example](https://github.com/ipfs/distributions/pull/1125))
  - [ ] Wait for [CI workflow](https://github.com/ipfs/distributions/actions/workflows/main.yml) triggered by merge
  - [ ] Verify release on [dist.ipfs.tech](https://dist.ipfs.tech/#kubo)
- [ ] **NPM:** Publish to [NPM](https://www.npmjs.com/package/kubo?activeTab=versions)
  - [ ] Manually dispatch [Release to npm](https://github.com/ipfs/npm-kubo/actions/workflows/main.yml) workflow if not auto-triggered
  - [ ] Verify release on [NPM](https://www.npmjs.com/package/kubo?activeTab=versions)
- [ ] **GitHub Release:** Publish to [GitHub](https://github.com/ipfs/kubo/releases)
  - [ ] [Create release](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository#creating-a-release) ([RC example](https://github.com/ipfs/kubo/releases/tag/v0.36.0-rc1), [FINAL example](https://github.com/ipfs/kubo/releases/tag/v0.35.0))
  - [ ] Use tag `vX.Y.Z(-rcN)`
  - [ ] Link to release issue
  - [ ] **RC:** Link to changelog, check `This is a pre-release`
  - [ ] **FINAL:** Copy changelog content (without header), do **NOT** check pre-release
  - [ ] Run [sync-release-assets](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml) workflow
  - [ ] Verify assets are attached to the GitHub release

## 3. Post-Release

### Technical Tasks

- [ ] **FINAL only:** Merge `release` → `master`
  - [ ] Create branch `merge-release-vX.Y.Z` from `release`
  - [ ] Merge `master` to `merge-release-vX.Y.Z` first, and resolve conflict in `version.go`
    - ⚠️ **NOTE:** make sure to ignore the changes to [version.go](https://github.com/ipfs/kubo/blob/master/version.go) (keep the `-dev` in `master`)
  - [ ] Create and merge PR from `merge-release-vX.Y.Z` to `master` using `Create a merge commit`
    - ⚠️ do **NOT** use `Squash and merge` nor `Rebase and merge` because we want to preserve original commit history
- [ ] Update [ipshipyard/waterworks-infra](https://github.com/ipshipyard/waterworks-infra)
  - [ ] Update Kubo staging environment ([Running Kubo tests on staging](https://www.notion.so/Running-Kubo-tests-on-staging-488578bb46154f9bad982e4205621af8))
    - [ ] **RC:** Test last release against current RC
    - [ ] **FINAL:** Latest release on both boxes
  - [ ] **FINAL:** Update collab cluster boxes to the tagged release
  - [ ] **FINAL:** Update libp2p bootstrappers to the tagged release
- [ ] Smoke test with [IPFS Companion Browser Extension](https://docs.ipfs.tech/install/ipfs-companion/)
- [ ] Update [ipfs-desktop](https://github.com/ipfs/ipfs-desktop)
  - [ ] Create PR updating kubo version in `package.json` and `package-lock.json`
  - [ ] **FINAL:** Merge PR and ship new ipfs-desktop release
- [ ] **FINAL only:** Update [docs.ipfs.tech](https://docs.ipfs.tech/): run [update-on-new-ipfs-tag.yml](https://github.com/ipfs/ipfs-docs/actions/workflows/update-on-new-ipfs-tag.yml) workflow and merge the PR

### Promotion

- [ ] Create [IPFS Discourse](https://discuss.ipfs.tech) topic ([RC example](https://discuss.ipfs.tech/t/kubo-v0-38-0-rc2-is-out/19772), [FINAL example](https://discuss.ipfs.tech/t/kubo-v0-38-0-is-out/19795))
  - [ ] Title: `Kubo vX.Y.Z(-rcN) is out!`, tag: `kubo`
  - [ ] Use title as heading (`##`) in description
  - [ ] Include: GitHub release link, IPNS binaries, docker pull command, release notes
  - [ ] Pin topic globally (make banner if no existing banner)
- [ ] Verify bot posted to [#ipfs-chatter](https://discord.com/channels/669268347736686612/669268347736686615) (Discord) or [#ipfs-chatter:ipfs.io](https://matrix.to/#/#ipfs-chatter:ipfs.io) (Matrix)
- [ ] **RC only:** Comment on release issue mentioning early testers ([example](https://github.com/ipfs/kubo/issues/9319#issuecomment-1311002478))
- [ ] **FINAL only:** Comment on release issue with link ([example](https://github.com/ipfs/kubo/issues/9417#issuecomment-1400740975))
- [ ] **FINAL only:** Create [blog.ipfs.tech](https://blog.ipfs.tech) entry ([example](https://github.com/ipfs/ipfs-blog/commit/32040d1e90279f21bad56b924fe4710bba5ba043))
- [ ] **FINAL non-PATCH:** (optional) Post on social media ([bsky](https://bsky.app/profile/ipshipyard.com/post/3ltxcsrbn5s2k), [x.com](https://x.com/ipshipyard/status/1944867893226635603), [Reddit](https://www.reddit.com/r/ipfs/comments/1lzy6ze/release_v0360_ipfskubo/))

### Final Steps

- [ ] **FINAL non-PATCH:** Create dependency update PR
  - [ ] Review direct dependencies from root `go.mod` (⚠️ do **NOT** run `go get -u` as it will upgrade indirect dependencies which may cause problems)
  - [ ] Run `make mod_tidy`
  - [ ] Create PR with `go.mod` and `go.sum` updates
  - [ ] Add PR to next release milestone
- [ ] **FINAL non-PATCH:** Create next release issue ([example](https://github.com/ipfs/kubo/issues/10816))
- [ ] **FINAL only:** Close release issue