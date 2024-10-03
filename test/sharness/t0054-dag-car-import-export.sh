#!/usr/bin/env bash
#

test_description="Test car file import/export functionality"

. lib/test-lib.sh
export -f ipfsi

set -o pipefail

tar -C ../t0054-dag-car-import-export-data/ --strip-components=1 -Jxf ../t0054-dag-car-import-export-data/test_dataset_car.tar.xz
tab=$'\t'

test_cmp_sorted() {
  # use test_cmp to dump out the unsorted file contents as a diff
  [[ "$( sort "$1" | sha256sum )" == "$( sort "$2" | sha256sum )" ]] \
    || test_cmp "$1" "$2"
}
export -f test_cmp_sorted

reset_blockstore() {
  node=$1

  ipfsi "$node" pin ls --quiet --type=recursive | ipfsi "$node" pin rm &>/dev/null
  ipfsi "$node" repo gc &>/dev/null

  test_expect_success "pinlist empty" '
    [[ -z "$( ipfsi $node pin ls )" ]]
  '
  test_expect_success "nothing left to gc" '
    [[ -z "$( ipfsi $node repo gc )" ]]
  '
}

# hammer with concurrent gc to ensure nothing clashes
do_import() {
  node="$1"; shift
  (
      touch spin.gc

      while [[ -e spin.gc ]]; do ipfsi "$node" repo gc &>/dev/null; done &
      while [[ -e spin.gc ]]; do ipfsi "$node" repo gc &>/dev/null; done &

      ipfsi "$node" dag import "$@" 2>&1 && ipfsi "$node" repo verify &>/dev/null
      result=$?

      rm -f spin.gc &>/dev/null
      wait || true  # work around possible trigger of a bash bug on overloaded circleci
      exit $result
  )
}

run_online_imp_exp_tests() {

  reset_blockstore 0
  reset_blockstore 1

  cat > basic_import_stats_expected <<EOE
Imported 1198 blocks (468513 bytes)
Pinned root${tab}bafkqaaa${tab}success
Pinned root${tab}bafy2bzaceaxm23epjsmh75yvzcecsrbavlmkcxnva66bkdebdcnyw3bjrc74u${tab}success
Pinned root${tab}bafy2bzaced4ueelaegfs5fqu4tzsh6ywbbpfk3cxppupmxfdhbpbhzawfw5oy${tab}success
EOE
  # output without the --stats line at the top
  tail -n +2 basic_import_stats_expected > basic_import_expected

  # Explainer:
  # naked_root_import_json_expected output is produced by dag import of combined_naked_roots_genesis_and_128.car
  # executed when roots are already present in the repo - thus the BlockCount=0
  # (if blocks were not present in the repo, ipld: could not find <CID> would be returned)
  cat >naked_root_import_json_expected <<EOE
{"Root":{"Cid":{"/":"bafy2bzaceaxm23epjsmh75yvzcecsrbavlmkcxnva66bkdebdcnyw3bjrc74u"},"PinErrorMsg":""}}
{"Root":{"Cid":{"/":"bafy2bzaced4ueelaegfs5fqu4tzsh6ywbbpfk3cxppupmxfdhbpbhzawfw5oy"},"PinErrorMsg":""}}
{"Stats":{"BlockCount":0,"BlockBytesCount":0}}
EOE

  test_expect_success "basic import" '
    do_import 0 \
      ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
      ../t0054-dag-car-import-export-data/lotus_testnet_export_128_shuffled_nulroot.car \
      ../t0054-dag-car-import-export-data/lotus_devnet_genesis_shuffled_nulroot.car \
    > basic_import_actual
  '

  test_expect_success "basic import output as expected" '
    test_cmp_sorted basic_import_expected basic_import_actual
  '

  test_expect_success "basic import with --stats" '
    do_import 0 --stats \
      ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
      ../t0054-dag-car-import-export-data/lotus_testnet_export_128_shuffled_nulroot.car \
      ../t0054-dag-car-import-export-data/lotus_devnet_genesis_shuffled_nulroot.car \
    > basic_import_actual
  '

  test_expect_success "basic import output with --stats as expected" '
    test_cmp_sorted basic_import_stats_expected basic_import_actual
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
    [[ -z "$( ipfsi 1 pin ls )" ]]
  '

  test_expect_success "import/pin naked roots only, relying on local blockstore having all the data" '
    ipfsi 1 dag import --stats --enc=json ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
      > naked_import_result_json_actual
  '

  test_expect_success "naked import output as expected" '
    test_cmp_sorted naked_root_import_json_expected naked_import_result_json_actual
  '

  reset_blockstore 0
  reset_blockstore 1

  mkfifo pipe_testnet
  mkfifo pipe_devnet

  test_expect_success "fifo import" '
    (
        cat ../t0054-dag-car-import-export-data/lotus_testnet_export_128_shuffled_nulroot.car > pipe_testnet &
        cat ../t0054-dag-car-import-export-data/lotus_devnet_genesis_shuffled_nulroot.car > pipe_devnet &

        do_import 0 --stats \
          pipe_testnet \
          pipe_devnet \
          ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
        > basic_fifo_import_actual
        result=$?

        wait || true	# work around possible trigger of a bash bug on overloaded circleci
        exit "$result"
    )
  '

  test_expect_success "remove fifos" '
    rm pipe_testnet pipe_devnet
  '

  test_expect_success "fifo-import output as expected" '
    test_cmp_sorted basic_import_stats_expected basic_fifo_import_actual
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
test_init_ipfs --empty-repo=false


test_expect_success "basic offline export of 'getting started' dag works" '
  ipfs dag export "$HASH_WELCOME_DOCS" >/dev/null
'

test_expect_success "basic offline export of nonexistent cid" '
  ! ipfs dag export QmYwAPJXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX 2> offline_fetch_error_actual >/dev/null
'
test_expect_success "correct error" '
  test_should_contain "Error: block was not found locally (offline): ipld: could not find QmYwAPJXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX" offline_fetch_error_actual
'

cat >multiroot_import_json_stats_expected <<EOE
{"Root":{"Cid":{"/":"bafy2bzaceb55n7uxyfaelplulk3ev2xz7gnq6crncf3ahnvu46hqqmpucizcw"},"PinErrorMsg":""}}
{"Root":{"Cid":{"/":"bafy2bzacebedrc4n2ac6cqdkhs7lmj5e4xiif3gu7nmoborihajxn3fav3vdq"},"PinErrorMsg":""}}
{"Root":{"Cid":{"/":"bafy2bzacede2hsme6hparlbr4g2x6pylj43olp4uihwjq3plqdjyrdhrv7cp4"},"PinErrorMsg":""}}
{"Stats":{"BlockCount":2825,"BlockBytesCount":1339709}}
EOE
# output without --stats line
head -3 multiroot_import_json_stats_expected > multiroot_import_json_expected

test_expect_success "multiroot import works (--enc=json)" '
  ipfs dag import --enc=json ../t0054-dag-car-import-export-data/lotus_testnet_export_256_multiroot.car > multiroot_import_json_actual
'
test_expect_success "multiroot import expected output" '
  test_cmp_sorted multiroot_import_json_expected multiroot_import_json_actual
'

test_expect_success "multiroot import works with --stats" '
  ipfs dag import --stats --enc=json ../t0054-dag-car-import-export-data/lotus_testnet_export_256_multiroot.car > multiroot_import_json_actual
'
test_expect_success "multiroot import expected output" '
  test_cmp_sorted multiroot_import_json_stats_expected multiroot_import_json_actual
'


cat >pin_import_expected << EOE
{"Stats":{"BlockCount":1198,"BlockBytesCount":468513}}
EOE
test_expect_success "pin-less import works" '
  ipfs dag import --stats --enc=json --pin-roots=false \
  ../t0054-dag-car-import-export-data/lotus_devnet_genesis.car \
  ../t0054-dag-car-import-export-data/lotus_testnet_export_128.car \
    > no-pin_import_actual
'
test_expect_success "expected no pins on --pin-roots=false" '
  test_cmp pin_import_expected no-pin_import_actual
'


test_expect_success "naked root import works" '
  ipfs dag import --stats --enc=json ../t0054-dag-car-import-export-data/combined_naked_roots_genesis_and_128.car \
    > naked_root_import_json_actual
'
test_expect_success "naked root import expected output" '
   test_cmp_sorted naked_root_import_json_expected naked_root_import_json_actual
'

test_expect_success "'ipfs dag import' check block size" '
    BIG_CID=$(dd if=/dev/zero bs=2MB count=1 | ipfs dag put --input-codec=raw --store-codec=raw --allow-big-block) &&
    ipfs dag export $BIG_CID > 2-MB-block.car &&
    test_expect_code 1 ipfs dag import 2-MB-block.car >dag_import_out 2>&1
'
test_expect_success "ipfs dag import output has the correct error" '
    grep "block is over 1MiB" dag_import_out
'

test_expect_success "ipfs dag import --allow-big-block works" '
    test_expect_code 0 ipfs dag import --allow-big-block 2-MB-block.car
'

cat > version_2_import_expected << EOE
{"Root":{"Cid":{"/":"bafy2bzaceaxm23epjsmh75yvzcecsrbavlmkcxnva66bkdebdcnyw3bjrc74u"},"PinErrorMsg":""}}
{"Root":{"Cid":{"/":"bafy2bzaced4ueelaegfs5fqu4tzsh6ywbbpfk3cxppupmxfdhbpbhzawfw5oy"},"PinErrorMsg":""}}
{"Stats":{"BlockCount":1198,"BlockBytesCount":468513}}
EOE

test_expect_success "version 2 import" '
  ipfs dag import --stats --enc=json \
    ../t0054-dag-car-import-export-data/lotus_testnet_export_128_v2.car \
    ../t0054-dag-car-import-export-data/lotus_devnet_genesis_v2.car \
  > version_2_import_actual
'

test_expect_success "version 2 import output as expected" '
  test_cmp_sorted version_2_import_expected version_2_import_actual
'

test_expect_success "'ipfs dag import' decode IPLD 'dag-json' codec works" '
  NEW_HASH=$(echo "{ \"test\": \"dag-json\" }" | ipfs dag put --store-codec dag-json) &&
  ipfs dag export $NEW_HASH > dag-json.car &&
  ipfs dag import dag-json.car &&
  rm dag-json.car
'

test_expect_success "'ipfs dag import' decode IPLD 'dag-cbor' codec works" '
  NEW_HASH=$(echo "{ \"test\": \"dag-cbor\" }" | ipfs dag put --store-codec dag-cbor) &&
  ipfs dag export $NEW_HASH > dag-cbor.car &&
  ipfs dag import dag-cbor.car &&
  rm dag-cbor.car
'

test_expect_success "'ipfs dag import' decode IPLD 'json' codec works" '
  NEW_HASH=$(echo "{ \"test\": \"json\" }" | ipfs dag put --store-codec json) &&
  ipfs dag export $NEW_HASH > json.car &&
  ipfs dag import json.car &&
  rm json.car
'

test_expect_success "'ipfs dag import' decode IPLD 'cbor' codec works" '
  NEW_HASH=$(echo "{ \"test\": \"cbor\" }" | ipfs dag put --store-codec cbor) &&
  ipfs dag export $NEW_HASH > cbor.car &&
  ipfs dag import cbor.car &&
  rm cbor.car
'

# IPIP-402
cat > partial_nopin_import_expected << EOE
{"Stats":{"BlockCount":1,"BlockBytesCount":1618}}
EOE
test_expect_success "'ipfs dag import' without pinning works fine with incomplete DAG (unixfs dir exported as dag-scope=entity from IPIP-402)" '
    ipfs dag import --stats --enc=json --pin-roots=false ../t0054-dag-car-import-export-data/partial-dag-scope-entity.car >partial_nopin_import_out 2>&1 &&
    test_cmp partial_nopin_import_expected partial_nopin_import_out
'

test_expect_success "'ipfs dag import' with pinning errors due to incomplete DAG (unixfs dir exported as dag-scope=entity from IPIP-402)" '
    ipfs dag import --stats --enc=json --pin-roots=true ../t0054-dag-car-import-export-data/partial-dag-scope-entity.car >partial_pin_import_out 2>&1 &&
    test_should_contain "\"PinErrorMsg\":\"block was not found locally" partial_pin_import_out
'

test_expect_success "'ipfs dag import' pin error in default CLI mode produces exit code 1 (unixfs dir exported as dag-scope=entity from IPIP-402)" '
    test_expect_code 1 ipfs dag import ../t0054-dag-car-import-export-data/partial-dag-scope-entity.car >partial_pin_import_out 2>&1 &&
    test_should_contain "Error: pinning root \"QmPDC11yLAbVw3dX5jMeEuSdk4BiVjSd9X87zaYRdVjzW3\" FAILED: block was not found locally" partial_pin_import_out
'

test_done
