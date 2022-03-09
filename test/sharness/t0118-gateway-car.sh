#!/usr/bin/env bash

test_description="Test HTTP Gateway CAR (application/vnd.ipld.car) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

# CAR stream is not deterministic, as blocks can arrive in random order,
# but if we have a small file that fits into a single block, and export its CID
# we will get a CAR that is a deterministic array of bytes.

test_expect_success "Create a deterministic CAR for testing" '
  mkdir -p subdir &&
  echo "hello application/vnd.ipld.car" > subdir/ascii.txt &&
  ROOT_DIR_CID=$(ipfs add -Qrw --cid-version 1 subdir) &&
  FILE_CID=$(ipfs resolve -r /ipfs/$ROOT_DIR_CID/subdir/ascii.txt | cut -d "/" -f3) &&
  ipfs dag export $ROOT_DIR_CID > test-dag.car &&
  ipfs dag export $FILE_CID > deterministic.car &&
  purge_blockstore
'

# GET unixfs file as CAR
# (by using a single file we ensure deterministic result that can be compared byte-for-byte)

    test_expect_success "GET with format=car param returns a CARv1 stream" '
    ipfs dag import test-dag.car &&
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID/subdir/ascii.txt?format=car" -o gateway-param.car &&
    test_cmp deterministic.car gateway-param.car
    '

    test_expect_success "GET for application/vnd.ipld.car returns a CARv1 stream" '
    ipfs dag import test-dag.car &&
    curl -sX GET -H "Accept: application/vnd.ipld.car" "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID/subdir/ascii.txt" -o gateway-header.car &&
    test_cmp deterministic.car gateway-header.car
    '

    # explicit version=1
    test_expect_success "GET for application/vnd.ipld.raw version=1 returns a CARv1 stream" '
    ipfs dag import test-dag.car &&
    curl -sX GET -H "Accept: application/vnd.ipld.car; version=1" "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID/subdir/ascii.txt" -o gateway-header-v1.car &&
    test_cmp deterministic.car gateway-header-v1.car
    '

# GET unixfs directory as a CAR with DAG and some selector

    # TODO: this is basic test for "full" selector, we will add support for custom ones in https://github.com/ipfs/go-ipfs/issues/8769
    test_expect_success "GET for application/vnd.ipld.car with unixfs dir returns a CARv1 stream with full DAG" '
    ipfs dag import test-dag.car &&
    curl -sX GET -H "Accept: application/vnd.ipld.car" "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID" -o gateway-dir.car &&
    purge_blockstore &&
    ipfs dag import gateway-dir.car &&
    ipfs dag stat --offline $ROOT_DIR_CID
    '

# Make sure expected HTTP headers are returned with the block bytes

    test_expect_success "GET response for application/vnd.ipld.car has expected Content-Type" '
    ipfs dag import test-dag.car &&
    curl -svX GET -H "Accept: application/vnd.ipld.car" "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID/subdir/ascii.txt" >/dev/null 2>curl_output &&
    cat curl_output &&
    grep "< Content-Type: application/vnd.ipld.car; version=1" curl_output
    '

    # CAR is streamed, gateway may not have the entire thing, unable to calculate total size
    test_expect_success "GET response for application/vnd.ipld.car includes no Content-Length" '
    grep -qv "< Content-Length:" curl_output
    '

    test_expect_success "GET response for application/vnd.ipld.car includes Content-Disposition" '
    grep "< Content-Disposition: attachment\; filename=\"${FILE_CID}.car\"" curl_output
    '

    test_expect_success "GET response for application/vnd.ipld.car includes nosniff hint" '
    grep "< X-Content-Type-Options: nosniff" curl_output
    '

# Cache control HTTP headers

    test_expect_success "GET response for application/vnd.ipld.car includes a weak Etag" '
    grep "< Etag: W/\"${FILE_CID}.car\"" curl_output
    '

    # (basic checks, detailed behavior for some fields is tested in  t0116-gateway-cache.sh)
    test_expect_success "GET response for application/vnd.ipld.car includes X-Ipfs-Path and X-Ipfs-Roots" '
    grep "< X-Ipfs-Path" curl_output &&
    grep "< X-Ipfs-Roots" curl_output
    '

    test_expect_success "GET response for application/vnd.ipld.raw includes expected Cache-Control" '
    grep "< Cache-Control: no-cache, no-transform" curl_output
    '

test_kill_ipfs_daemon

test_done
