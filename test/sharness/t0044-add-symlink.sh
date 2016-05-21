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
	echo "some text" > files/foo/baz &&
	ln -s files/foo/baz files/bar/baz &&
	ln -s files/does/not/exist files/bad
'

test_add_symlinks() {
	test_expect_success "ipfs add files succeeds" '
		ipfs add -q -r files >filehash_all &&
		tail -n 1 filehash_all >filehash_out
	'

	test_expect_success "output looks good" '
		echo QmWdiHKoeSW8G1u7ATCgpx4yMoUhYaJBQGkyPLkS9goYZ8 > filehash_exp &&
		test_cmp filehash_exp filehash_out
	'

	test_expect_success "adding a symlink adds the link itself" '
		ipfs add -q files/bar/baz > goodlink_out
	'

	test_expect_success "output looks good" '
		echo "QmdocmZeF7qwPT9Z8SiVhMSyKA2KKoA2J7jToW6z6WBmxR" > goodlink_exp &&
		test_cmp goodlink_exp goodlink_out
	'

	test_expect_success "adding a broken symlink works" '
		ipfs add -q files/bad > badlink_out
	'

	test_expect_success "output looks good" '
		echo "QmWYN8SEXCgNT2PSjB6BnxAx6NJQtazWoBkTRH9GRfPFFQ" > badlink_exp &&
		test_cmp badlink_exp badlink_out
	'
}

test_init_ipfs

test_add_symlinks

test_launch_ipfs_daemon

test_add_symlinks

test_kill_ipfs_daemon

test_done
