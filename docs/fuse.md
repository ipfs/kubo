# FUSE

**EXPERIMENTAL:** FUSE support is functional but still evolving. Please report issues at [kubo/issues](https://github.com/ipfs/kubo/issues).

Kubo makes it possible to mount `/ipfs`, `/ipns` and `/mfs` namespaces in your OS,
allowing arbitrary apps access to IPFS using standard filesystem operations.

The underlying FUSE implementation uses [`hanwen/go-fuse`](https://github.com/hanwen/go-fuse).

- [Install FUSE](#install-fuse)
  - [Linux](#linux)
  - [macOS](#macos)
  - [FreeBSD](#freebsd)
- [Prepare mountpoints](#prepare-mountpoints)
- [Mounting IPFS](#mounting-ipfs)
- [MFS mountpoint](#mfs-mountpoint)
- [Mode and mtime](#mode-and-mtime)
- [Troubleshooting](#troubleshooting)

## Install FUSE

You will need to install and configure FUSE before you can mount IPFS.

#### Linux

Install `fuse3` with your package manager:

```sh
# Debian / Ubuntu
sudo apt-get install fuse3

# Fedora
sudo dnf install fuse3

# Arch
sudo pacman -S fuse3
```

On some older Linux distributions, you may need to add yourself to the `fuse` group
for `allow_other` support (if no `fuse` group exists, you can skip this step):

```sh
sudo usermod -a -G fuse <username>
```

Restart your session for the change to apply.

#### macOS

Install [macFUSE](https://macfuse.github.io/):

```sh
brew install --cask macfuse
```

After installation, open **System Settings > Privacy & Security** and allow the macFUSE kernel extension to load. A reboot may be required.

Kubo automatically sets `volname`, `noapplexattr`, and `noappledouble` mount options on macOS:

- `volname` shows the filesystem name (ipfs, ipns, mfs) in Finder instead of the generic "macfuse Volume 0"
- `noapplexattr` prevents Finder from probing Apple-private extended attributes on every file access, reducing unnecessary FUSE traffic on network-backed mounts
- `noappledouble` prevents macOS from creating `._` resource fork sidecar files, which would pollute the DAG with macOS-only metadata

> [!NOTE]
> macOS has known FUSE limitations (frequent STATFS calls, limited notification support) that may affect performance. See the [`hanwen/go-fuse` macOS notes](https://github.com/hanwen/go-fuse#macos-support) for details.

#### FreeBSD

Load the FUSE kernel module:

```sh
sudo kldload fusefs
```

To load automatically on boot:

```sh
echo 'fusefs_load="YES"' | sudo tee -a /boot/loader.conf
```

## Prepare mountpoints

By default ipfs uses `/ipfs`, `/ipns` and `/mfs` directories for mounting. These can be
changed in config (see [`Mounts`](https://github.com/ipfs/kubo/blob/master/docs/config.md#mounts)). You will have to create the directories
explicitly. Note that modifying root requires sudo permissions.

```sh
# make the directories
sudo mkdir /ipfs /ipns /mfs

# chown them so ipfs can use them without root permissions
sudo chown <username> /ipfs /ipns /mfs
```

## Mounting IPFS

Make sure no other IPFS daemon is already running, then start the daemon with FUSE mounts enabled:

```sh
ipfs daemon --mount
```

Or, if the daemon is already running:

```sh
ipfs mount
```

If you wish to allow other users to use the mount points, edit `/etc/fuse.conf`
to enable non-root users:

```sh
# /etc/fuse.conf - Configuration file for Filesystem in Userspace (FUSE)

# Set the maximum number of FUSE mounts allowed to non-root users.
# The default is 1000.
#mount_max = 1000

# Allow non-root users to specify the allow_other or allow_root mount options.
user_allow_other
```

Next set `Mounts.FuseAllowOther` config option to `true`:

```sh
ipfs config --json Mounts.FuseAllowOther true
ipfs daemon --mount
```

## MFS mountpoint

The `/mfs` mount exposes the MFS (Mutable File System) root as a FUSE filesystem.
This is the same virtual mutable filesystem as the one behind `ipfs files` commands
(see `ipfs files --help`), enabling manipulation of content-addressed data like regular files.

Standard tools like `vim`, `rsync`, and `tar` work on writable mounts (`/mfs` and `/ipns`).
Operations like `fsync`, `ftruncate`, `chmod`, `touch`, and rename-over-existing are all supported.

The CID for any file or directory is retrievable via the `ipfs.cid`
extended attribute:

```sh
$ getfattr -n ipfs.cid /mfs/hello.txt
# file: mfs/hello.txt
ipfs.cid="bafkreifjjcie6lypi6ny7amxnfftagclbuxndqonfipmb64f2km2devei4"
```

> [!TIP]
> New IPFS nodes should run `ipfs config profile apply unixfs-v1-2025` to use CIDv1 with modern defaults. Without this, files default to CIDv0 (base58 `Qm...` hashes).

## Mode and mtime

By default, IPFS does not persist POSIX file mode or modification time. Most content on IPFS
does not include this metadata.

When mode or mtime is absent, FUSE mounts use sensible defaults:

- Read-only mounts (`/ipfs`): files `0444`, directories `0555`
- Writable mounts (`/ipns`, `/mfs`): files `0644`, directories `0755`

When UnixFS metadata is present in the DAG (e.g. content added with mode/mtime preservation),
all three mounts show the stored values in `stat` responses regardless of config flags.

To persist mode and mtime when writing through FUSE, enable the opt-in config flags:

```sh
ipfs config --json Mounts.StoreMtime true
ipfs config --json Mounts.StoreMode true
```

These flags change the resulting CID even when file content is identical, because mode and mtime
are stored in the UnixFS DAG node metadata.

See [`Mounts.StoreMtime`](https://github.com/ipfs/kubo/blob/master/docs/config.md#mountsstoremtime) and [`Mounts.StoreMode`](https://github.com/ipfs/kubo/blob/master/docs/config.md#mountsstoremode).

## Troubleshooting

#### `Permission denied` or `fusermount: user has no write access to mountpoint` error in Linux

Verify that the config file can be read by your user:

```sh
sudo ls -l /etc/fuse.conf
-rw-r----- 1 root fuse 216 Jan  2  2013 /etc/fuse.conf
```

In most distributions, the group named `fuse` will be created during fuse
installation. You can check this with:

```sh
sudo grep -q fuse /etc/group && echo fuse_group_present || echo fuse_group_missing
```

If the group is present, just add your regular user to the `fuse` group:

```sh
sudo usermod -G fuse -a <username>
```

If the group didn't exist, create `fuse` group (add your regular user to it) and
set necessary permissions, for example:

```sh
sudo chgrp fuse /etc/fuse.conf
sudo chmod g+r  /etc/fuse.conf
```

Note that the use of `fuse` group is optional and may depend on your operating
system. It is okay to use a different group as long as proper permissions are
set for user running `ipfs mount` command.

#### Mount command crashes and mountpoint gets stuck

```sh
sudo umount /ipfs
sudo umount /ipns
sudo umount /mfs
```

#### Mounting fails with "error mounting: could not resolve name"

Make sure your node's IPNS address has a directory published:

```sh
$ mkdir hello/; echo 'hello world' > hello/hello.txt
$ ipfs add -rQ ./hello/
bafybeidhkumeonuwkebh2i4fc7o7lguehauradvlk57gzake6ggjsy372a

$ ipfs name publish bafybeidhkumeonuwkebh2i4fc7o7lguehauradvlk57gzake6ggjsy372a
```

#### Enabling debug logging

Set the `IPFS_FUSE_DEBUG` environment variable before starting the daemon to log all FUSE operations to stderr:

```sh
IPFS_FUSE_DEBUG=1 ipfs daemon --mount
```
