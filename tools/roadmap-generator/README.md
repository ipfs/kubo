# go-ipfs Roadmap Generator

This directory contains tools for generating this project's roadmap.

Uses [roadmap-generator](https://github.com/haadcode/roadmap-generator).

## Requirements
- Node.js v6.x
- Npm v3.x
- GITHUB_TOKEN environment variable set
  - See [Creating an access token](https://help.github.com/articles/creating-an-access-token-for-command-line-use/) for help.

## Install
```
npm install
```
## Configure

Put configuration in `go-ipfs-roadmap.conf.js`

## Generate Roadmap
```
npm run roadmap
```

This will generate ROADMAP.md file in this directory. You can copy the ROADMAP.md file to the project's root directory and commit it to Git.
