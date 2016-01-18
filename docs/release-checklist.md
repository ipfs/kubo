# go-ipfs release checklist

- [ ] before release, tag 'release canidate' for users to test against
  - if bugs are found/fixed, do another release canidate
- [ ] all tests pass (no exceptions)
- [ ] webui works (for most definitions of 'works')
- [ ] CHANGELOG.md has been updated
  - use `git log --pretty=short vLAST..master`
- [ ] version string in `repo/config/version.go` has been updated
- [ ] tag commit with vX.Y.Z
- [ ] update release branch to point to release commit
- [ ] ensure version is built in gobuilder
- [ ] publish next version to https://github.com/ipfs/npm-go-ipfs
