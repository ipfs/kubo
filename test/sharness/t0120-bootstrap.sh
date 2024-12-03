#!/usr/bin/env bash
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

# changing the bootstrap peers will require changing it in two places :)
BP1="/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"
BP2="/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa"
BP3="/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb"
BP4="/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt"
BP5="/dnsaddr/va1.bootstrap.libp2p.io/p2p/12D3KooWKnDdG3iXw9eTFijk3EWSunZcFi54Zka4wmtqtt6rPxc8"
BP6="/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
BP7="/ip4/104.131.131.82/udp/4001/quic-v1/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"

test_description="Test ipfs repo operations"

. lib/test-lib.sh

test_init_ipfs

# we use a function so that we can run it both offline + online
test_bootstrap_list_cmd() {
  printf "" >list_expected

  for BP in "$@"
  do
    echo "$BP" >>list_expected
  done

  test_expect_success "'ipfs bootstrap' succeeds" '
    ipfs bootstrap >list_actual
  '

  test_expect_success "'ipfs bootstrap' output looks good" '
    test_cmp list_expected list_actual
  '

  test_expect_success "'ipfs bootstrap list' succeeds" '
    ipfs bootstrap list >list2_actual
  '

  test_expect_success "'ipfs bootstrap list' output looks good" '
    test_cmp list_expected list2_actual
  '
}

# we use a function so that we can run it both offline + online
test_bootstrap_cmd() {

  # remove all peers just in case.
  # if this fails, the first listing may not be empty
  ipfs bootstrap rm --all

  test_bootstrap_list_cmd

  test_expect_success "'ipfs bootstrap add' succeeds" '
    ipfs bootstrap add "$BP1" "$BP2" "$BP3" >add_actual
  '

  test_expect_success "'ipfs bootstrap add' output looks good" '
    echo "added $BP1" >add_expected &&
    echo "added $BP2" >>add_expected &&
    echo "added $BP3" >>add_expected &&
    test_cmp add_expected add_actual
  '

  test_bootstrap_list_cmd $BP1 $BP2 $BP3

  test_expect_success "'ipfs bootstrap rm' succeeds" '
    ipfs bootstrap rm "$BP1" "$BP3" >rm_actual
  '

  test_expect_success "'ipfs bootstrap rm' output looks good" '
    echo "removed $BP1" >rm_expected &&
    echo "removed $BP3" >>rm_expected &&
    test_cmp rm_expected rm_actual
  '

  test_expect_success "'ipfs bootstrap rm' fails on bad peers" '
    test_expect_code 1 ipfs bootstrap rm "foo/bar"
  '

  test_bootstrap_list_cmd $BP2

  test_expect_success "'ipfs bootstrap add --default' succeeds" '
    ipfs bootstrap add --default >add2_actual
  '

  test_expect_success "'ipfs bootstrap add --default' output has default BP" '
    echo "added $BP1" >add2_expected &&
    echo "added $BP2" >>add2_expected &&
    echo "added $BP3" >>add2_expected &&
    echo "added $BP4" >>add2_expected &&
    echo "added $BP5" >>add2_expected &&
    echo "added $BP6" >>add2_expected &&
    echo "added $BP7" >>add2_expected &&
    test_cmp add2_expected add2_actual
  '

  test_bootstrap_list_cmd $BP1 $BP2 $BP3 $BP4 $BP5 $BP6 $BP7

  test_expect_success "'ipfs bootstrap rm --all' succeeds" '
    ipfs bootstrap rm --all >rm2_actual
  '

  test_expect_success "'ipfs bootstrap rm' output looks good" '
    echo "removed $BP1" >rm2_expected &&
    echo "removed $BP2" >>rm2_expected &&
    echo "removed $BP3" >>rm2_expected &&
    echo "removed $BP4" >>rm2_expected &&
    echo "removed $BP5" >>rm2_expected &&
    echo "removed $BP6" >>rm2_expected &&
    echo "removed $BP7" >>rm2_expected &&
    test_cmp rm2_expected rm2_actual
  '

  test_bootstrap_list_cmd

  test_expect_success "'ipfs bootstrap add' accepts args from stdin" '
  echo $BP1 > bpeers &&
  echo $BP2 >> bpeers &&
  echo $BP3 >> bpeers &&
  echo $BP4 >> bpeers &&
  cat bpeers | ipfs bootstrap add > add_stdin_actual
  '

  test_expect_success "output looks good" '
  echo "added $BP1" > bpeers_add_exp &&
  echo "added $BP2" >> bpeers_add_exp &&
  echo "added $BP3" >> bpeers_add_exp &&
  echo "added $BP4" >> bpeers_add_exp &&
  test_cmp add_stdin_actual bpeers_add_exp
  '

  test_bootstrap_list_cmd $BP1 $BP2 $BP3 $BP4

  test_expect_success "'ipfs bootstrap rm' accepts args from stdin" '
  cat bpeers | ipfs bootstrap rm > rm_stdin_actual
  '

  test_expect_success "output looks good" '
  echo "removed $BP1" > bpeers_rm_exp &&
  echo "removed $BP2" >> bpeers_rm_exp &&
  echo "removed $BP3" >> bpeers_rm_exp &&
  echo "removed $BP4" >> bpeers_rm_exp &&
  test_cmp rm_stdin_actual bpeers_rm_exp
  '

  test_bootstrap_list_cmd
}

# should work offline
test_bootstrap_cmd

# should work online
test_launch_ipfs_daemon
test_bootstrap_cmd
test_kill_ipfs_daemon


test_done
