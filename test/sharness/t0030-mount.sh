#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test mount command"

. lib/test-lib.sh

# if in travis CI, don't test mount (no fuse)
if ! test_have_prereq FUSE; then
  skip_all='skipping mount tests, fuse not available'

  test_done
fi


# echo -n "ipfs" > expected && ipfs add --cid-version 1 -Q -w expected
export IPFS_NS_MAP="welcome.example.com:/ipfs/bafybeicq7bvn5lz42qlmghaoiwrve74pzi53auqetbantp5kajucsabike"

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -count $NUM_NODES -init
'
startup_cluster $NUM_NODES

# test mount failure before mounting properly.
test_expect_success "'ipfs mount' fails when there is no mount dir" '
  tmp_ipfs_mount() { ipfsi 0 mount -f=not_ipfs -n=not_ipns -m=not_mfs >output 2>output.err; } &&
  test_must_fail tmp_ipfs_mount
'

test_expect_success "'ipfs mount' output looks good" '
  test_must_be_empty output &&
  test_should_contain "not_ipns\|not_ipfs\|not_mfs" output.err
'

test_expect_success "setup and publish default IPNS value" '
  mkdir "$(pwd)/ipfs" "$(pwd)/ipns" "$(pwd)/mfs" &&
  ipfsi 0 name publish QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn
'

# make sure stuff is unmounted first
# then mount properly
test_expect_success FUSE "'ipfs mount' succeeds" '
  do_umount "$(pwd)/ipfs" || true &&
  do_umount "$(pwd)/ipns" || true &&
  do_umount "$(pwd)/mfs" || true &&
  ipfsi 0 mount -f "$(pwd)/ipfs" -n "$(pwd)/ipns" -m "$(pwd)/mfs" >actual
'

test_expect_success FUSE "'ipfs mount' output looks good" '
  echo "IPFS mounted at: $(pwd)/ipfs" >expected &&
  echo "IPNS mounted at: $(pwd)/ipns" >>expected &&
  echo "MFS mounted at: $(pwd)/mfs" >>expected &&
  test_cmp expected actual
'

test_expect_success FUSE "local symlink works" '
  ipfsi 0 id -f"<id>\n" > expected &&
  basename $(readlink ipns/local) > actual &&
  test_cmp expected actual
'

test_expect_success FUSE "can resolve ipns names" '
  echo -n "ipfs" > expected &&
  ipfsi 0 add --cid-version 1 -Q -w expected &&
  cat ipns/welcome.example.com/expected > actual &&
  test_cmp expected actual
'

test_expect_success FUSE "create mfs file via fuse" '
  touch mfs/testfile &&
  ipfsi 0 files ls | grep testfile
'

test_expect_success FUSE "create mfs dir via fuse" '
  mkdir mfs/testdir &&
  ipfsi 0 files ls | grep testdir
'

test_expect_success FUSE "read mfs file from fuse" '
  echo content > mfs/testfile &&
  getfattr -n ipfs_cid mfs/testfile
'
test_expect_success FUSE "ipfs add file and read it back via fuse" '
  echo content3 | ipfsi 0 files  write -e /testfile3 &&
  grep content3 mfs/testfile3
'

test_expect_success FUSE "ipfs add file and read it back via fuse" '
  echo content > testfile2 &&
  ipfsi 0  add --to-files /testfile2 testfile2 &&
  grep content mfs/testfile2
'

test_expect_success FUSE "test file xattr" '
  echo content > mfs/testfile &&
  getfattr -n ipfs_cid mfs/testfile
'

test_expect_success FUSE "test file removal" '
  touch mfs/testfile &&
  rm mfs/testfile
'

test_expect_success FUSE "test nested dirs" '
  mkdir -p mfs/foo/bar/baz/qux &&
  echo content > mfs/foo/bar/baz/qux/quux &&
  ipfsi 0 files stat /foo/bar/baz/qux/quux
'

test_expect_success "mount directories cannot be removed while active" '
  test_must_fail rmdir ipfs ipns mfs 2>/dev/null
'

test_expect_success "unmount directories" '
  do_umount "$(pwd)/ipfs" &&
  do_umount "$(pwd)/ipns" &&
  do_umount "$(pwd)/mfs"
'

test_expect_success "mount directories can be removed after shutdown" '
  rmdir ipfs ipns mfs
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
