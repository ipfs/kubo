# ipfs {{version}} checklist

## pre-release

- [ ] CHANGELOG.md updated - PR: 
- [ ] tests are green
  - [ ] go-ipfs tests
  - [ ] sharness tests
  - [ ] webui works
  - [ ] js-ipfs-api tests
  - [ ] deploy on one of our hosts

## RC cycle
- [ ] versions changed to {{version}}-rcX:
  - [ ] in repo/config/version.go
  - [ ] in package.json
- [ ] release {{version}}-rcX on dist

## release
- [ ] versions changed to {{version}}:
  - [ ] in repo/config/version.go
  - [ ] in package.json
- [ ] gx publish done and commited
- [ ] signed version tag pushed
- [ ] fast forward merge of **master** to **release**
- [ ] push release to dist

## post-release
- [ ] bump version to {{version+1}}-dev:
  - [ ] in repo/config/version.go 
  - [ ] in package.json

