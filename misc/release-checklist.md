# ipfs release checklist

- [ ] version changed in repo/config/version.go
- [ ] changelog.md updated
- [ ] commit tagged
- [ ] tests
  - [ ] go-ipfs tests
  - [ ] sharness tests
  - [ ] webui works
  - [ ] js-ipfs-api tests
- [ ] builds
  - [ ] windows
  - [ ] linux
    - [ ] amd64
	- [ ] arm
  - [ ] osx

## post release
- [ ] bump repo/config/version.go to $NEXT-dev
