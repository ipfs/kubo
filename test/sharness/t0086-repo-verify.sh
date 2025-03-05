#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo fsck"

. lib/test-lib.sh

test_init_ipfs

sort_rand() {
  case `uname` in
    Linux|FreeBSD)
      sort -R
      ;;
    Darwin)
      ruby -e 'puts STDIN.readlines.shuffle'
      ;;
    *)
      echo "unsupported system: $(uname)"
  esac
}

check_random_corruption() {
  to_break=$(find "$IPFS_PATH/blocks" -type f -name '*.data' | sort_rand | head -n 1)

  test_expect_success "repo verify detects a failure" '
    mv "$to_break" backup_file &&
    echo -n "this block will not match expected hash" > "$to_break" &&
    test_expect_code 1 ipfs repo verify
  '

  test_expect_success "repo verify passes once a failure is fixed" '
    mv backup_file "$to_break" &&
    ipfs repo verify
  '
}

test_expect_success "create some files" '
  random-files -depth=3 -dirs=4 -files=10 foobar > /dev/null
'

test_expect_success "add them all" '
  ipfs add -r -q foobar > /dev/null
'

for i in `seq 20`
do
  check_random_corruption
done

test_done
