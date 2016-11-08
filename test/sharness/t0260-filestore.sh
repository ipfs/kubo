#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-filestore-lib.sh
. lib/test-lib.sh

test_init_ipfs

test_add_cat_file "add --no-copy" "`pwd`" "QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH"

test_post_add "add --no-copy" "`pwd`"

test_add_cat_file "add --no-copy --raw-leaves" "`pwd`" "zdvgqC4vX1j7higiYBR1HApkcjVMAFHwJyPL8jnKK6sVMqd1v"

test_post_add "add --no-copy --raw-leaves" "`pwd`"

test_add_empty_file "add --no-copy" "`pwd`"

test_add_cat_5MB "add --no-copy" "`pwd`" "QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb"

test_add_cat_5MB "add --no-copy  --raw-leaves" "`pwd`" "QmefsDaD3YVphd86mxjJfPLceKv8by98aB6J6sJxK13xS2"

test_add_mulpl_files "add --no-copy"

test_expect_success "fail after file move" '
    mv mountdir/bigfile mountdir/bigfile2
    test_must_fail ipfs cat "$HASH" >/dev/null
'

test_expect_success "must use absolute path" '
    echo "some content" > somefile &&
    test_must_fail ipfs add --no-copy somefile 
'

test_add_cat_200MB "add --no-copy" "`pwd`" "QmVbVLFLbz72tRSw3HMBh6ABKbRVavMQLoh2BzQ4dUSAYL"

test_add_cat_200MB "add --no-copy --raw-leaves" "`pwd`" "QmYJWknpk2HUjVCkTDFMcTtxEJB4XbUpFRYW4BCAEfDN6t"

test_done
