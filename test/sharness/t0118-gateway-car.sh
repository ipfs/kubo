#!/usr/bin/env bash

test_description="Test HTTP Gateway CAR (application/vnd.ipld.car) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

# CAR stream is not deterministic, as blocks can arrive in random order,
# but if we have a small file that fits into a single block, and export its CID
# we will get a CAR that is a deterministic array of bytes.

# Import test case
# See the static fixtures in ./t0118-gateway-car/
test_expect_success "Add the dir test directory" '
    cp ../t0118-gateway-car/test-dag.car ./test-dag.car &&
    cp ../t0118-gateway-car/deterministic.car ./deterministic.car
'
ROOT_DIR_CID=bafybeiefu3d7oytdumk5v7gn6s7whpornueaw7m7u46v2o6omsqcrhhkzi # ./
FILE_CID=bafkreifkam6ns4aoolg3wedr4uzrs3kvq66p4pecirz6y2vlrngla62mxm # /subdir/ascii.txt

# GET a reference DAG with dag-cbor+dag-pb+raw blocks as CAR

    # This test uses official CARv1 fixture from https://ipld.io/specs/transport/car/fixture/carv1-basic/
    test_expect_success "GET for application/vnd.ipld.car with dag-cbor root returns a CARv1 stream with full DAG" '
    ipfs dag import ../t0118-gateway-car/carv1-basic.car &&
    DAG_CBOR_CID=bafyreihyrpefhacm6kkp4ql6j6udakdit7g3dmkzfriqfykhjw6cad5lrm &&
    curl -sX GET -H "Accept: application/vnd.ipld.car" "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_CBOR_CID" -o gateway-dag-cbor.car &&
    purge_blockstore &&
    ipfs dag import gateway-dag-cbor.car &&
    ipfs dag stat --offline $DAG_CBOR_CID
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
    curl -sX GET -H "Accept: application/vnd.ipld.car;version=1" "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID/subdir/ascii.txt" -o gateway-header-v1.car &&
    test_cmp deterministic.car gateway-header-v1.car
    '

    # explicit version=1 with whitepace
    test_expect_success "GET for application/vnd.ipld.raw version=1 returns a CARv1 stream (with whitespace)" '
    ipfs dag import test-dag.car &&
    curl -sX GET -H "Accept: application/vnd.ipld.car; version=1" "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID/subdir/ascii.txt" -o gateway-header-v1.car &&
    test_cmp deterministic.car gateway-header-v1.car
    '

    # explicit version=2
    test_expect_success "GET for application/vnd.ipld.raw version=2 returns HTTP 400 Bad Request error" '
    curl -svX GET -H "Accept: application/vnd.ipld.car;version=2" "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID/subdir/ascii.txt" > curl_output 2>&1 &&
    cat curl_output &&
    grep "400 Bad Request" curl_output &&
    grep "unsupported CAR version" curl_output
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

# Make sure expected HTTP headers are returned with the CAR bytes

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

    # CAR is streamed, gateway may not have the entire thing, unable to support range-requests
    # Partial downloads and resumes should be handled using
    # IPLD selectors: https://github.com/ipfs/go-ipfs/issues/8769
    test_expect_success "GET response for application/vnd.ipld.car includes Accept-Ranges header" '
    grep "< Accept-Ranges: none" curl_output
    '

    test_expect_success "GET for application/vnd.ipld.car with query filename includes Content-Disposition with custom filename" '
    curl -svX GET -H "Accept: application/vnd.ipld.car" "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT_DIR_CID/subdir/ascii.txt?filename=foobar.car" >/dev/null 2>curl_output_filename &&
    cat curl_output_filename &&
    grep "< Content-Disposition: attachment\; filename=\"foobar.car\"" curl_output_filename
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

    test_expect_success "GET response for application/vnd.ipld.car includes same Cache-Control as a block or a file" '
    grep "< Cache-Control: public, max-age=29030400, immutable" curl_output
    '

test_kill_ipfs_daemon

test_done
