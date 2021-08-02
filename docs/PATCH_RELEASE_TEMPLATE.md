# Patch Release Checklist

This process handles patch releases from version `vX.Y.Z` to `vX.Y.Z+1` assuming that `vX.Y.Z` is the latest released version of go-ipfs.

- [ ] Fork a new branch (`release-vX.Y.Z`) from `release` and cherry-pick the relevant commits from master (or custom fixes) onto this branch
- [ ] Make a minimal changelog update tracking the relevant fixes to CHANGELOG.
- [ ] version string in `version.go` has been updated (in the `release-vX.Y.Z+1` branch).
- [ ] Make a PR merging `release-vX.Y.Z+1` into the release branch
- [ ] tag the merge commit in the `release` branch with `vX.Y.Z+1`
- [ ] upload to dist.ipfs.io
  1. Build: https://github.com/ipfs/distributions#usage.
  2. Pin the resulting release.
  3. Make a PR against ipfs/distributions with the updated versions, including the new hash in the PR comment.
  4. Ask the infra team to update the DNSLink record for dist.ipfs.io to point to the new distribution.
- [ ] cut a release on [github](https://github.com/ipfs/go-ipfs/releases) and upload the result of the ipfs/distributions build in the previous step.
- Announce the Release:
  - [ ] On IRC/Matrix (both #ipfs and #ipfs-dev)
  - [ ] On discuss.ipfs.io
- [ ] Release published
  - [ ] to [dist.ipfs.io](https://dist.ipfs.io)
  - [ ] to [npm-go-ipfs](https://github.com/ipfs/npm-go-ipfs)
  - [ ] to [chocolatey](https://chocolatey.org/packages/ipfs)
  - [ ] to [snap](https://snapcraft.io/ipfs)
  - [ ] to [github](https://github.com/ipfs/go-ipfs/releases)
  - [ ] to [arch](https://www.archlinux.org/packages/community/x86_64/go-ipfs/) (flag it out of date)
- [ ] Cut a new ipfs-desktop release
- [ ] Merge the `release` branch back into `master`, ignoring the changes to `version.go` (keep the `-dev` version from master).
