#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test migrations auto update prompt"

. lib/test-lib.sh

test_init_ipfs

# Remove explicit AutoConf.Enabled=false from test profile to use implicit default
# This allows daemon to work with 'auto' values added by v16-to-17 migration
ipfs config --json AutoConf.Enabled null >/dev/null 2>&1

MIGRATION_START=7
IPFS_REPO_VER=$(<.ipfs/version)

# Generate mock migration binaries
gen_mock_migrations() {
  mkdir bin
  i=$((MIGRATION_START))
  until [ $i -ge $IPFS_REPO_VER ]
  do
    j=$((i+1))
    echo "#!/bin/bash" > bin/fs-repo-${i}-to-${j}
    echo "echo fake applying ${i}-to-${j} repo migration" >> bin/fs-repo-${i}-to-${j}
    # Update version file to the target version for hybrid migration system
    echo "if [ \"\$1\" = \"-path\" ] && [ -n \"\$2\" ]; then" >> bin/fs-repo-${i}-to-${j}
    echo "  echo $j > \"\$2/version\"" >> bin/fs-repo-${i}-to-${j}
    echo "elif [ -n \"\$IPFS_PATH\" ]; then" >> bin/fs-repo-${i}-to-${j}
    echo "  echo $j > \"\$IPFS_PATH/version\"" >> bin/fs-repo-${i}-to-${j}
    echo "fi" >> bin/fs-repo-${i}-to-${j}
    chmod +x bin/fs-repo-${i}-to-${j}
    ((i++))
  done
}

# Check for expected output from each migration
check_migration_output() {
  out_file="$1"
  i=$((MIGRATION_START))
  until [ $i -ge $IPFS_REPO_VER ]
  do
    j=$((i+1))
    grep "applying ${i}-to-${j} repo migration" "$out_file" > /dev/null
    ((i++))
  done
}

# Create fake migration binaries instead of letting ipfs download from network
# To test downloading and running actual binaries, comment out this test.
test_expect_success "setup mock migrations" '
  gen_mock_migrations &&
  find bin -name "fs-repo-*-to-*" | wc -l > mock_count &&
  echo $((IPFS_REPO_VER-MIGRATION_START)) > expect_mock_count &&
  export PATH="$(pwd)/bin":$PATH &&
  test_cmp mock_count expect_mock_count
'

test_expect_success "manually reset repo version to $MIGRATION_START" '
  echo "$MIGRATION_START" > "$IPFS_PATH"/version
'

test_expect_success "ipfs daemon --migrate=false fails" '
  test_expect_code 1 ipfs daemon --migrate=false > false_out 2>&1
'

test_expect_success "output looks good" '
  grep "Kubo repository at .* has version .* and needs to be migrated to version" false_out &&
  grep "Error: fs-repo requires migration" false_out
'

# The migrations will succeed and the daemon will continue running
# since the mock migrations now properly update the repo version number.
test_expect_success "ipfs daemon --migrate=true runs migration" '
  ipfs daemon --migrate=true > true_out 2>&1 &
  DAEMON_PID=$!
  # Wait for daemon to be ready then shutdown gracefully
  sleep 3 && ipfs shutdown 2>/dev/null || kill $DAEMON_PID 2>/dev/null || true
  wait $DAEMON_PID 2>/dev/null || true
'

test_expect_success "output looks good" '
  check_migration_output true_out &&
  (grep "Success: fs-repo migrated to version $IPFS_REPO_VER" true_out > /dev/null ||
   grep "Hybrid migration completed successfully: v$MIGRATION_START → v$IPFS_REPO_VER" true_out > /dev/null)
'

test_expect_success "reset repo version for auto-migration test" '
  echo "$MIGRATION_START" > "$IPFS_PATH"/version
'

test_expect_success "'ipfs daemon' prompts to auto migrate" '
  test_expect_code 1 ipfs daemon > daemon_out 2>&1
'

test_expect_success "output looks good" '
  grep "Kubo repository at .* has version .* and needs to be migrated to version" daemon_out > /dev/null &&
  grep "Run migrations now?" daemon_out > /dev/null &&
  grep "Error: fs-repo requires migration" daemon_out > /dev/null
'

test_expect_success "ipfs repo migrate succeed" '
  test_expect_code 0 ipfs repo migrate > migrate_out
'

test_expect_success "output looks good" '
  grep "Migrating repository from version" migrate_out > /dev/null &&
  (grep "Success: fs-repo migrated to version $IPFS_REPO_VER" migrate_out > /dev/null ||
   grep "Hybrid migration completed successfully: v$MIGRATION_START → v$IPFS_REPO_VER" migrate_out > /dev/null)
'

test_expect_success "manually reset repo version to latest" '
  echo "$IPFS_REPO_VER" > "$IPFS_PATH"/version
'

test_expect_success "detect repo does not need migration" '
  test_expect_code 0 ipfs repo migrate > migrate_out
'

test_expect_success "output looks good" '
  grep "Repository is already at version" migrate_out > /dev/null
'

# ensure that we get a lock error if we need to migrate and the daemon is running
test_launch_ipfs_daemon

test_expect_success "manually reset repo version to $MIGRATION_START" '
  echo "$MIGRATION_START" > "$IPFS_PATH"/version
'

test_expect_success "ipfs repo migrate fails" '
  test_expect_code 1 ipfs repo migrate 2> migrate_out
'

test_expect_success "output looks good" '
  grep "repo.lock" migrate_out > /dev/null
'

test_kill_ipfs_daemon

test_done
