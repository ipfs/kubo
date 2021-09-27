#!/usr/bin/env bash
#
# Copyright (c) 2017 Jakub Sztandera
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test git plugin"

. lib/test-lib.sh

test_init_ipfs

# from https://github.com/ipfs/go-ipld-git/blob/master/make-test-repo.sh
test_expect_success "prepare test data" '
  tar xzf ../t0280-plugin-git-data/git.tar.gz
'

test_dag_git() {
  test_expect_success "add objects via dag put" '
    find objects -type f -exec ipfs dag put --store-codec=git-raw --input-codec=0x300078 --hash=sha1 {} \; -exec echo -n \; > hashes
  '

  test_expect_success "successfully get added objects" '
    cat hashes | xargs -I {} ipfs dag get -- {} > /dev/null
  '

  test_expect_success "dag get works" '
    echo -n "{\"message\":\"Some version\n\",\"object\":{\"/\":\"baf4bcfeq6c2mspupcvftgevza56h7rmozose6wi\"},\"tag\":\"v1\",\"tagger\":{\"date\":\"1497302532\",\"email\":\"johndoe@example.com\",\"name\":\"John Doe\",\"timezone\":\"+0200\"},\"type\":\"commit\"}" > tag_expected &&
    ipfs dag get baf4bcfhzi72pcj5cc4ocz7igcduubuu7aa3cddi > tag_actual
  '

  test_expect_success "outputs look correct" '
    test_cmp tag_expected tag_actual
  '

  test_expect_success "path traversals work" '
    echo -n "{\"date\":\"1497302532\",\"email\":\"johndoe@example.com\",\"name\":\"John Doe\",\"timezone\":\"+0200\"}" > author_expected &&
    echo -n "{\"/\":{\"bytes\":\"YmxvYiAxMgBIZWxsbyB3b3JsZAo\"}}" > file1_expected &&
    echo -n "{\"/\":{\"bytes\":\"YmxvYiA3ACcsLnB5Zgo\"}}" > file2_expected &&
    ipfs dag get baf4bcfhzi72pcj5cc4ocz7igcduubuu7aa3cddi/object/author > author_actual &&
    ipfs dag get baf4bcfhzi72pcj5cc4ocz7igcduubuu7aa3cddi/object/tree/file/hash > file1_actual &&
    ipfs dag get baf4bcfhzi72pcj5cc4ocz7igcduubuu7aa3cddi/object/parents/0/tree/dir2/hash/f3/hash > file2_actual
  '

  test_expect_success "outputs look correct" '
    test_cmp author_expected author_actual &&
    test_cmp file1_expected file1_actual &&
    test_cmp file2_expected file2_actual
  '
}

# should work offline
test_dag_git

# should work online
test_launch_ipfs_daemon
test_dag_git
test_kill_ipfs_daemon

test_done
