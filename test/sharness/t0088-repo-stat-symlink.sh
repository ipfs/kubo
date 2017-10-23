#!/bin/sh
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

# compare RepoSize when getting it directly vs via symbolic link
test_expect_success "'ipfs repo stat' RepoSize is correct with sym link" '
  export IPFS_PATH="sym_link_target" &&
  reposize_direct=$(ipfs repo stat | grep RepoSize | awk '\''{ print $2 }'\'') &&
  export IPFS_PATH=".ipfs" &&
  reposize_symlink=$(ipfs repo stat | grep RepoSize | awk '\''{ print $2 }'\'') &&
  test $reposize_symlink -ge $reposize_direct
'

test_done
