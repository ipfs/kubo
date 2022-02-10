#!/usr/bin/env bash

test_description="Test the gRPC datastore plugin"

. lib/test-lib.sh

test_init_ipfs

SPEC=$(cat ../t0280-plugin-grpcds-data/spec-grpcds)

test_expect_success "change spec config" '
  ipfs config --json Datastore.Spec "$SPEC"
'

test_expect_success "run grpc datastore server" '
  grpc-ds-server &
  echo $! > server_pid
  sleep 0.5
'

test_expect_success "backup config and re-init to properly initalize datastore" '
  cp .ipfs/config config-backup
  rm -rf .ipfs
  ipfs init config-backup
'

test_expect_success "can not launch daemon when server is not running" '
  kill $(cat server_pid) || true
  test_must_fail ipfs daemon
'

test_expect_success "run grpc datastore server" '
  grpc-ds-server &
  echo $! > server_pid
  sleep 0.5
'

test_launch_ipfs_daemon
test_kill_ipfs_daemon

test_expect_success "kill the datastore server" '
  kill $(cat server_pid) || true
'

test_done
