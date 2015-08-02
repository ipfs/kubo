# go-ipfs changelog

### 0.3.7 - 2015-08-02

This patch update fixes a problem we introduced in 0.3.6 and did not
catch: the webui failed to work with out-of-the-box CORS configs.
This has been fixed and now should work correctly. @jbenet

### 0.3.6 - 2015-07-30

This patch improves the resource consumption of go-ipfs,
introduces a few new options on the CLI, and also
fixes (yet again) windows builds.

* Resource consumption:
  * fixed goprocess memory leak @rht
  * implement batching on datastore @whyrusleeping
  * Fix bitswap memory leak @whyrusleeping
  * let bitswap ignore temporary write errors @whyrusleeping
  * remove logging to disk in favor of api endpoint @whyrusleeping
  * --only-hash option for add to skip writing to disk @whyrusleeping

* Tool changes
  * improved `ipfs daemon` output with all addresses @jbenet
  * improved `ipfs id -f` output, added `<addrs>` and  `\n \t` support @jbenet
  * `ipfs swarm addrs local` now shows the local node's addrs @jbenet
  * improved config json parsing @rht
  * improved Dockerfile to use alpine linux @Luzifer @lgierth
  * improved bash completion @MichaelMure
  * Improved 404 for gateway @cryptix
  * add unixfs ls to list correct filesizes @wking
  * ignore hidden files by default @gatesvp
  * global --timeout flag @whyrusleeping
  * fix random API failures by closing resp bodies @whyrusleeping
  * ipfs swarm filters @whyrusleeping
  * api returns errors in http trailers @whyrusleeping @jbenet
  * `ipfs patch` learned to create intermediate nodes @whyrusleeping
  * `ipfs object stat` now shows Hash @whyrusleeping
  * `ipfs cat` now clears progressbar on exit @rht
  * `ipfs add -w -r <dir>` now wraps directories @jbenet
  * `ipfs add -w <file1> <file2>` now wraps with one dir @jbenet
  * API + Gateway now support arbitrary HTTP Headers from config @jbenet
  * API now supports CORS properly from config @jbenet
  * **Deprecated:** `API_ORIGIN` env var (use config, see `ipfs daemon --help`) @jbenet

* General Codebase
  * `nofuse` tag for windows @Luzifer
  * improved `ipfs add` code @gatesvp
  * started requiring license trailers @chriscool @jbenet
  * removed CtxCloser for goprocess @rht
  * remove deadcode @lgierth @whyrusleeping
  * reduced number of logging libs to 2 (soon to be 1) @rht
  * dial address filtering @whyrusleeping
  * prometheus metrics @lgierth
  * new index page for gateway @krl @cryptix
  * move ping to separate protocol @whyrusleeping
  * add events to bitswap for a dashboard @whyrusleeping
  * add latency and bandwidth options to mocknet @heems
  * levenshtein distance cmd autosuggest @sbruce
  * refactor/cleanup of cmds http handler @whyrusleeping
  * cmds http stream reports errors in trailers @whyrusleeping

* Bugfixes
  * fixed path resolution and validation @rht
  * fixed `ipfs get -C` output and progress bar @rht
  * Fixed install pkg dist bug @jbenet @Luzifer
  * Fix `ipfs get` silent failure   @whyrusleeping
  * `ipfs get` tarx no longer times out @jbenet
  * `ipfs refs -r -u` is now correct @gatesvp
  * Fix `ipfs add -w -r <dir>` wrapping bugs @jbenet
  * Fixed FUSE unmount failures @jbenet
  * Fixed `ipfs log tail` command (api + cli) @whyrusleeping

* Testing
  * sharness updates @chriscool
  * ability to disable secio for testing @jbenet
  * fixed many random test failures, more reliable CI @whyrusleeping
  * Fixed racey notifier failures @whyrusleeping
  * `ipfs refs -r -u` test cases @jbenet
  * Fix failing pinning test @jbenet
  * Better CORS + Referer tests @jbenet
  * Added reversible gc test @rht
  * Fixed bugs in FUSE IPNS tests @whyrusleeping
  * Fixed bugs in FUSE IPFS tests @jbenet
  * Added `random-files` tool for easier sharness tests @jbenet

* Documentation
  * Add link to init system examples @slang800
  * Add CORS documentation to daemon init @carver  (Note: this will change soon)

### 0.3.5 - 2015-06-11

This patch improves overall stability and performance

* added 'object patch' and 'object new' commands @whyrusleeping
* improved symmetric NAT avoidance @jbenet
* move util.Key to blocks.Key @whyrusleeping
* fix memory leak in provider store @whyrusleeping
* updated webui to 0.2.0 @krl
* improved bitswap performance @whyrusleeping
* update fuse lib @cryptix
* fix path resolution @wking
* implement test_seq() in sharness @chriscool
* improve parsing of stdin for commands @chriscool
* fix 'ipfs refs' failing silently @whyrusleeping
* fix serial dialing bug @jbenet
* improved testing @chriscool @rht @jbenet
* fixed domain resolving @luzifer
* fix parsing of unwanted stdin @lgierth
* added CORS handlers to gateway @NodeGuy
* added `ipfs daemon --unrestricted-api` option @krl
* general cleanup of dependencies

### 0.3.4 - 2015-05-10

* fix ipns append bug @whyrusleeping
* fix out of memory panic @whyrusleeping
* add in expvar metrics @tv42
* bitswap improvements @whyrusleeping
* fix write-cache in blockstore @tv42
* vendoring cleanup @cryptix
* added `launchctl` plist for OSX @grncdr
* improved Dockerfile, changed root and mount paths @ehd
* improved `pin ls` output to show types @vitorbaptista

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
