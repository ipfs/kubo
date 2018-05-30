Files API
=========

Easy example to generate a code flow that would allow to examine the most important components.

```bash
echo "Contents" > file
ipfs add file
added QmSt12vmyifMbDMvFHWMr24oZChyknrpi1sDUzv1pyhxai file

ipfs files cp /ipfs/QmSt12vmyifMbDMvFHWMr24oZChyknrpi1sDUzv1pyhxai /file
# No output.
# TODO: Is that ok? Could we add a file added legend?

ipfs files cp /ipfs/QmSt12vmyifMbDMvFHWMr24oZChyknrpi1sDUzv1pyhxai /home/user/file
# Error: file does not exist
# TODO: Why is that? WHICH file does not exist? Explain what is happening in the MFS lookup.

ipfs files stat /
# Qmdqzo2zpSv7zzZgwRV3eDXywyMndnd2LC9djrhJSQL71J
# Size: 0
# CumulativeSize: 67
# ChildBlocks: 1
# Type: directory

# TODO: What is that hash? that is not the hash of the file added (it's the root).
# TODO: What is the size of a directory? Is it the same of a unix directory size (no, explain that difference).

ipfs files stat /file
# QmSt12vmyifMbDMvFHWMr24oZChyknrpi1sDUzv1pyhxai
# Size: 9
# CumulativeSize: 17
# ChildBlocks: 0
# Type: file
```


Adding a file
-------------

* Chunking
* Balanced Layout
* Adding the block(s)

```bash
echo "AAAA BBBB CCCC EOF" | IPFS_LOGGING=debug ipfs add
```
* TODO: Maybe adding a file is not the best place to start, maybe start with an already created MFS tree and copy without creating contetns


TODO
====

* Text when successfully added file through `ipfs files cp`.

* What is the root of an MFS file system? (the root is a directory).
