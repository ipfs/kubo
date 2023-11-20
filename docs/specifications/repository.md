# ![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square) IPFS Repo Spec

**Author(s)**:
- [Juan Benet](github.com/jbenet)

**Abstract**

This spec defines an IPFS Repo, its contents, and its interface. It does not specify how the repo data is actually stored, as that is done via swappable implementations.

# Table of Contents

- [Definition](#definition)
- [Repo Contents](#repo-contents)
  - [version](#version)
  - [datastore](#datastore)
  - [keystore](#keystore)
  - [config (state)](#config-state)
  - [locks](#locks)
  - [datastore\_spec](#datastore_spec)
  - [hooks (TODO)](#hooks-todo)
- [Notes](#notes)

## Definition

A `repo` is the storage repository of an IPFS node. It is the subsystem that
actually stores the data IPFS nodes use. All IPFS objects are stored
in a repo (similar to git).

There are many possible repo implementations, depending on the storage media
used. Most commonly, IPFS nodes use an [fs-repo](repository_fs.md).

Repo Implementations:
- [fs-repo](repository_fs.md) - stored in the os filesystem
- mem-repo - stored in process memory
- s3-repo - stored in amazon s3

## Repo Contents

The Repo stores a collection of [IPLD](https://github.com/ipld/specs#readme) objects that represent:

- **config** - node configuration and settings
- **datastore** - content stored locally, and indexing data
- **keystore** - cryptographic keys, including node's identity
- **hooks** - scripts to run at predefined times (not yet implemented)

Note that the IPLD objects a repo stores are divided into:
- **state** (system, control plane) used for the node's internal state
- **content** (userland, data plane) which represent the user's cached and pinned data.

Additionally, the repo state must determine the following. These need not be IPLD objects, though it is of course encouraged:

- **version** - the repo version, required for safe migrations
- **locks** - process semaphores for correct concurrent access
- **datastore_spec** - array of mounting points and their properties

Finally, the repo also stores the blocks with blobs containing binary data.

![](./ipfs-repo-contents.png)

### version

Repo implementations may change over time, thus they MUST include a `version` recognizable across versions. Meaning that a tool MUST be able to read the `version` of a given repo type.

For example, the `fs-repo` simply includes a `version` file with the version number. This way, the repo contents can evolve over time but the version remains readable the same way across versions.

### datastore

IPFS nodes store some IPLD objects locally. These are either (a) **state objects** required for local operation -- such as the `config` and `keys` -- or (b) **content objects** used to represent data locally available. **Content objects** are either _pinned_ (stored until they are unpinned) or _cached_ (stored until the next repo garbage collection).

The name "datastore" comes from [go-datastore](https://github.com/jbenet/go-datastore), a library for swappable key-value stores. Like its name-sake, some repo implementations feature swappable datastores, for example:
- an fs-repo with a leveldb datastore
- an fs-repo with a boltdb datastore
- an fs-repo with a union fs and leveldb datastore
- an fs-repo with an s3 datastore
- an s3-repo with a cached fs and s3 datastore

This makes it easy to change properties or performance characteristics of a repo without an entirely new implementation.

### keystore

A Repo typically holds the keys a node has access to, for signing and for encryption.

Details on operation and storage of the keystore can be found in [`repository_fs.md`](repository_fs.md) and [`keystore.md`](keystore.md).

### config (state)

The node's `config` (configuration) is a tree of variables, used to configure various aspects of operation. For example:
- the set of bootstrap peers IPFS uses to connect to the network
- the Swarm, API, and Gateway network listen addresses
- the Datastore configuration regarding the construction and operation of the on-disk storage system.

There is a set of properties, which are mandatory for the repo usage. Those are `Addresses`, `Discovery`, `Bootstrap`, `Identity`, `Datastore` and `Keychain`.

It is recommended that `config` files avoid identifying information, so that they may be re-shared across multiple nodes.

**CHANGES**: today, implementations like js-ipfs and go-ipfs store the peer-id and private key directly in the config. These will be removed and moved out.

### locks

IPFS implementations may use multiple processes, or may disallow multiple processes from using the same repo simultaneously. Others may disallow using the same repo but may allow sharing _datastores_ simultaneously. This synchronization is accomplished via _locks_.

All repos contain the following standard locks:
- `repo.lock` - prevents concurrent access to the repo. Must be held to _read_ or _write_.

### datastore_spec

This file is created according to the Datastore configuration specified in the `config` file. It contains an array with all the mounting points that the repo is using, as well as its properties. This way, the `datastore_spec` file must have the same mounting points as defined in the Datastore configuration.

It is important pointing out that the `Datastore` in config must have a `Spec` property, which defines the structure of the ipfs datastore. It is a composable structure, where each datastore is represented by a json object.

### hooks (TODO)

Like git, IPFS nodes will allow `hooks`, a set of user configurable scripts to run at predefined moments in IPFS operations. This makes it easy to customize the behavior of IPFS nodes without changing the implementations themselves.

## Notes

#### A Repo uniquely identifies an IPFS Node

A repository uniquely identifies a node. Running two different IPFS programs with identical repositories -- and thus identical identities -- WILL cause problems.

Datastores MAY be shared -- with proper synchronization -- though note that sharing datastore access MAY erode privacy.

#### Repo implementation changes MUST include migrations

**DO NOT BREAK USERS' DATA.** This is critical. Thus, any changes to a repo's implementation **MUST** be accompanied by a **SAFE** migration tool.

See https://github.com/jbenet/go-ipfs/issues/537 and https://github.com/jbenet/random-ideas/issues/33

#### Repo Versioning

A repo version is a single incrementing integer. All versions are considered non-compatible. Repos of different versions MUST be run through the appropriate migration tools before use.
