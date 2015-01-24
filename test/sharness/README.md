# ipfs whole tests using the [sharness framework](https://github.com/mlafeldt/sharness/)

## Running all the tests

Just use `make` in this directory to run all the tests.
Run with `TEST_VERBOSE=1` to get helpful verbose output.

```
TEST_VERBOSE=1 make
```

The usual ipfs env flags also apply:

```sh
# the output will make your eyes bleed
IPFS_LOGGING=debug TEST_VERBOSE=1 make
```


## Running just one test

You can run only one test script by launching it like a regular shell
script:

```
$ ./t0010-basic-commands.sh
```

## Sharness

When running "make" in this directory for the first time, sharness
will be downloaded from its github repo and installed in a "lib/sharness"
directory.

Please do not change anything in the "lib/sharness" directory.

If you really need some changes in sharness, please fork it from
[its cannonical repo](https://github.com/mlafeldt/sharness/) and
send pull requests there.

## Writing Tests


### Diagnostics

Make your test case output helpful for when running sharness verbosely.
This means cating certain files, or running diagnostic commands.
For example:

```
test_expect_success ".go-ipfs/ has been created" '
  test -d ".go-ipfs" &&
  test -f ".go-ipfs/config" &&
  test -d ".go-ipfs/datastore" ||
  test_fsh ls -al .go-ipfs
'
```

The `|| ...` is a diagnostic run when the preceding command fails.
test_fsh is a shell function that echoes the args, runs the cmd,
and then also fails, making sure the test case fails. (wouldnt want
the diagnostic accidentally returning true and making it _seem_ like
the test case succeeded!).


### Testing commands on daemon or mounted

Use the provided functions in `lib/test-lib.sh` to run the daemon or mount:

To init, run daemon, and mount in one go:

```sh
test_launch_ipfs_daemon_and_mount

test_expect_success "'ipfs add --help' succeeds" '
  ipfs add --help >actual
'

# other tests here...

# dont forget to kill the daemon!!
test_kill_ipfs_daemon
```

To init, run daemon, and then mount separately:

```sh
test_init_ipfs

# tests inited but not running here

test_launch_ipfs_daemon

# tests running but not mounted here

test_mount_ipfs

# tests mounted here

# dont forget to kill the daemon!!
test_kill_ipfs_daemon
```
