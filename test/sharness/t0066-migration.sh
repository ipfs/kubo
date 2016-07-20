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
	echo "echo 4" >> bin/fs-repo-migrations &&
	chmod +x bin/fs-repo-migrations &&
	export PATH="$(pwd)/bin":$PATH
'

test_expect_success "manually reset repo version to 3" '
	echo "3" > "$IPFS_PATH"/version
'

test_expect_success "ipfs daemon --migrate=false fails" '
	test_expect_code 1 ipfs daemon --migrate=false 2> false_out
'

test_expect_success "output looks good" '
	grep "please run the migrations manually" false_out
'

test_expect_success "ipfs daemon --migrate=true runs migration" '
	test_expect_code 1 ipfs daemon --migrate=true > true_out
'

test_expect_success "output looks good" '
	grep "running migration" true_out > /dev/null &&
	grep "binary completed successfully" true_out > /dev/null
'

test_expect_success "'ipfs daemon' prompts to auto migrate" '
	test_expect_code 1 ipfs daemon > daemon_out 2> daemon_err
'

test_expect_success "output looks good" '
	grep "Found old repo version" daemon_out > /dev/null &&
	grep "Run migrations automatically?" daemon_out > /dev/null &&
	grep "please run the migrations manually" daemon_err > /dev/null
'

test_done
