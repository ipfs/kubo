#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo fsck"

. lib/test-lib.sh

test_init_ipfs

ROOT_HASH=QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn

test_expect_success "ipfs repo rm-root fails without --confirm" '
  test_must_fail ipfs repo rm-root 2> err &&
  cat err &&
  fgrep -q "please pass --confirm to proceed" err
'

test_expect_success "ipfs repo rm-root fails to remove existing root without --remove-local-root" '
  test_must_fail ipfs repo rm-root --confirm 2> err &&
  cat err &&
  fgrep -q "Are you sure you want to unlink this?" err
'

test_expect_success "ipfs repo rm-root" '
  ipfs repo rm-root --confirm --remove-local-root | tee rm-root.actual &&
  echo "Unlinked files API root.  Root hash was $ROOT_HASH." > rm-root.expected &&
  test_cmp rm-root.expected rm-root.actual
'

test_expect_success "files api root really removed" '
  ipfs repo rm-root --confirm | tee rm-root-post.actual &&
  echo "Files API root not found." > rm-root-post.expected &&
  test_cmp rm-root-post.expected rm-root-post.actual
'

test_launch_ipfs_daemon

test_expect_success "ipfs repo rm-root does not run on daemon" '
  test_must_fail ipfs repo rm-root --confirm 2> err &&
  cat err &&
  fgrep -q "ipfs daemon is running" err
'

test_kill_ipfs_daemon

test_done
