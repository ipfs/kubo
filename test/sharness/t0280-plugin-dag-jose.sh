#!/usr/bin/env bash
#
# Copyright (c) 2021 Mohsin Zaidi
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test dag-jose plugin"

. lib/test-lib.sh

test_init_ipfs

test_dag_jose() {
  test_expect_success "encode as dag-jose, decode back to original, verify round-trip" $'
    find ../t0280-plugin-dag-jose-data -type f | xargs -I {} sh -c \' \
      codec=$(basename $(dirname {})); \
      joseHash=$(ipfs dag put --store-codec dag-jose --input-codec=$codec {}); \
      ipfs dag get --output-codec $codec $joseHash > $(basename {}); \
      diff {} $(basename {}) \'
  '

  test_expect_success "retrieve dag-jose in non-dag-jose encodings" $'
      find ../t0280-plugin-dag-jose-data -type f | xargs -I {} sh -c \' \
        codec=$(basename $(dirname {})); \
        joseHash=$(ipfs dag put --store-codec dag-jose --input-codec=$codec {}); \
        ipfs dag get --output-codec dag-cbor $joseHash > /dev/null; \
        ipfs dag get --output-codec dag-json $joseHash > /dev/null \'
    '
}

# should work offline
test_dag_jose

# should work online
test_launch_ipfs_daemon
test_dag_jose
test_kill_ipfs_daemon

test_done
