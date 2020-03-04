#!/usr/bin/env bash
#

test_description="Test car file import/export functionality"

. lib/test-lib.sh

test_init_ipfs


echo "Error: merkledag: not found (currently offline, perhaps retry after attaching to the network)" > offline_fetch_error_expected

test_expect_success "basic offline export of nonexistent cid" '
  ! ipfs dag export QmYwAPJXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX 2> offline_fetch_error_actual
'

test_expect_success "correct error" '
  test_cmp offline_fetch_error_expected offline_fetch_error_actual
'

test_expect_success "basic offline export of 'getting started' dag" '
  ipfs dag export QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv >/dev/null
'

test_done
