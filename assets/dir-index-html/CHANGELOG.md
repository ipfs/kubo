# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.2.0] - 2020-08-03
This release streamlines the process for making future updates to this repo:

- Moves source-of-truth HTML and CSS files into new `src` directory ([details](https://github.com/ipfs/dir-index-html/pull/40#issue-456530181))
- Adds build script in `package.json` to generate minified/inlined `dir-index.html` at top level from individual files in `src` directory ([details](https://github.com/ipfs/dir-index-html/pull/40#issue-456530181))
- Adds GitHub Action to guard against committing state where `dir-index.html` does not match source materials in `src` ([details](https://github.com/ipfs/dir-index-html/pull/40#pullrequestreview-456126397))

## [v1.1.0] - 2020-07-24
This release brings general tidying, plus some substantial UI enhancements! Big thanks to @neatonk for all the work.

- Adds a column for CIDs between the name and size columns; CIDs are clickable links that open the item as a new "root path", enabling users to copy direct links to images or subdirectories (see https://github.com/ipfs/dir-index-html/issues/37 and https://github.com/ipfs/dir-index-html/issues/15)
- Adds the size of the current directory to the header of the table (see https://github.com/ipfs/dir-index-html/issues/37 and https://github.com/ipfs/dir-index-html/issues/25)
- Makes path components in table headers into links, so clicking on segments between directory slashes will go to that level of the directory tree (see https://github.com/ipfs/dir-index-html/issues/37 and https://github.com/ipfs/dir-index-html/issues/2)
- Updates tests to include testing the above (see https://github.com/ipfs/dir-index-html/pull/38)
- Reconciles legacy discrepancies between `dir-index.html` and `dir-index-uncat.html` (see https://github.com/ipfs/dir-index-html/pull/39)


## [v1.0.6] - 2020-06-25
- Adds favicon: visual consistency/prettiness, but more importantly prevents 404 error on an implicit /favicon.ico (see [#35](https://github.com/ipfs/dir-index-html/issues/35))
- Adds social sharing metadata (see [#34](https://github.com/ipfs/dir-index-html/issues/34))
- Updates contributing link in readme (thanks @stensonb!)


## [v1.0.5] - 2020-05-05

- Removes extraneous references to Glyphicons (closes [#23](https://github.com/ipfs/dir-index-html/issues/23))
- Makes page responsive overall (closes [#24](https://github.com/ipfs/dir-index-html/issues/24))
- Adds file icons for .wmv, .mov, .mkv (closes [#19](https://github.com/ipfs/dir-index-html/issues/19))
- Strips out unneeded CSS
- Makes colors more accessible consistent with those in ipfs-css
- Tidies up in general


## [v1.0.4] - 2020-04-21
- Update style to match IPFS branding
- Add links to ipfs.io, install help, and the bug tracker


## [v1.0.3] - 2016-08-31
- No release notes added


## [v1.0.2] - 2016-08-31
- No release notes added


## [v1.0.1] - 2016-08-31
- No release notes added
