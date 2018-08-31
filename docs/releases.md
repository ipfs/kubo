# go-ipfs releases

## Release Schedule
go-ipfs is on a six week release schedule. Following a release, there will be
five weeks for code of any type (features, bugfixes, etc) to be added. After
the five weeks is up, a release candidate is tagged and only important bugfixes
will be allowed up to release day.

## Release Candidate Checklist
- [ ] CHANGELOG.md has been updated
  - use `./bin/mkreleaselog` to generate a nice starter list
- [ ] version string in `repo/version.go` has been updated
- [ ] tag commit with vX.Y.Z-rcN
- [ ] publish gx version with `gx publish`, as per [gx release guidelines](https://github.com/whyrusleeping/gx#publishing-and-releasing)
  - you will have to manually adjust the gx version to 'rc'

## Pre-Release Checklist
- [ ] before release, tag 'release candidate' for users to test against
  - if bugs are found/fixed, do another release candidate
- [ ] all tests pass (no exceptions)
- [ ] run interop tests https://github.com/ipfs/interop#test-with-a-non-yet-released-version-of-go-ipfs
- [ ] webui works (for most definitions of 'works') - Test the multiple pages and verify that no visible errors are shown.
- [ ] CHANGELOG.md has been updated
  - use `./bin/mkreleaselog` to generate a nice starter list
- [ ] version string in `repo/version.go` has been updated
- [ ] tag commit with vX.Y.Z
- [ ] update release branch to point to release commit
- [ ] publish dist.ipfs.io
- [ ] publish next version to https://github.com/ipfs/npm-go-ipfs
- [ ] publish gx version with `gx release`, as per [gx release guidelines](https://github.com/whyrusleeping/gx#publishing-and-releasing)

## Post-Release
- [ ] Bump version string in `repo/version.go` to `vX.Y.Z-dev`
- [ ] Upload the final release to the github releases page: https://github.com/ipfs/go-ipfs/releases
- Communication
  - [ ] Create the release issue
  - [ ] Announcements (both pre-release and post-release)
    - [ ] Twitter
    - [ ] IRC
    - [ ] Reddit
  - [ ] Blog post (at minimum, paste the changelog. optionally add context and thank contributors.)
- [ ] Update HTTP-API Documentation on the Website using https://github.com/ipfs/http-api-docs
