#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-filestore-lib.sh
. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon

test_add_cat_file "filestore add " "`pwd`"

test_post_add "filestore add " "`pwd`"

test_add_cat_5MB "filestore add " "`pwd`"

test_expect_success "ipfs add -S fails unless enable" '
  echo "Hello Worlds!" >mountdir/hello.txt &&
  test_must_fail ipfs filestore add -S "`pwd`"/mountdir/hello.txt >actual
'

test_expect_success "filestore mv should fail" '
  HASH=QmQHRQ7EU8mUXLXkvqKWPubZqtxYPbwaqYo6NXSfS9zdCc &&
  random 5242880 42 >mountdir/bigfile-42 &&
  ipfs add mountdir/bigfile-42 &&
  test_must_fail ipfs filestore mv $HASH "`pwd`/mountdir/bigfile-42-also"
'

test_kill_ipfs_daemon

test_expect_success "clean filestore" '
  ipfs filestore ls -q | xargs ipfs filestore rm &&
  test -z "`ipfs filestore ls -q`"
'

test_expect_success "enable Filestore.APIServerSidePaths" '
  ipfs config Filestore.APIServerSidePaths --bool true
'

test_launch_ipfs_daemon

test_add_cat_file "filestore add -S" "`pwd`"

test_post_add "filestore add -S" "`pwd`"

test_add_cat_5MB "filestore add -S" "`pwd`"

cat <<EOF > add_expect
added QmQhAyoEzSg5JeAzGDCx63aPekjSGKeQaYs4iRf4y6Qm6w adir
added QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb adir/file3
added QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH adir/file1
added QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN adir/file2
EOF

test_expect_success "testing filestore add -S -r" '
  mkdir adir &&
  echo "Hello Worlds!" > adir/file1 &&
  echo "HELLO WORLDS!" > adir/file2 &&
  random 5242880 41 > adir/file3 &&
  ipfs filestore add -S -r "`pwd`/adir" | LC_ALL=C sort > add_actual &&
  test_cmp add_expect add_actual &&
  ipfs cat QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH > cat_actual
  test_cmp adir/file1 cat_actual
'

test_expect_success "filestore mv" '
  HASH=QmQHRQ7EU8mUXLXkvqKWPubZqtxYPbwaqYo6NXSfS9zdCc &&
  test_must_fail ipfs filestore mv $HASH "mountdir/bigfile-42-also" &&
  ipfs filestore mv $HASH "`pwd`/mountdir/bigfile-42-also"
'

test_kill_ipfs_daemon

test_done
