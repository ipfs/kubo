# Patch Release Checklist

This process handles patch releases from version `vX.Y.Z` to `vX.Y.Z+1` assuming that `vX.Y.Z` is the latest released version of go-ipfs.
- [ ] Fork a new branch (`release-vX.Y.Z`) from `release` and cherry-pick the relevant commits from master (or custom fixes) onto this branch
  - [ ] Use `git cherry-pick -x` so that the commit message says `(cherry picked from commit ...)`
- [ ] Make a minimal changelog update tracking the relevant fixes to CHANGELOG, as its own commit e.g. `docs: update changelog vX.Y.Z+1`
- [ ] version string in `version.go` has been updated (in the `release-vX.Y.Z+1` branch), as its own commit. 
- [ ] Make a PR merging `release-vX.Y.Z+1` into the release branch
  - This may be unnecessary, e.g. for backports
- [ ] Tag the merge commit in the `release` branch with `vX.Y.Z+1` (ensure the tag is signed)
- [ ] Upload to dist.ipfs.io
  1. Build: https://github.com/ipfs/distributions#usage.
  2. Pin the resulting release.
  3. Make a PR against ipfs/distributions with the updated versions, including the new hash in the PR comment.
  - Note the DNSLink record for dist.ipfs.io points to the new distribution as part of [CI after merging into master](https://github.com/ipfs/distributions/blob/master/.github/workflows/main.yml#L154).
- [ ] cut a release on [github](https://github.com/ipfs/go-ipfs/releases) and upload the result of the ipfs/distributions build in the previous step.
- [ ] Announce the Release:
  - [ ] On discuss.ipfs.io
    - This will automatically post to IPFS Discord #ipfs-chatter
    - Examples from the past: [0.13.1](https://discuss.ipfs.io/t/go-ipfs-v0-13-1-has-been-released/14599)
    - [ ] Pin the topic
- [ ] Release published
  - [ ] to [dist.ipfs.io](https://dist.ipfs.io)
  - [ ] to [npm-go-ipfs](https://github.com/ipfs/npm-go-ipfs) (should be done by [ipfs/npm-go-ipfs](https://github.com/ipfs/npm-go-ipfs), but ok to dispatch job manually)
  - [ ] to [chocolatey](https://chocolatey.org/packages/go-ipfs) (should be done by [ipfs/choco-go-ipfs](https://github.com/ipfs/choco-go-ipfs/), but ok to dispatch job manually)
  - [ ] to [snap](https://snapcraft.io/ipfs) (should happen automatically, see [snap/snapcraft.yaml](https://github.com/ipfs/go-ipfs/blob/master/snap/snapcraft.yaml))
  - [ ] to [github](https://github.com/ipfs/go-ipfs/releases)
  - [ ] to [arch](https://www.archlinux.org/packages/community/x86_64/go-ipfs/) (flag it out of date)
- [ ] Cut a new ipfs-desktop release
- [ ] Merge the `release` branch back into `master`, ignoring the changes to `version.go` (keep the `-dev` version from master).
