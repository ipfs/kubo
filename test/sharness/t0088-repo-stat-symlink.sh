#!/usr/bin/env bash
#
# Copyright (c) 2017 John Reed
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test 'ipfs repo stat' where IPFS_PATH is a symbolic link"

. lib/test-lib.sh

test_expect_success "create symbolic link for IPFS_PATH" '
  mkdir sym_link_target &&
  ln -s sym_link_target .ipfs
'

test_init_ipfs

# ensure that the RepoSize is reasonable when checked via a symlink.
test_expect_success "'ipfs repo stat' RepoSize is correct with sym link" '
  reposize_symlink=$(ipfs repo stat | grep RepoSize | awk '\''{ print $2 }'\'') &&
  symlink_size=$(file_size .ipfs) &&
  test "${reposize_symlink}" -gt "${symlink_size}"
'

test_done
