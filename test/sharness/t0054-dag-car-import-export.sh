#!/usr/bin/env bash
#

test_description="Test car file import/export functionality"

. lib/test-lib.sh
export -f ipfsi

reset_blockstore() {
  node=$1
  ipfsi $1 pin ls --quiet --type=recursive | ipfsi $1 pin rm &>/dev/null
  ipfsi $1 repo gc &>/dev/null

  test_expect_success "pinlist empty" '
    [ -z "$( ipfsi $1 pin ls )" ]
  '
  test_expect_success "nothing left to gc" '
    [ -z "$( ipfsi $1 repo gc )" ]
  '
}

# hammer with concurrent gc to ensure nothing clashes
do_import() {
  node=$1; shift

  bash -c "while [ -e spin.gc ]; do ipfsi $node repo gc >>gc_out 2>&1; done" & gc1_pid=$!
  bash -c "while [ -e spin.gc ]; do ipfsi $node repo gc >>gc_out 2>&1; done" & gc2_pid=$!

  ipfsi $node dag import --progress=false "$@"

  rm spin.gc
  wait $gc1_pid
  wait $gc2_pid
}

run_imp_exp_tests() {

  reset_blockstore 0
  reset_blockstore 1

  echo -e "Pinned root\tbafkqaaa\tsuccess (root specified in .car header without its data)" > basic_import_expected
  echo -e "Pinned root\tbafy2bzaceaxm23epjsmh75yvzcecsrbavlmkcxnva66bkdebdcnyw3bjrc74u\tsuccess" >> basic_import_expected
  echo -e "Pinned root\tbafy2bzaced4ueelaegfs5fqu4tzsh6ywbbpfk3cxppupmxfdhbpbhzawfw5oy\tsuccess" >> basic_import_expected

  echo -e "Pinned root\tbafy2bzaceaxm23epjsmh75yvzcecsrbavlmkcxnva66bkdebdcnyw3bjrc74u\tsuccess (root specified in .car header without its data)" > naked_import_result_expected
  echo -e "Pinned root\tbafy2bzaced4ueelaegfs5fqu4tzsh6ywbbpfk3cxppupmxfdhbpbhzawfw5oy\tsuccess (root specified in .car header without its data)" >> naked_import_result_expected

  touch spin.gc
  test_expect_success "basic import" '
    do_import 0 \
      ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
      ../t0054-dag-car-import-export-data/lotus_testnet_export_128_shuffled_nulroot.car \
      ../t0054-dag-car-import-export-data/lotus_devnet_genesis_shuffled_nulroot.car \
    | sort > basic_import_actual
  '

  # FIXME - the fact we reliably fail this is indicative of some sort of race...
  test_expect_failure "concurrent GC did not manage to grab anything" '
    ! [ -s gc_out ]
  '
  test_expect_success "basic import output as expected" '
    test_cmp basic_import_expected basic_import_actual
  '

  test_expect_success "basic fetch+export 1" '
    ipfsi 1 dag export bafy2bzaced4ueelaegfs5fqu4tzsh6ywbbpfk3cxppupmxfdhbpbhzawfw5oy > reexported_testnet_128.car
  '
  test_expect_success "export of shuffled testnet export identical to canonical original" '
    test_cmp reexported_testnet_128.car ../t0054-dag-car-import-export-data/lotus_testnet_export_128.car
  '

  test_expect_success "basic fetch+export 2" '
    ipfsi 1 dag export bafy2bzaceaxm23epjsmh75yvzcecsrbavlmkcxnva66bkdebdcnyw3bjrc74u > reexported_devnet_genesis.car
  '
  test_expect_success "export of shuffled devnet export identical to canonical original" '
    test_cmp reexported_devnet_genesis.car ../t0054-dag-car-import-export-data/lotus_devnet_genesis.car
  '

  test_expect_success "pinlist on node1 still empty" '
    [ -z "$( ipfsi 1 pin ls )" ]
  '

  test_expect_success "import/pin naked roots only, relying on local blockstore having all the data" '
    ipfsi 1 dag import --progress=false  ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
      | sort > naked_import_result_actual
  '

  test_expect_success "naked import output as expected" '
    test_cmp  naked_import_result_expected naked_import_result_actual
  '

  reset_blockstore 0
  reset_blockstore 1

  mkfifo pipe_testnet
  mkfifo pipe_devnet

  # test that ipfs correctly opens both pipes and deleting them doesn't interfere with cleanup
  bash -c '
    sleep 1
    cat ../t0054-dag-car-import-export-data/lotus_testnet_export_128_shuffled_nulroot.car > pipe_testnet &
    cat ../t0054-dag-car-import-export-data/lotus_devnet_genesis_shuffled_nulroot.car > pipe_devnet &
    rm pipe_testnet pipe_devnet
  ' &

  touch spin.gc
  test_expect_success "fifo import" '
    do_import 0 \
      pipe_testnet \
      pipe_devnet \
      ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
    | sort > basic_fifo_import_actual
  '
  # FIXME - the fact we reliably fail this is indicative of some sort of race...
  test_expect_failure "concurrent GC did not manage to grab anything" '
    ! [ -s gc_out ]
  '

  test_expect_success "fifo-import output as expected" '
    test_cmp basic_import_expected basic_fifo_import_actual
  '

  test_expect_success "fifos no longer present" '
    ! [ -e pipe_testnet ] && ! [ -e pipe_devnet ]
  '
}


test_expect_success "set up testbed" '
   iptb testbed create -type localipfs -count 2 -force -init
'
startup_cluster 2

run_imp_exp_tests

test_expect_success "shut down nodes" '
  iptb stop && iptb_wait_stop
'

test_expect_success "multiroot import works" '
  ipfsi 0 dag import ../t0054-dag-car-import-export-data/lotus_testnet_export_256_multiroot.car
'



test_done
