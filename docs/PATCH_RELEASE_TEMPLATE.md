# Patch Release Checklist

This process handles patch releases from version `vX.Y.Z` to `vX.Y.Z+1` assuming that `vX.Y.Z` is the latest released version of Kubo.

- [ ] Get temporary permissions to force-push to `release-*` branches
- [ ] Fork a new branch (`release-vX.Y.Z`) from `release` and cherry-pick the relevant commits from master (or custom fixes) onto this branch
  - [ ] Use `git cherry-pick -x` so that the commit message says `(cherry picked from commit ...)`
- [ ] Make a minimal changelog update tracking the relevant fixes to CHANGELOG, as its own commit e.g. `docs: update changelog vX.Y.Z+1`
- [ ] version string in `version.go` has been updated (in the `release-vX.Y.Z+1` branch), as its own commit. 
- [ ] Make a PR merging `release-vX.Y.Z+1` into the release branch
  - This may be unnecessary, e.g. for backports
- [ ] Tag the merge commit in the `release` branch with `vX.Y.Z+1` (ensure the tag is signed)
- [ ] Add artifacts to https://dist.ipfs.tech/kubo
  1. Make a PR against [ipfs/distributions](https://github.com/ipfs/distributions) with local changes produced by `add-version` (see [usage](https://github.com/ipfs/distributions#usage))
  2. Wait for PR to build artifacts and generate diff
  3. Inspect results, merge if CI is green and the diff looks ok
  4. Wait for `master` branch to build and update DNSLink at https://dist.ipfs.tech
- [ ] Cut a release on [github](https://github.com/ipfs/kubo/releases) and reuse signed artifacts from https://dist.ipfs.tech/kubo (run [sync-release-assets.yml workflow](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml)).
- [ ] Announce the Release:
  - [ ] On [discuss.ipfs.tech](https://discuss.ipfs.tech)
    - This will automatically post to Matrix (`#lobby:ipfs.io`) and IPFS Discord (`#ipfs-chatter`)
    - Examples from the past: [0.13.1](https://discuss.ipfs.tech/t/go-ipfs-v0-13-1-has-been-released/14599)
    - [ ] Pin the discuss topic
- [ ] Release published
  - [ ] to [dist.ipfs.tech](https://dist.ipfs.tech)
  - [ ] to [npm-go-ipfs](https://www.npmjs.com/package/go-ipfs) (should be done by [ipfs/npm-go-ipfs](https://github.com/ipfs/npm-go-ipfs), but ok to dispatch [this job](https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml) manually)
  - [ ] to [chocolatey](https://chocolatey.org/packages/go-ipfs) (should be done by [ipfs/choco-go-ipfs](https://github.com/ipfs/choco-go-ipfs/), but ok to dispatch [this job](https://github.com/ipfs/choco-go-ipfs/actions/workflows/main.yml) manually)
  - [ ] to [snap](https://snapcraft.io/ipfs) (should happen automatically, see [snap/snapcraft.yaml](https://github.com/ipfs/kubo/blob/master/snap/snapcraft.yaml))
  - [ ] to [github](https://github.com/ipfs/kubo/releases)
  - [ ] to [arch](https://www.archlinux.org/packages/community/x86_64/go-ipfs/) (flag it out of date)
- [ ] Cut a new ipfs-desktop release
- [ ] Merge the `release` branch back into `master`, ignoring the changes to `version.go` (keep the `-dev` version from master).
