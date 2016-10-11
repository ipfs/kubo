#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test adding multiple directories"


. lib/test-lib.sh

test_expect_success "make some setup directories" '
	mkdir -p a/foo/bar/fish &&
	mkdir -p a/foo/cats &&
	mkdir -p a/dogs/baz &&
	echo "stuff" > a/foo/bar/fish/food &&
	echo "blah blah" > a/foo/cats/meow &&
	echo "ipfs is cool" > a/dogs/baz/bazbaz &&

	mkdir -p other/a/foo/cats &&
	mkdir -p other/a/ipfs &&
	echo "meoooowwww" > other/a/foo/cats/meow &&
	echo "hashing hashing" > other/a/ipfs/blah &&

	mkdir -p something/example &&
	echo "example" > something/example/text
'

test_add_many_r() {
  test_expect_success "add multiple directories at the same time" '
	ipfs add -r a other/a something > add_out
  '

  test_expect_success "output looks correct" '
	grep "added QmU5Ekbnb9H8bLz6ECEnMDnkEZh5JoY9tXHGCBxnsLqsDg a/foo/cats/meow" add_out &&
	grep "added QmNwTsPq4BEBwuQ1aFUPZ6ux3P7GnupfQd1mBre19iSej5 other/a/foo/cats/meow" add_out &&
	grep "added QmTHApDiVaLa2yc8G6Jg6booheoFGffWuSKTqwQ4zjP6fQ a" add_out &&
	grep "added QmSsiWzY9yxtcRMsKvYivDKzBN8kftAgsVjpyGAxstj4Aw other" add_out &&
	grep "added QmWGDSqhr53etAVa35XBCmknWvMuuUU5VMS7zf3TYNnm89 something" add_out
  '
}

test_init_ipfs

test_add_many_r

test_launch_ipfs_daemon

test_add_many_r

test_kill_ipfs_daemon

test_done
