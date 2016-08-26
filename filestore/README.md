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

To verify the contents of the filestore use `filestore verify`.
See `--help` for additional info.

## Maintenance

Invalid blocks should be cleared out from time to time.  An invalid
block is a block where the data from the backing file is no longer
available.  Operations that only depend on the DAG object metadata
will continue to function (such as `refs` and `repo gc`) but any
attempt to retrieve the block will fail.

Currently no regular maintenance is done and it is unclear if this is
a desirable thing as I image the filestore will primary be used in
conjunction will some higher level tools that will automatically
manage the filestore.

Before performing maintenance any invalid pinned blocks need to be
manually unpinned.  The maintenance commands will skip pinned blocks.

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

Removing `incomplete` blocks is generally safe as the interior node
is basically useless without the children.  However, there is nothing
wrong with the block itself, so if the missing children are still
available elsewhere removing `incomplete` blocks is immature and might
lead to the lose of data.

Removing `orphan` blocks, like `incomplete` blocks, runs the risk of data
lose if the root node is found elsewhere.  Also `orphan` blocks may still be
useful and they only take up a small amount of space.

## Pinning and removing blocks manually.

Filestore blocks are never garage collected and hence filestore blocks
are not pinned by default when added.  If you add a directory it will
also not be pinned (as that will indirectly pin filestore objects) and
hense the directory object might be gargage collected as it is not
stored in the filestore.

To manually remove blocks use `filestore rm`.  The syntax for the
command is the same as for `block rm` except that filestore blocks
will be removed rather than blocks in cache.  The best way to remove
all blocks associated with a file is to remove the root node and then
do a `filestore clean orphan` to remove the children.  An alternative
way is to parse `ipfs filestore ls` for all blocks associated with a
file.  Note through, that by doing this you might remove blocks that
are shared with another file.

## Duplicate blocks.

If a block has already been added to the datastore, adding it again
with `filestore add` will add the block to the filestore but the now
duplicate block will still exists in the normal datastore.  If the
block is not pinned it will be removed from the normal datastore when
garbage collected.  If the block is pinned it will exist in both
locations.  Removing the duplicate may not always be the most
desirable thing to do as filestore blocks are less stable.

The command "filestore dups" will list duplicate blocks.  "block rm"
can then be used to remove the blocks.  It is okay to remove a
duplicate pinned block as long as at least one copy is still around.

Once a file is in the filestore it will not be added to the normal
datastore, the option "--allow-dup" will override this behavior and
add the file anyway.  This is useful for testing and to make a more
stable copy of an important peace of data.

To determine the location of a block use "block locate".

## Upgrading the filestore

As the filestore is a work in progress changes to the format of
filestore repository will be made from time to time.  These changes
will be temporary backwards compatible but not forwards compatible.
Eventually support for the old format will be removed.  While both
versions are supported the command "filestore upgrade" can be used to
upgrade the repository to the new format.
