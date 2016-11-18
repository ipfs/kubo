#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "create some files" '
	echo A > fileA &&
	echo B > fileB &&
	echo C > fileC
'

test_expect_success "add files all at once" '
	ipfs add -q fileA fileB fileC > hashes
'

test_expect_success "unpin one of the files" '
	ipfs pin rm `head -1 hashes` > pin-out
'

test_expect_success "unpin output looks good" '
	echo "unpinned `head -1 hashes`" > pin-expect
	test_cmp pin-expect pin-out
'

test_expect_success "create files with same name but in different directories" '
	mkdir dirA &&
	mkdir dirB &&
	echo AA > dirA/fileA &&
	echo BA > dirB/fileA
'

test_expect_success "add files with same name but in different directories" '
	ipfs add -q dirA/fileA dirB/fileA > hashes
'

cat <<EOF | LC_ALL=C sort > cat-expected
AA
BA
EOF

test_expect_success "check that both files are added" '
	cat hashes | xargs ipfs cat | LC_ALL=C sort > cat-actual
	test_cmp cat-expected cat-actual
'

test_expect_success "adding multiple directories fails cleanly" '
	test_must_fail ipfs add -q -r dirA dirB
'

test_expect_success "adding multiple directories with -w is okay" '
	ipfs add -q -r -w dirA dirB > hashes &&
	ipfs ls `tail -1 hashes` > ls-res
'

test_done
