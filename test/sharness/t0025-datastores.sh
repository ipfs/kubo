#!/usr/bin/env bash

test_description="Test non-standard datastores"

. lib/test-lib.sh

profiles=("flatfs" "pebbleds" "badgerds")
proot="$(mktemp -d "${TMPDIR:-/tmp}/t0025.XXXXXX")"

for profile in "${profiles[@]}"; do
    test_expect_success "'ipfs init --empty-repo=false --profile=$profile' succeeds" '
        BITS="2048" &&
        IPFS_PATH="$proot/$profile" &&
        ipfs init --empty-repo=false --profile=$profile
    '
    test_expect_success "'ipfs pin add' and 'pin ls' works with $profile" '
        export IPFS_PATH="$proot/$profile" &&
        echo -n "hello_$profile" | ipfs block put --pin=true > hello_cid &&
        ipfs pin ls -t recursive "$(cat hello_cid)"
    '
done

test_done
