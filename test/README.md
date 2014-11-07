# ipfs whole tests using the [sharness framework](https://github.com/mlafeldt/sharness/)

## Running all the tests

Just use `make` in this directory to run all the tests.

## Running just one test

You can run only one test script by launching it like a regular shell
script:

```
$ ./t0010-basic-commands.sh
```

## Sharness

When running "make" in this directory for the first time, sharness
will be downloaded from its github repo and installed in a "sharness"
directory.

Please do not change anything in the "sharness" directory.

If you really need some changes in sharness, please fork it from
[its cannonical repo](https://github.com/mlafeldt/sharness/) and
send pull requests there.