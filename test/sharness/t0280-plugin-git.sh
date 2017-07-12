#!/bin/sh
#
# Copyright (c) 2016 Jakub Sztandera
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test git plugin"

. lib/test-lib.sh

# if in travis CI, dont test mount (no fuse)
if ! test_have_prereq PLUGIN; then
	skip_all='skipping git plugin tests, plugins not available'

	test_done
fi

test_init_ipfs

test_expect_success "copy plugin" '
	mkdir -p "$IPFS_PATH/plugins" &&
	cp ../plugins/git.so "$IPFS_PATH/plugins/"
'

# test here

test_done
