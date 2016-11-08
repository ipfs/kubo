# Notes on the Filestore

The filestore is a work-in-progress datastore that stores the unixfs
data component of blocks in files on the filesystem instead of in the
block itself.  The main use of the datastore is to add content to IPFS
without duplicating the content in the IPFS datastore.

The filestore is developed on Debian (GNU/Linux).  It has has limited
testing on Windows and should work on MacOS X and other Unix like
systems.

## Adding Files

To add a file to IPFS without copying, use `add --no-copy` or to add a
directory use `add --no-copy`.  (Throughout this document all
command are assumed to start with `ipfs` so `filestore add` really
mains `ipfs filestore add`).  For example to add the file `hello.txt`
use:
```
  ipfs filestore add "`pwd`"/hello.txt
```

Paths stored in the filestore must be absolute.

By default, the contents of the file are always verified by
recomputing the hash.  The setting `Filestore.Verify` can be used to
change this to never recompute the hash (not recommended) or to only
recompute the hash when the modification-time has changed.

Adding files to the filestore will generally be faster than adding
blocks normally as less data is copied around.  Retrieving blocks from
the filestore takes about the same time when the hash is not
recomputed, when it is, retrieval is slower.

## About filestore entries

Each entry in the filestore is uniquely refereed to by combining the
(1) the hash of the block, (2) the path to the file, and (3) the
offset within the file, using the following syntax:
```
  <HASH>/<FILEPATH>//<OFFSET>
```
for example:
```
  QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH//somedir/hello.txt//0
```

In the case that there is only one entry for a hash the entry is
stored using just the hash.  If there is more than one entry for a
hash (for example if adding two files with identical content) than one
entry will be stored using just the hash and the others will be stored
using the full key.  If the backing file changes or becomes
inaccessible for the default entry (the one with just the hash) the
other entries are tried until a valid entry is found.  Once a valid
entry is found that entry will become the default.

When listing the contents of the filestore entries that are stored
using just the hash are displayed as
```
  <HASH> /<FILEPATH>//<OFFSET>
```
with a space between the <HASH> amd <FILEPATH>.

It is always possible to refer to a specific entry in the filestore
using the full key regardless to how it is stored.

## Controlling when blocks are verified.

The config variable `Filestore.Verify` can be used to customize when
blocks from the filestore are verified.  The default value `Always`
will always verify blocks.  A value of `IfChanged.  will verify a
block if the modification time of the backing file has changed.  This
value works well in most cases, but can miss some changes, espacally
if the filesystem only tracks file modification times with a
resolution of one second (HFS+, used by OS X) or less (FAT32).  A
value of `Never`, never checks blocks.
