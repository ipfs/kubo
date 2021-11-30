# dir-index-html

[![Made by Protocol Labs](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](https://protocol.ai)
[![Project: IPFS](https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square)](https://ipfs.io/)
[![Matrix](https://img.shields.io/badge/matrix-%23ipfs%3Amatrix.org-blue.svg?style=flat-square)](https://matrix.to/#/room/#ipfs:matrix.org)
[![IRC](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

> Directory listing HTML for `go-ipfs` gateways

**NOTE:** This repo is not intended to be used as a standalone project! This code is used by the gateway code within [`go-ipfs`](https://github.com/ipfs/go-ipfs). In the long term, once the the gateway is extracted from `go-ipfs`, the code in this repo will be merged into that gateway package.

![](https://user-images.githubusercontent.com/157609/88379209-ce6f0600-cda2-11ea-9620-20b9237bb441.png)

## Updating

When making updates to the directory listing page template, please note the following:

1. Make your changes to the (human-friendly) source documents in the `src` directory
2. Before testing or releasing, make sure to run the build script to update the minified version in the top-level directory:

```bash
> npm run build
```
3. To get your updates into `go-ipfs`, you'll need to do the following:
     - Cut a new, appropriately versioned release of `dir-index-html` (don't forget to bump the version number in `package.json`)
     - Make a PR against `go-ipfs` following [these instructions](https://github.com/ipfs/go-ipfs/tree/master/assets#updating-dir-index-html) for updating the directory index

## Testing

1. Make sure you have [Go](https://golang.org/dl/) installed
2. Start the test server, which lives in its own directory:

```bash
> cd test
> go run .
```
This will listen on [`localhost:3000`](http://localhost:3000/) and reload the template every time you refresh the page.

If you get a "no such file or directory" error upon trying `go run .`, make sure you ran `npm run build` to generate the minified artifact that the test is looking for.

## Contribute

Feel free to join in. All are welcome! A good place to start is to check the [issues](https://github.com/ipfs/dir-index-html/issues) for anything you find interesting.

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

### Want to hack on IPFS?

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

## License

MIT
