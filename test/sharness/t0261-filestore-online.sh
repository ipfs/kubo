#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-filestore-lib.sh
. lib/test-lib.sh

test_init_ipfs

test_expect_success "enable API.ServerSideAdds" '
  ipfs config API.ServerSideAdds --bool true
'

test_launch_ipfs_daemon

test_add_cat_file "add-ss --no-copy" "`pwd`"

test_add_cat_5MB "add-ss --no-copy" "`pwd`"

cat <<EOF > add_expect
added QmQhAyoEzSg5JeAzGDCx63aPekjSGKeQaYs4iRf4y6Qm6w adir
added QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb adir/file3
added QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH adir/file1
added QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN adir/file2
EOF

test_expect_success "testing add-ss -r --no-copy" '
  mkdir adir &&
  echo "Hello Worlds!" > adir/file1 &&
  echo "HELLO WORLDS!" > adir/file2 &&
  random 5242880 41 > adir/file3 &&
  ipfs add-ss --no-copy -r "`pwd`/adir" | LC_ALL=C sort > add_actual &&
  test_cmp add_expect add_actual &&
  ipfs cat QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH > cat_actual
  test_cmp adir/file1 cat_actual
'

test_kill_ipfs_daemon

test_done
