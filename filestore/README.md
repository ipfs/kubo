# Notes on the Filestore

The filestore is a work-in-progress datastore that stores the unixfs
data component of blocks in files on the filesystem instead of in the
block itself.  The main use of the datastore is to add content to IPFS
without duplicating the content in the IPFS datastore.

The filestore is developed on Debian (GNU/Linux).  It has has limited
testing on Windows and should work on MacOS X and other Unix like
systems.

Before the filestore can be used it must be enabled with
```
  ipfs filestore enable
```

## Adding Files

To add a file to IPFS without copying, use `filestore add -P` or to add a
directory use `filestore add -P -r`.  (Throughout this document all
command are assumed to start with `ipfs` so `filestore add` really
mains `ipfs filestore add`).  For example to add the file `hello.txt`
use:
```
  ipfs filestore add -P hello.txt
```

Paths stored in the filestore must be absolute.  You can either
provide an absolute path or use one of `-P` (`--physical`) or `-l`
(`--logical`) to create one.  The `-P` (or `--physical`) means to make
an absolute path from the physical working directory without any
symbolic links in it; the `-l` (or `--logical`) means to use the `PWD`
env. variable if possible.

When adding a file with the daemon online the same file must be
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

Adding all files in a directory using `-r` is limited.  For one thing,
it can normally only be done with the daemon offline.  In addition it is
not a resumable operation.  A better way is to use the "add-dir" script
found in the `examples/` directory.  It usage is:
```
  add-dir [--scan] DIR [CACHE]
```
In it's most basic usage it will work like `filestore add -r` but will
add files individually rather than as a directory.  If the `--scan`
option is used the script will scan the filestore for any files
already added and only add new files or those that have changed.  When
the `--scan` option is used to keep a directory in sync, duplicate
files will always be readded.  In addition, if two files have
overlapping content it is not guaranteed to find all changes.  To
avoid these problems a cache file can also be specified.

If a cache file is specified, then, information about the files will
be stored in the file `CACHE` in order to keep the directory contents
in sync with what is in the filestore.  The cache files is written out
as files are added so that if the script is aborted it will pick up
from where it left off the next time it is run.

If the cache file does not exist and `--scan` is specified than the
cache will be initialized with what is in the filestore.

A good use of the add-dir script is to add it to crontab to rerun the
script periodically.

The add-dir does not perform any maintenance to remove blocks that
have become invalid so it would be a good idea to run something like
`ipfs filestore clean full` periodically.  See the maintenance section
later in this document for more details.

The `add-dir` script if fairly simple way to keep a directly in sync.
A more sophisticated application could use i-notify or a similar
interface to re-add files as they are changed.

## Listing and verifying blocks

To list the contents of the filestore use the command `filestore ls`,
or `filestore ls-files`.  See `--help` for additional information.

To verify the contents of the filestore use `filestore verify`.
Again see `--help` for additional info.

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

Maintenance commands are safe to run with the daemon running; however,
if other filestore modification operations are running in parallel,
the maintaince command may not be complete.  Most maintenance commands
will operate on a snapshot of the database when it was last in a
consistent state.

## Removing Invalid blocks

The `filestore clean` command will remove invalid blocks as reported
by `filestore verify`.  You must specify what type of invalid blocks to
remove.  This command should be used with some care to avoid removing
more than is intended.  For help with the command use
`filestore clean --help`

Removing `changed` and `no-file` blocks (as reported by `filestore verify`
is generally a safe thing to do.  When removing `no-file` blocks there
is a slight risk of removing blocks to files that might reappear, for
example, if a filesystem containing the file for the block is not
mounted.

Removing `error` blocks runs the risk of removing blocks to files that
are not available due to transient or easily correctable errors (such as
permission problems).

Removing `incomplete` blocks is generally safe as the interior node is
basically useless without the children.  However, there is nothing
wrong with the block itself, so if the missing children are still
available elsewhere removing `incomplete` blocks is immature and might
lead to the lose of data.

Removing `orphan` blocks, like `incomplete` blocks, runs the risk of
data lose if the root node is found elsewhere.  Also, unlike
`incomplete` blocks `orphan` blocks may still be useful and only take
up a small amount of space.

## Pinning and removing blocks manually.

Filestore blocks are never garage collected and hence filestore blocks
are not pinned by default when added.  If you add a directory it will
also not be pinned (as that will indirectly pin filestore objects) and
hence the directory object might be garbage collected as it is not
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

## Controlling when blocks are verified.

The config variable `Filestore.Verify` can be used to customize when
blocks from the filestore are verified.  The default value `IfChanged`
will verify a block if the modification time of the backing file has
changed.  This default works well in most cases, but can miss some
changes, espacally if the filesystem only tracks file modification
times with a resolution of one second (HFS+, used by OS X) or less
(FAT32).  A value of `Always`, always checks blocks, and the value of
`Never`, never checks blocks.

## Upgrading the filestore

As the filestore is a work in progress changes to the format of
filestore repository will be made from time to time.  These changes
will be temporary backwards compatible but not forwards compatible.
Eventually support for the old format will be removed.  While both
versions are supported the command "filestore upgrade" can be used to
upgrade the repository to the new format.

