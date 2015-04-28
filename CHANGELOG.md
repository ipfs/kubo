# go-ipfs changelog

### 0.3.3 - 2015-04-28

This patch update fixes various issues, in particular:
- windows support (0.3.0 had broken it)
- commandline parses spaces correctly.

* much improved commandline parsing by @AtnNn
* improved dockerfile by @luzifer
* add cmd cleanup by @wking
* fix flatfs windows support by @tv42 and @gatesvp
* test case improvements by @chriscool
* ipns resolution timeout bug fix by @whyrusleeping
* new cluster tests with iptb by @whyrusleeping
* fix log callstack printing bug by @whyrusleeping
* document bash completiong by @dylanPowers

### 0.3.2 - 2015-04-22

This patch update implements multicast dns as well as fxing a few test issues.

* implment mdns peer discovery @whyrusleeping
* fix mounting issues in sharness tests @chriscool

### 0.3.1 - 2015-04-21

This patch update fixes a few bugs:

* harden shutdown logic by @torarnv
* daemon locking fixes by @travisperson
* don't re-add entire dirs by @whyrusleeping
* tests now wait for graceful shutdown by @jbenet
* default key size is now 2048 by @jbenet

### 0.3.0 - 2015-04-20

We've just released version 0.3.0, which contains many
performance improvements, bugfixes, and new features.
Perhaps the most noticeable change is moving block storage
from leveldb to flat files in the filesystem.

What to expect:

* _much faster_ performance

* Repo format 2
  * moved default location from ~/.go-ipfs -> ~/.ipfs
  * renamed lock filename daemon.lock -> repo.lock
  * now using a flat-file datastore for local blocks

* Fixed lots of bugs
  * proper ipfs-path in various commands
  * fixed two pinning bugs (recursive pins)
  * increased yamux streams window (for speed)
  * increased bitswap workers (+ env var)
  * fixed memory leaks
  * ipfs add error returns
  * daemon exit bugfix
  * set proper UID and GID on fuse mounts

* Gateway
  * Added support for HEAD requests

* configuration
  * env var to turn off SO_REUSEPORT: IPFS_REUSEPORT=false
  * env var to increase bitswap workers: IPFS_BITSWAP_TASK_WORKERS=n

* other
  * bash completion is now available
  * ipfs stats bw -- bandwidth meetering

And many more things.

### 0.2.3 - 2015-03-01

* Alpha Release

### 2015-01-31:

* bootstrap addresses now have .../ipfs/... in format
  config file Bootstrap field changed accordingly. users
  can upgrade cleanly with:

      ipfs bootstrap >boostrap_peers
      ipfs bootstrap rm --all
      <install new ipfs>
      <manually add .../ipfs/... to addrs in bootstrap_peers>
      ipfs bootstrap add <bootstrap_peers
