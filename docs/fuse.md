As a golang project, `go-ipfs` is easily downloaded and installed with `go get github.com/jbenet/go-ipfs`. All data is stored in a leveldb data store in `~/.go-ipfs/datastore`. If, however, you would like to mount the datastore (`ipfs mount /ipfs`) and use it as you would a normal filesystem, you will need to install fuse.

As a precursor, you will have to create the `/ipfs` and `/ipns` directories explicitly, as they require sudo permissions to create (`sudo mkdir /ipfs; sudo mkdir /ipns`). Then `chown` them so `ipfs` can access them without root permission, eg `sudo chown <username>:<groupname> /ipfs` and same for `/ipns`.

MacOSX (Mountain Lion)
----------------------

It has been discovered that versions of `osxfuse` prior to `2.7.0` will cause a kernel panic. For everyone's sake, please upgrade (latest at time of writing is `2.7.2`). The disk image can be found at https://osxfuse.github.io/. There is also a homebrew formula, if you like, `brew install osxfuse`.

Note that `ipfs` attempts an automatic version check on `osxfuse` to prevent you from shooting yourself in the foot if you have pre `2.7.0`. However, that version check has become unreliable, and may fail even if you are up-to-date. If you are sure about your `osxfuse` version and would like to mount, simply add `return nil` at the top of this function: https://github.com/jbenet/go-ipfs/blob/master/cmd/ipfs/mount_darwin.go#L15. Then reinstall by running `go install` in `$GOPATH/src/github.com/jbenet/go-ipfs/cmd/ipfs`

You should be good to go! If you're not, hop on IRC and speak with us, or if you figure something out, add to this wiki!

Linux (Ubuntu 14.04)
--------------------

Install Fuse with `sudo apt-get install fuse`. 
Change permissions on the fuse config: `sudo chown <username>:<groupname> /etc/fuse.conf`
(note `<groupname>` should probably be `fuse`)

If the mount command crashes and your mountpoint gets stuck, `sudo umount /ipfs` 


Other
-----
If you manage to mount on other systems (or followed an alternative path to one above), please contribute to these docs :D
