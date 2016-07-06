# Notes on the Filestore Datastore

The filestore is a work-in-progress datastore that stores the unixfs
data component of blocks in files on the filesystem instead of in the
block itself.  The main use of the datastore is to add content to IPFS
without duplicating the content in the IPFS datastore.

The filestore is developed on Debian (GNU/Linux).  It has been tested on
Windows and should work on MacOS X and other Unix like systems.

## Quick start

To add a file to IPFS without copying, use `filestore add -P` or to add a
directory use `filestore add -P -r`.  (Throughout this document all
command are assumed to start with `ipfs` so `filestore add` really
mains `ipfs filestore add`).  For example to add the file `hello.txt`
use:
```
  ipfs filestore add -P hello.txt
```
The file or directory will then be added.  You can now try to retrieve
it from another node such as the ipfs.io gateway.

Paths stored in the filestore must be absolute.  You can either
provide an absolute path or use one of `-P` (`--physical`) or `-l`
(`--logical`) to create one.  The `-P` (or `--physical`) means to make
an absolute path from the physical working directory without any
symbolic links in it; the `-l` (or `--logical`) means to use the `PWD`
env. variable if possible.

If adding a file with the daemon online the same file must be
accessible via the path provided by both the client and the server.
Without extra options it is currently not possible to add directories
with the daemon online.

If the contents of an added file have changed the block will become
invalid.  By default, the filestore uses the modification-time to
determine if a file has changed.  If the mod-time of a file differs
from what is expected the contents of the block are rechecked by
recomputing the multihash and failing if the hash differs from what is
expected.

Adding files to the filestore will generally be faster than adding
blocks normally as less data is copied around.  Retrieving blocks from
the filestore takes about the same time when the hash is not
recomputed, when it is, retrieval is slower.

## Adding all files in a directory

The example script in filestore/examples/add-dir.sh can be used to add
all files in a directly to the filestore and keep the filestore in
sync with what is the directory.  Just specify the directory you want
to add or update.  The first time it is run it will add all the files
in the directory.  When run again it will re-add any modified files.  A
good use of this script is to add it to crontab to rerun the script
periodically.

The script is fairly basic but serves as an example of how to use the
filestore.  A more sophisticated application could use i-notify or a
similar interface to re-add files as they are changed.

## Server side adds

When adding a file when the daemon is online.  The client sends both
the file contents and path to the server, and the server will then
verify that the same content is available via the specified path by
reading the file again on the server side.  To avoid this extra
overhead and allow directories to be added when the daemon is
online server side paths can be used.

To use this feature you must first enable API.ServerSideAdds using:
```
  ipfs config Filestore.APIServerSidePaths --bool true
```
*This option should be used with care since it will allow anyone with
access to the API Server access to any files that the daemon has
permission to read.* For security reasons it is probably best to only
enable this on a single user system and to make sure the API server is
configured to the default value of only binding to the localhost
(`127.0.0.1`).

With the `Filestore.APIServerSidePaths` option enabled you can add
files using `filestore add -S`.  For example, to add the file
`hello.txt` in the current directory use:
```
  ipfs filestore add -S -P hello.txt
```

## Verifying blocks

To list the contents of the filestore use the command `filestore ls`.
See `--help` for additional information.

Note that due to a known bug, datastore keys are sometimes mangled
(see [go-ipfs issue #2601][1]).  Do not be alarmed if you see keys
like `6PKtDkh6GvBeJZ5Zo3v8mtXajfR4s7mgvueESBKTu5RRy`.  The block is
still valid and can be retrieved by the unreported correct hash.
(Filestore maintenance operations will still function on the mangled
hash, although operations outside the filestore might complain of an
`invalid ipfs ref path`).

[1]: https://github.com/ipfs/go-ipfs/issues/2601

To verify the contents of the filestore use `filestore verify`.
See `--help` for additional info.

## Maintenance

Invalid blocks will cause problems with various parts of ipfs and
should be cleared out on a regular basis.  For example, `pin ls` will
currently abort if it is unable to read any blocks pinned (to get
around this use `pin ls -t direct` or `pin ls -r recursive`).  Invalid
blocks may cause problems elsewhere.

Currently no regular maintenance is done and it is unclear if this is
a desirable thing as I image the filestore will primary be used in
conjunction will some higher level tools that will automatically
manage the filestore.

All maintenance commands should currently be run with the daemon
offline.  Running them with the daemon online is untested, in
particular the code has not been properly audited to make sure all the
correct locks are being held.

## Removing Invalid blocks

The `filestore clean` command will remove invalid blocks as reported
by `filstore verify`.  You must specify what type of invalid blocks to
remove.  This command should be used with some care to avoid removing
more than is intended.  For help with the command use
`filestore clean --help`

Removing `changed` and `no-file` blocks (as reported by `filestore verify`
is generally a safe thing to do.  When removing `no-file` blocks there
is a slight risk of removing blocks to files that might reappear, for
example, if a filesystem containing the file for the block is not
mounted.

Removing `error` blocks runs the risk of removing blocks to files that
are not available due to transient or easily correctable (such as
permission problems) errors.

Removing `incomplete` blocks is generally a good thing to do to avoid
problems with some of the other ipfs maintenance commands such as the
pinner.  However, note that there is nothing wrong with the block
itself, so if the missing blocks are still available elsewhere
removing `incomplete` blocks is immature and might lead to lose of
data.

Removing `orphan` blocks like `incomplete` blocks runs the risk of
data lose if the root node is found elsewhere.  Also `orphan` blocks
do not cause any problems, they just take up a small amount of space.

## Fixing Pins

When removing blocks `filestore clean` will generally remove any pins
associated with the blocks.  However, it will not handle `indirect`
pins.  For example if you add a directory using `filestore add -r` and
some of the files become invalid the recursive pin will become invalid
and needs to be fixed.

One way to fix this is to use `filestore fix-pins`.  This will
remove any pins pointing to invalid non-existent blocks and also
repair recursive pins by making the recursive pin a direct pin and
pinning any children still valid.  

Pinning the original root as a direct pin may not always be the most
desirable thing to do, in which case you can use the `--skip-root` 
to unpin the root, but still pin any children still valid.

## Pinning and removing blocks manually.

The desirable behavior of pinning and garbage collection with
filestore blocks is unclear.  For now filestore blocks are pinned as
normal when added, but unpinned blocks are not garbage collected and
need to be manually removed.

To list any unpinned objects in the filestore use `filestore
unpinned`.  This command will list unpinned blocks corresponding to
whole files.  You can either pin them by piping the output into `pin
add` or manually delete them.

To manually remove blocks use `filestore rm`.  By default only blocks
representing whole files can be removed and the removal will be
recursive.  Direct and recursive pins will be removed along with the
block but `filestore rm` will abort if any indirect pins are detected.
To allow the removal of files with indirect pins use the `--force`
option.  Individual blocks can be removed with the `--direct` option.

## Duplicate blocks.

If a block has already been added to the datastore, adding it
again with `filestore add` will add the block to the filestore
but the now duplicate block will still exists in the normal
datastore. Furthermore, since the block is likely to be pinned
it will not be removed when `repo gc` in run.  This is nonoptimal
and will eventually be fixed.  For now, you can remove duplicate
blocks by running `filestore rm-dups`.

## Upgrading the filestore

As the filestore is a work in progress changes to the format of
filestore repository will be made from time to time.  These changes
will be temporary backwards compatible but not forwards compatible.
Eventually support for the old format will be removed.  While both
versions are supported the command "filestore upgrade" can be used to
upgrade the repository to the new format.
