# Kubo changelog vTBD

<a href="https://ipshipyard.com/"><img align="right" src="https://github.com/user-attachments/assets/39ed3504-bb71-47f6-9bf8-cb9a1698f272" /></a>

This release was brought to you by the [Shipyard](https://ipshipyard.com/) team.

- [vTBD](#vtbd)

## vTBD

- [Overview](#overview)
- [🔦 Highlights](#-highlights)
  - [🧪 Opt-in value separation for pebbleds](#-opt-in-value-separation-for-pebbleds)
- [📝 Changelog](#-changelog)
- [👨‍👩‍👧‍👦 Contributors](#-contributors)

### Overview

### 🔦 Highlights

#### 🧪 Opt-in value separation for pebbleds

The experimental `pebbleds` datastore can now keep large values in separate
append-only blob files instead of inside pebble's sstables: set
`"valueSeparationEnabled": true` in the datastore spec. An IPFS block is a
large value behind a small key, and the inline layout forces background
compactions to rewrite unchanged block bytes and key-only scans (the
listing phase of `ipfs repo gc` and `ipfs refs local`) to read through all
block data. On a repo that keeps blocks in pebble (the `pebbleds` profile
default), those scans ran 19-58x faster in synthetic benchmarks, random
block reads 10-30% faster, and ordinary (non-fsync) writes 15-35% slower.
Storage reclamation after deletes is also slower: pebble rewrites a blob
file only once its share of dead bytes exceeds
`valueSeparationTargetGarbageRatio` (default 0.20), so after `ipfs repo gc`
disk usage falls gradually, and up to a fifth of blob-file bytes may stay
unreclaimed.

Value separation is experimental in pebble upstream and requires pebble
database format `24` or newer; a repo that pinned an older
`"formatMajorVersion"` must raise it first, which is irreversible. Read the
precautions in
[datastores.md](https://github.com/ipfs/kubo/blob/master/docs/datastores.md#value-separation-experimental)
before enabling. Nothing changes for nodes that do not opt in; `flatfs`
remains the default and the right choice for most users.

### 📝 Changelog

### 👨‍👩‍👧‍👦 Contributors
