#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add --no-copy"

. lib/test-lib.sh

client_err() {
    printf "$@\n\nUse 'ipfs add --help' for information about this command\n"
}

test_add_cat_file() {
    test_expect_success "ipfs add succeeds" '
    	echo "Hello Worlds!" >mountdir/hello.txt &&
        ipfs add --no-copy mountdir/hello.txt >actual
    '

    test_expect_success "ipfs add output looks good" '
    	HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
        echo "added $HASH hello.txt" >expected &&
    	test_cmp expected actual
    '

    test_expect_success "ipfs cat succeeds" '
    	ipfs cat "$HASH" >actual
    '

    test_expect_success "ipfs cat output looks good" '
    	echo "Hello Worlds!" >expected &&
    	test_cmp expected actual
    '

    test_expect_success "fail after file move" '
        mv mountdir/hello.txt mountdir/hello2.txt
    	test_must_fail ipfs cat "$HASH" >/dev/null
    '

    test_expect_success "okay again after moving back" '
        mv mountdir/hello2.txt mountdir/hello.txt
    	ipfs cat "$HASH" >/dev/null
    '
    
    test_expect_success "fail after file change" '
        # note: filesize shrinks
    	echo "hello world!" >mountdir/hello.txt &&
    	test_must_fail ipfs cat "$HASH" >cat.output
    '

    test_expect_success "fail after file change, same size" '
        # note: filesize does not change
    	echo "HELLO WORLDS!" >mountdir/hello.txt &&
    	test_must_fail ipfs cat "$HASH" >cat.output
    '
}

test_add_cat_5MB() {
    test_expect_success "generate 5MB file using go-random" '
    	random 5242880 41 >mountdir/bigfile
    '

    test_expect_success "sha1 of the file looks ok" '
    	echo "11145620fb92eb5a49c9986b5c6844efda37e471660e" >sha1_expected &&
    	multihash -a=sha1 -e=hex mountdir/bigfile >sha1_actual &&
    	test_cmp sha1_expected sha1_actual
    '

    test_expect_success "'ipfs add bigfile' succeeds" '
    	ipfs add --no-copy mountdir/bigfile >actual
    '

    test_expect_success "'ipfs add bigfile' output looks good" '
    	HASH="QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb" &&
    	echo "added $HASH bigfile" >expected &&
    	test_cmp expected actual
    '
    test_expect_success "'ipfs cat' succeeds" '
    	ipfs cat "$HASH" >actual
    '

    test_expect_success "'ipfs cat' output looks good" '
    	test_cmp mountdir/bigfile actual
    '

    test_expect_success "fail after file move" '
        mv mountdir/bigfile mountdir/bigfile2
    	test_must_fail ipfs cat "$HASH" >/dev/null
    '
}

test_init_ipfs

test_add_cat_file

test_add_cat_5MB

# check "ipfs filestore " cmd by using state left by add commands

cat <<EOF > ls_expect
QmQ8jJxa1Ts9fKsyUXcdYRHHUkuhJ69f82CF8BNX14ovLT
QmQNcknfZjsABxg2bwxZQ9yqoUZW5dtAfCK3XY4eadjnxZ
QmQnNhFzUjVRMHxafWaV2z7XZV8no9xJTdybMZbhgZ7776
QmSY1PfYxzxJfQA3A19NdZGAu1fZz33bPGAhcKx82LMRm2
QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb
QmTFH6xrLxiwC7WRwou2QkvgZwVSdQNHc1uGfPDNBqH2rK
QmTbkLhagokC5NsxRLC2fanadqzFdTCdBB7cJWCg3U2tgL
QmTvvmPaPBHRAo2CTvQC6VRYJaMwsFigDbsqhRjLBDypAa
QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH
QmWgZKyDJzixHydY5toiJ2EHFdDkooWJnvH5uixY4rhq2W
QmYNVKQFvW3UwUDGoGSS68eBBYSuFY8RVp7UTinkY8zkYv
QmZBe6brSjd2XBzAyqJRAYnNm3qRYR4BXk8Akfuot7fuSY
QmayX17gA63WRmcZkQuJGcDAv1hWP4ULpXaPUHSf7J6UbC
Qmb6wyFUBKshoaFRfh3xsdbrRF9WA5sdp62R6nWEtgjSEK
QmcZm5DH1JpbWkNnXsCXMioaQzXqcq7AmoQ3BK5Q9iWXJc
Qmcp8vWcq2xLnAum4DPqf3Pfr2Co9Hsj7kxkg4FxUAC4EE
QmeXTdS4ZZ99AcTg6w3JwndF3T6okQD17wY1hfRR7qQk8f
QmeanV48k8LQxWMY1KmoSAJiF6cSm1DtCsCzB5XMbuYNeZ
Qmej7SUFGehBVajSUpW4psbrMzcSC9Zip9awX9anLvofyZ
QmeomcMd37LRxkYn69XKiTpGEiJWRgUNEaxADx6ssfUJhp
QmfAGX7cH2G16Wb6tzVgVjwJtphCz3SeuRqvFmGuVY3C7D
QmfYBbC153rBir5ECS2rzrKVUEer6rgqbRpriX2BviJHq1
EOF

test_expect_success "testing filestore ls" '
  ipfs filestore ls -q | LC_ALL=C sort > ls_actual &&
  test_cmp ls_expect ls_actual
'
test_expect_success "testing filestore verify" '
  ipfs filestore verify > verify_actual &&
  grep -q "changed  QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" verify_actual &&
  grep -q "missing  QmQ8jJxa1Ts9fKsyUXcdYRHHUkuhJ69f82CF8BNX14ovLT" verify_actual
'

test_expect_success "tesing re-adding file after change" '
  ipfs add --no-copy mountdir/hello.txt &&
  ipfs filestore ls -q | grep -q QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN
'

cat <<EOF > ls_expect
QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb
QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN
EOF

test_expect_success "tesing filestore rm-invalid" '
  ipfs filestore rm-invalid missing > rm-invalid-output &&
  ipfs filestore ls -q | LC_ALL=C sort > ls_actual &&
  test_cmp ls_expect ls_actual
'

test_expect_success "re-added file still available" '
  ipfs cat QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN > expected &&
  test_cmp expected mountdir/hello.txt
'

test_expect_success "testing filestore rm" '
  ipfs filestore rm QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN
'

test_expect_success "testing file removed" '
  test_must_fail cat QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN > expected
'

test_done
