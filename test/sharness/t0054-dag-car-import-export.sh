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

  ipfsi $node dag import "$@"

  rm spin.gc
  wait $gc1_pid
  wait $gc2_pid
}

run_online_imp_exp_tests() {

  reset_blockstore 0
  reset_blockstore 1

  echo -e "Pinned root\tbafkqaaa\tsuccess (root specified in .car header without its data)" > basic_import_expected
  echo -e "Pinned root\tbafy2bzaceaxm23epjsmh75yvzcecsrbavlmkcxnva66bkdebdcnyw3bjrc74u\tsuccess" >> basic_import_expected
  echo -e "Pinned root\tbafy2bzaced4ueelaegfs5fqu4tzsh6ywbbpfk3cxppupmxfdhbpbhzawfw5oy\tsuccess" >> basic_import_expected

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

run_online_imp_exp_tests

test_expect_success "shut down nodes" '
  iptb stop && iptb_wait_stop
'


# We want to just init the repo, without using a daemon for stuff below
# The ulimit is only adjusted when a server starts
# Do it here instead of mucking with lib/test-lib.sh
ulimit -n 2048
test_init_ipfs


test_expect_success "basic offline export of 'getting started' dag works" '
  ipfs dag export QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv >/dev/null
'


echo "Error: merkledag: not found (currently offline, perhaps retry after attaching to the network)" > offline_fetch_error_expected
test_expect_success "basic offline export of nonexistent cid" '
  ! ipfs dag export QmYwAPJXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX 2> offline_fetch_error_actual
'
test_expect_success "correct error" '
  test_cmp offline_fetch_error_expected offline_fetch_error_actual
'


cat >multiroot_import_expected <<EOE
{"Root":{"Cid":{"/":"bafy2bzaceb55n7uxyfaelplulk3ev2xz7gnq6crncf3ahnvu46hqqmpucizcw"},"PresentInCar":true,"PinErrorMsg":""}}
{"Root":{"Cid":{"/":"bafy2bzacebedrc4n2ac6cqdkhs7lmj5e4xiif3gu7nmoborihajxn3fav3vdq"},"PresentInCar":true,"PinErrorMsg":""}}
{"Root":{"Cid":{"/":"bafy2bzacede2hsme6hparlbr4g2x6pylj43olp4uihwjq3plqdjyrdhrv7cp4"},"PresentInCar":true,"PinErrorMsg":""}}
EOE
test_expect_success "multiroot import works" '
  ipfs dag import --enc=json ../t0054-dag-car-import-export-data/lotus_testnet_export_256_multiroot.car | sort > multiroot_import_actual
'
test_expect_success "multiroot import expected output" '
   test_cmp multiroot_import_expected multiroot_import_actual
'


test_expect_success "pin-less import works" '
  ipfs dag import --enc=json --pin-roots=false \
  ../t0054-dag-car-import-export-data/lotus_devnet_genesis.car \
  ../t0054-dag-car-import-export-data/lotus_testnet_export_128.car \
    > no-pin_import_actual
'
test_expect_success "expected silence on --pin-roots=false" '
  ! [ -s no-pin_import_actual ]
'


cat >naked_root_import_expected <<EOE
{"Root":{"Cid":{"/":"bafy2bzaceaxm23epjsmh75yvzcecsrbavlmkcxnva66bkdebdcnyw3bjrc74u"},"PresentInCar":false,"PinErrorMsg":""}}
{"Root":{"Cid":{"/":"bafy2bzaced4ueelaegfs5fqu4tzsh6ywbbpfk3cxppupmxfdhbpbhzawfw5oy"},"PresentInCar":false,"PinErrorMsg":""}}
EOE
test_expect_success "naked root import works" '
  ipfs dag import --enc=json ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
  | sort > naked_root_import_actual
'
test_expect_success "naked root import expected output" '
   test_cmp naked_root_import_expected naked_root_import_actual
'


test_done
