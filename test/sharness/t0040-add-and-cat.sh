#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

test_launch_ipfs_daemon_and_mount

test_expect_success "'ipfs add --help' succeeds" '
	ipfs add --help >actual
'

test_expect_success "'ipfs add --help' output looks good" '
	egrep "ipfs add.*<path>" actual >/dev/null ||
	test_fsh cat actual
'

test_expect_success "'ipfs cat --help' succeeds" '
	ipfs cat --help >actual
'

test_expect_success "'ipfs cat --help' output looks good" '
	egrep "ipfs cat.*<ipfs-path>" actual >/dev/null ||
	test_fsh cat actual
'

test_expect_success "ipfs add succeeds" '
	echo "Hello Worlds!" >mountdir/hello.txt &&
	ipfs add mountdir/hello.txt >actual
'

test_expect_success "ipfs add output looks good" '
	HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
	echo "added $HASH mountdir/hello.txt" >expected &&
	test_cmp expected actual
'

test_expect_success "ipfs add --only-hash succeeds" '
	ipfs add --only-hash mountdir/hello.txt > oh_actual
'

test_expect_success "ipfs add --only-hash output looks good" '
	ipfs add --only-hash mountdir/hello.txt > oh_actual
'

test_expect_success "ipfs cat succeeds" '
	ipfs cat "$HASH" >actual
'

test_expect_success "ipfs cat output looks good" '
	echo "Hello Worlds!" >expected &&
	test_cmp expected actual
'

test_expect_success "ipfs cat succeeds with stdin opened (issue #1141)" '
	cat mountdir/hello.txt | while read line; do ipfs cat "$HASH" >actual || exit; done
'

test_expect_success "ipfs cat output looks good" '
	test_cmp expected actual
'

test_expect_success "ipfs cat accept hash from stdin" '
	echo "$HASH" | ipfs cat >actual
'

test_expect_success "ipfs cat output looks good" '
	test_cmp expected actual
'

test_expect_success FUSE "cat ipfs/stuff succeeds" '
	cat "ipfs/$HASH" >actual
'

test_expect_success FUSE "cat ipfs/stuff looks good" '
	test_cmp expected actual
'

test_expect_success "'ipfs add -q' succeeds" '
	echo "Hello Venus!" >mountdir/venus.txt &&
	ipfs add -q mountdir/venus.txt >actual
'

test_expect_success "'ipfs add -q' output looks good" '
	HASH="QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4" &&
	echo "$HASH" >expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs add -q' with stdin input succeeds" '
	echo "Hello Jupiter!" | ipfs add -q >actual
'

test_expect_success "'ipfs add -q' output looks good" '
	HASH="QmUnvPcBctVTAcJpigv6KMqDvmDewksPWrNVoy1E1WP5fh" &&
	echo "$HASH" >expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs cat' succeeds" '
	ipfs cat "$HASH" >actual
'

test_expect_success "ipfs cat output looks good" '
	echo "Hello Jupiter!" >expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs add' with stdin input succeeds" '
	printf "Hello Neptune!\nHello Pluton!" | ipfs add >actual
'

test_expect_success "'ipfs add' output looks good" '
	HASH="QmZDhWpi8NvKrekaYYhxKCdNVGWsFFe1CREnAjP1QbPaB3" &&
	echo "added $HASH " >expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs cat' with stdin input succeeds" '
	echo "$HASH" | ipfs cat >actual
'

test_expect_success "ipfs cat with stdin input output looks good" '
	printf "Hello Neptune!\nHello Pluton!" >expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs add -r' succeeds" '
	mkdir mountdir/planets &&
	echo "Hello Mars!" >mountdir/planets/mars.txt &&
	echo "Hello Venus!" >mountdir/planets/venus.txt &&
	ipfs add -r mountdir/planets >actual
'

test_expect_success "'ipfs add -r' output looks good" '
	PLANETS="QmWSgS32xQEcXMeqd3YPJLrNBLSdsfYCep2U7CFkyrjXwY" &&
	MARS="QmPrrHqJzto9m7SyiRzarwkqPcCSsKR2EB1AyqJfe8L8tN" &&
	VENUS="QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4" &&
	echo "added $MARS mountdir/planets/mars.txt" > expected &&
	echo "added $VENUS mountdir/planets/venus.txt" >> expected &&
	echo "added $PLANETS mountdir/planets" >> expected &&
	test_cmp expected actual
'
test_expect_success "'ipfs add -r' treats . as ignore" '
       echo "Hello Mars!" >mountdir/planets/mars.txt &&
       echo "Hello Venus!" >mountdir/planets/venus.txt &&
       echo "ignore*" >mountdir/planets/.ipfsignore &&
       ipfs add -r mountdir/planets >actual
       PLANETS="QmWSgS32xQEcXMeqd3YPJLrNBLSdsfYCep2U7CFkyrjXwY" &&
       MARS="QmPrrHqJzto9m7SyiRzarwkqPcCSsKR2EB1AyqJfe8L8tN" &&
       VENUS="QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4" &&
       echo "added $MARS mountdir/planets/mars.txt" > expected &&
       echo "added $VENUS mountdir/planets/venus.txt" >> expected &&
       echo "added $PLANETS mountdir/planets" >> expected &&
       test_cmp expected actual
'

test_expect_success "'ipfs add -r --hidden' will include hidden file" '
	ipfs add -r --hidden mountdir/planets > actual 
	IGNORE="QmcPE2m7ori3WozZT6HzAxLP7nSUNAfgRENUEkKpfvAerJ" &&
	PLANETS="QmQALn8EZGDAPUKijt56mYLdqTNvxMAZBAoQpDjV7G1A8D" &&
	MARS="QmPrrHqJzto9m7SyiRzarwkqPcCSsKR2EB1AyqJfe8L8tN" &&
	VENUS="QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4" &&
	echo "added $IGNORE mountdir/planets/.ipfsignore" > expected &&
	echo "added $MARS mountdir/planets/mars.txt" >> expected &&
	echo "added $VENUS mountdir/planets/venus.txt" >> expected &&
	echo "added $PLANETS mountdir/planets" >> expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs add -r' respects .ipfsignore in active folder" '
       echo "Hello Mars!" >mountdir/planets/mars.txt &&
       echo "Hello Venus!" >mountdir/planets/venus.txt &&
       echo "ignore*" >mountdir/planets/.ipfsignore &&
       echo "this file should be ignored" >mountdir/planets/ignore_me.txt &&
       ipfs add -r mountdir/planets >actual
       PLANETS="QmWSgS32xQEcXMeqd3YPJLrNBLSdsfYCep2U7CFkyrjXwY" &&
       MARS="QmPrrHqJzto9m7SyiRzarwkqPcCSsKR2EB1AyqJfe8L8tN" &&
       VENUS="QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4" &&
       echo "added $MARS mountdir/planets/mars.txt" >expected &&
       echo "added $VENUS mountdir/planets/venus.txt" >>expected &&
       echo "added $PLANETS mountdir/planets" >>expected &&
       test_cmp expected actual
'

test_expect_success "'ipfs add -r' respects root .ipfsignore files" '
       mkdir -p mountdir/planets/moons &&
       echo "mars*" >mountdir/.ipfsignore &&
       echo "ignore*" >mountdir/planets/.ipfsignore &&
       echo "europa*" >mountdir/planets/moons/.ipfsignore &&
       echo "Hello Mars!" >mountdir/planets/mars.txt &&
       echo "Hello Venus!" >mountdir/planets/venus.txt &&
       echo "Hello IO!" >mountdir/planets/moons/io.txt &&
       echo "Hello Europa!" >mountdir/planets/moons/europa.txt &&
       echo "this file should be ignored" >mountdir/planets/ignore_me.txt &&
       ipfs add -r mountdir/planets > recursive_ignore &&
       echo "added QmTXxVggAJ1Jh3ZZgDteSMcrxX9T6VgxdxUisKmZMy2qdT mountdir/planets/moons/io.txt" > expected
       echo "added Qmd9A64XXUgHqUkkxNiZww2C2yxm3uPbrfvvLfCujDFFGq mountdir/planets/moons" >> expected
       echo "added QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4 mountdir/planets/venus.txt" >> expected
       echo "added Qmc5Lpt1LczLsbvcx4AkX9YMJbfRYfwVyHufgYpP41SJBB mountdir/planets" >> expected
       test_cmp expected recursive_ignore
       #cat recursive_ignore
'

test_expect_success "ipfs cat accept many hashes from stdin" '
	{ echo "$MARS"; echo "$VENUS"; } | ipfs cat >actual
'

test_expect_success "ipfs cat output looks good" '
	cat mountdir/planets/mars.txt mountdir/planets/venus.txt >expected &&
	test_cmp expected actual
'

test_expect_success "ipfs cat accept many hashes as args" '
	ipfs cat "$MARS" "$VENUS" >actual
'

test_expect_success "ipfs cat output looks good" '
	test_cmp expected actual
'

test_expect_success "ipfs cat with both arg and stdin" '
	echo "$MARS" | ipfs cat "$VENUS" >actual
'

test_expect_success "ipfs cat output looks good" '
	cat mountdir/planets/venus.txt >expected &&
	test_cmp expected actual
'

test_expect_success "ipfs cat with two args and stdin" '
	echo "$MARS" | ipfs cat "$VENUS" "$VENUS" >actual
'

test_expect_success "ipfs cat output looks good" '
	cat mountdir/planets/venus.txt mountdir/planets/venus.txt >expected &&
	test_cmp expected actual
'

test_expect_success "go-random is installed" '
	type random
'

test_expect_success "generate 5MB file using go-random" '
	random 5242880 41 >mountdir/bigfile
'

test_expect_success "sha1 of the file looks ok" '
	echo "11145620fb92eb5a49c9986b5c6844efda37e471660e" >sha1_expected &&
	multihash -a=sha1 -e=hex mountdir/bigfile >sha1_actual &&
	test_cmp sha1_expected sha1_actual
'

test_expect_success "'ipfs add bigfile' succeeds" '
	ipfs add mountdir/bigfile >actual
'

test_expect_success "'ipfs add bigfile' output looks good" '
	HASH="QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb" &&
	echo "added $HASH mountdir/bigfile" >expected &&
	test_cmp expected actual
'
test_expect_success "'ipfs cat' succeeds" '
	ipfs cat "$HASH" >actual
'

test_expect_success "'ipfs cat' output looks good" '
	test_cmp mountdir/bigfile actual
'

test_expect_success FUSE "cat ipfs/bigfile succeeds" '
	cat "ipfs/$HASH" >actual
'

test_expect_success FUSE "cat ipfs/bigfile looks good" '
	test_cmp mountdir/bigfile actual
'

test_expect_success EXPENSIVE "generate 100MB file using go-random" '
	random 104857600 42 >mountdir/bigfile
'

test_expect_success EXPENSIVE "sha1 of the file looks ok" '
	echo "1114885b197b01e0f7ff584458dc236cb9477d2e736d" >sha1_expected &&
	multihash -a=sha1 -e=hex mountdir/bigfile >sha1_actual &&
	test_cmp sha1_expected sha1_actual
'

test_expect_success EXPENSIVE "ipfs add bigfile succeeds" '
	ipfs add mountdir/bigfile >actual
'

test_expect_success EXPENSIVE "ipfs add bigfile output looks good" '
	HASH="QmU9SWAPPmNEKZB8umYMmjYvN7VyHqABNvdA6GUi4MMEz3" &&
	echo "added $HASH mountdir/bigfile" >expected &&
	test_cmp expected actual
'

test_expect_success EXPENSIVE "ipfs cat succeeds" '
	ipfs cat "$HASH" | multihash -a=sha1 -e=hex >sha1_actual
'

test_expect_success EXPENSIVE "ipfs cat output looks good" '
	ipfs cat "$HASH" >actual &&
	test_cmp mountdir/bigfile actual
'

test_expect_success EXPENSIVE "ipfs cat output hashed looks good" '
	echo "1114885b197b01e0f7ff584458dc236cb9477d2e736d" >sha1_expected &&
	test_cmp sha1_expected sha1_actual
'

test_expect_success FUSE,EXPENSIVE "cat ipfs/bigfile succeeds" '
	cat "ipfs/$HASH" | multihash -a=sha1 -e=hex >sha1_actual
'

test_expect_success FUSE,EXPENSIVE "cat ipfs/bigfile looks good" '
	test_cmp sha1_expected sha1_actual
'

test_expect_success "ipfs add -w succeeds" '
	ipfs add -w mountdir/hello.txt >actual
'

test_expect_success "ipfs add -w output looks good" '
	HASH="QmVJfrqd4ogGZME6LWkkikAGddYgh9dBs2U14DHZZUBk7W" &&
	echo "added $HASH/hello.txt mountdir/hello.txt" >expected &&
	test_cmp expected actual
'

test_kill_ipfs_daemon

test_done
