#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test migrations auto update prompt"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "setup mock migrations" '
	mkdir bin &&
	echo "#!/bin/bash" > bin/fs-repo-migrations &&
	echo "echo 5" >> bin/fs-repo-migrations &&
	chmod +x bin/fs-repo-migrations &&
	export PATH="$(pwd)/bin":$PATH
'

test_expect_success "manually reset repo version to 3" '
	echo "3" > "$IPFS_PATH"/version
'

test_expect_success "ipfs daemon --migrate=false fails" '
	test_expect_code 1 ipfs daemon --migrate=false > false_out
'

test_expect_success "output looks good" '
	grep "Please get fs-repo-migrations from https://dist.ipfs.io" false_out
'

test_expect_success "ipfs daemon --migrate=true runs migration" '
	test_expect_code 1 ipfs daemon --migrate=true > true_out
'

test_expect_success "output looks good" '
	grep "Running: " true_out > /dev/null &&
	grep "Success: fs-repo has been migrated to version 5." true_out > /dev/null
'

test_expect_success "'ipfs daemon' prompts to auto migrate" '
	test_expect_code 1 ipfs daemon > daemon_out 2> daemon_err
'

test_expect_success "output looks good" '
	grep "Found outdated fs-repo" daemon_out > /dev/null &&
	grep "Run migrations now?" daemon_out > /dev/null &&
	grep "Please get fs-repo-migrations from https://dist.ipfs.io" daemon_out > /dev/null
'

test_launch_ipfs_daemon

test_expect_success "build fake dist.ipfs.io" '
	mkdir -p fakedist/fs-repo-migrations/v1.0.0/
	echo "v1.0.0" > fakedist/fs-repo-migrations/versions

	echo "#!/bin/sh" >  fakedist/linux
	echo "echo linux $@" >> fakedist/linux
	tar -czf fakedist/fs-repo-migrations/fs-repo-migrations_v1.0.0_linux-amd64.tar.gz fakedist/linux

	echo "#!/bin/sh" >  fakedist/linux-musl
	echo "echo linux-musl $@" >> fakedist/linux-musl
	tar -czf fakedist/fs-repo-migrations/fs-repo-migrations_v1.0.0_linux-musl-amd64.tar.gz fakedist/linux-musl

	ipfs add -q -r fakedist/ > fakedisthash
'

test_expect_success "detect musl" '
	IPFS_DIST_PATH="http://172.17.0.1:$GWAY_PORT" echo $IPFS_DIST_PATH
'

# make fakedist with executables that just echo "I'm $GOOS-$variant with $ARGV"
# ipfs add -r fakedist
# find out IPFS_DIST_PATH
# run daemon --migrate end-to-end
# check for correct output

test_done
