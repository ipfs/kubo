#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test migrations auto update prompt"

. lib/test-lib.sh

test_init_ipfs

# Create fake migration binaries instead of letting ipfs download from network
# To test downloading and running actual binaries, comment out this test.
test_expect_success "setup mock migrations" '
  mkdir bin &&
  echo "#!/bin/bash" > bin/fs-repo-7-to-8 &&
  echo "echo fake applying 7-to-8 repo migration" >> bin/fs-repo-7-to-8 &&
  chmod +x bin/fs-repo-7-to-8 &&
  echo "#!/bin/bash" > bin/fs-repo-8-to-9 &&
  echo "echo fake applying 8-to-9 repo migration" >> bin/fs-repo-8-to-9 &&
  chmod +x bin/fs-repo-8-to-9 &&
  echo "#!/bin/bash" > bin/fs-repo-9-to-10 &&
  echo "echo fake applying 9-to-10 repo migration" >> bin/fs-repo-9-to-10 &&
  chmod +x bin/fs-repo-9-to-10 &&
  echo "#!/bin/bash" > bin/fs-repo-10-to-11 &&
  echo "echo fake applying 10-to-11 repo migration" >> bin/fs-repo-10-to-11 &&
  chmod +x bin/fs-repo-10-to-11 &&
  export PATH="$(pwd)/bin":$PATH
'

test_expect_success "manually reset repo version to 7" '
  echo "7" > "$IPFS_PATH"/version
'

test_expect_success "ipfs daemon --migrate=false fails" '
  test_expect_code 1 ipfs daemon --migrate=false > false_out
'

test_expect_success "output looks good" '
  grep "Please get fs-repo-migrations from https://dist.ipfs.io" false_out
'

# The migrations will succeed, but the daemon will still exit with 1 because
# the fake migrations do not update the repo version number.
#
# If run with real migrations, the daemon continues running and must be killed.
test_expect_success "ipfs daemon --migrate=true runs migration" '
  test_expect_code 1 ipfs daemon --migrate=true > true_out 2>&1
'

test_expect_success "output looks good" '
  grep "applying 7-to-8 repo migration" true_out > /dev/null &&
  grep "applying 8-to-9 repo migration" true_out > /dev/null &&
  grep "applying 9-to-10 repo migration" true_out > /dev/null &&
  grep "applying 10-to-11 repo migration" true_out > /dev/null &&
  grep "Success: fs-repo migrated to version 11" true_out > /dev/null
'

test_expect_success "'ipfs daemon' prompts to auto migrate" '
  test_expect_code 1 ipfs daemon > daemon_out 2> daemon_err
'

test_expect_success "output looks good" '
  grep "Found outdated fs-repo" daemon_out > /dev/null &&
  grep "Run migrations now?" daemon_out > /dev/null &&
  grep "Please get fs-repo-migrations from https://dist.ipfs.io" daemon_out > /dev/null
'

test_done
