#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-filestore-lib.sh
. lib/test-lib.sh

test_init_ipfs

test_add_cat_file "add --no-copy" "."

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


test_add_cat_5MB "add --no-copy" "."

test_expect_success "fail after file move" '
    mv mountdir/bigfile mountdir/bigfile2
    test_must_fail ipfs cat "$HASH" >/dev/null
'

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
  test_must_fail ipfs filestore verify > verify_actual &&
  grep -q "changed  QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" verify_actual &&
  grep -q "no-file  QmQ8jJxa1Ts9fKsyUXcdYRHHUkuhJ69f82CF8BNX14ovLT" verify_actual &&
  grep -q "incomplete QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb" verify_actual
'

test_expect_success "tesing re-adding file after change" '
  ipfs add --no-copy mountdir/hello.txt &&
  ipfs filestore ls -q | grep -q QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN
'

cat <<EOF > ls_expect
QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb
QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN
EOF

test_expect_success "tesing filestore clean invalid" '
  ipfs filestore clean invalid > rm-invalid-output &&
  ipfs filestore ls -q | LC_ALL=C sort > ls_actual &&
  test_cmp ls_expect ls_actual
'

cat <<EOF > ls_expect
QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN
EOF

test_expect_success "tesing filestore clean incomplete" '
  ipfs filestore clean incomplete > rm-invalid-output &&
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

test_expect_success "testing filestore rm-dups" '
  ipfs add mountdir/hello.txt > /dev/null &&
  ipfs add --no-copy mountdir/hello.txt > /dev/null &&
  ipfs filestore rm-dups > rm-dups-output &&
  grep -q "duplicate QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN" rm-dups-output &&
  ipfs cat QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN > expected &&
  test_cmp expected mountdir/hello.txt
'

#
# Pin related tests
#

clear_pins() {
    test_expect_success "clearing all pins" '
      ipfs pin ls -q -t recursive > pin_ls &&
      ipfs pin ls -q -t direct >> pin_ls &&
      cat pin_ls | xargs ipfs pin rm > pin_rm &&
      ipfs pin ls -q > pin_ls &&
      test -e pin_ls -a ! -s pin_ls
    '
}

cat <<EOF > add_expect
added QmQhAyoEzSg5JeAzGDCx63aPekjSGKeQaYs4iRf4y6Qm6w adir
added QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb adir/file3
added QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH adir/file1
added QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN adir/file2
EOF

clear_pins

test_expect_success "testing add -r --no-copy" '
  mkdir adir &&
  echo "Hello Worlds!" > adir/file1 &&
  echo "HELLO WORLDS!" > adir/file2 &&
  random 5242880 41 > adir/file3 &&
  ipfs add --no-copy -r adir | LC_ALL=C sort > add_actual &&
  test_cmp add_expect add_actual
'

test_expect_success "testing rm of indirect pinned file" '
  test_must_fail ipfs filestore rm QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN
'

test_expect_success "testing forced rm of indirect pinned file" '
  ipfs filestore rm --force QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN
'


cat <<EOF > pin_ls_expect
QmQhAyoEzSg5JeAzGDCx63aPekjSGKeQaYs4iRf4y6Qm6w direct
QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb recursive
QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH recursive
EOF

test_expect_success "testing filestore fix-pins" '
  ipfs filestore fix-pins > fix_pins_actual &&
  ipfs pin ls | LC_ALL=C sort | grep -v " indirect" > pin_ls_actual &&
  test_cmp pin_ls_expect pin_ls_actual
'

clear_pins

cat <<EOF > pin_ls_expect
QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb recursive
QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH recursive
EOF

test_expect_success "testing filestore fix-pins --skip-root" '
  ipfs add --no-copy -r adir > add_actual &&
  ipfs filestore rm --force QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN > rm_actual
  ipfs filestore fix-pins --skip-root > fix_pins_actual &&
  ipfs pin ls | LC_ALL=C sort | grep -v " indirect" > pin_ls_actual &&
  test_cmp pin_ls_expect pin_ls_actual
'

clear_pins

cat <<EOF > unpinned_expect
QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb
QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH
EOF

test_expect_success "testing filestore unpinned" '
  ipfs filestore unpinned  | LC_ALL=C sort > unpinned_actual &&
  test_cmp unpinned_expect unpinned_actual
'

test_done
