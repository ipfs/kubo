#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add -w"

. lib/test-lib.sh

test_expect_success "creating files succeeds" '
	mkdir -p files/foo &&
	mkdir -p files/bar &&
	mkdir -p files/badin
	echo "some text" > files/foo/baz &&
	ln -s ../foo/baz files/bar/baz &&
	ln -s files/does/not/exist files/badin/bad &&
	mkdir -p files2/a/b/c &&
	echo "some other text" > files2/a/b/c/foo &&
	ln -s b files2/a/d
'

test_add_symlinks() {
	test_expect_success "ipfs add files succeeds" '
		ipfs add -q -r files >filehash_all &&
		tail -n 1 filehash_all >filehash_out
	'

	test_expect_success "output looks good" '
		echo QmQRgZT6xVFKJLVVpJDu3WcPkw2iqQ1jqK1F9jmdeq9zAv > filehash_exp &&
		test_cmp filehash_exp filehash_out
	'

	test_expect_success "adding a symlink adds the file itself" '
		ipfs add -q files/bar/baz > goodlink_out
	'

	test_expect_success "output looks good" '
		echo QmcPNXE5zjkWkM24xQ7Bi3VAm8fRxiaNp88jFsij7kSQF1 > goodlink_exp &&
		test_cmp goodlink_exp goodlink_out
	'

	test_expect_success "adding a broken symlink works" '
		ipfs add -qr files/badin | head -1 > badlink_out
	'

	test_expect_success "output looks good" '
		echo "QmWYN8SEXCgNT2PSjB6BnxAx6NJQtazWoBkTRH9GRfPFFQ" > badlink_exp &&
		test_cmp badlink_exp badlink_out
	'

	test_expect_success "adding with symlink in middle of path is same as\
adding with no symlink" '
		ipfs add -rq files2/a/b/c > no_sym &&
		ipfs add -rq files2/a/d/c > sym &&
		test_cmp no_sym sym
	'
}

test_init_ipfs

test_add_symlinks

test_launch_ipfs_daemon

test_add_symlinks

test_kill_ipfs_daemon

test_done
